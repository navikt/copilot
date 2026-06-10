# Video HUD Architecture

## System Overview

The video HUD is a production-ready playback system for the my-copilot homepage shorts feed. It manages video playback state, progress tracking, telemetry emission, and URL synchronization through a **4-adapter pattern** with strict separation of concerns.

**Key characteristics:**

- 470+ tests passing, WCAG 2.1 AA compliant
- Event guard pattern prevents background video corruption
- KPI deduplication ensures accurate metrics
- Pure state machine for legal transitions
- Minimal, explicit API for presentational components

```
┌─────────────────────────────────────────────────┐
│         ShortsFeed Component (JSX)              │
│  - Renders video cards                          │
│  - Binds controllers to event handlers          │
│  - No imperative logic                          │
└────────────────┬────────────────────────────────┘
                 │
                 ↓
┌─────────────────────────────────────────────────┐
│   useShortsFeedController (220 lines)           │
│  - DOM refs (video, card, scroll)               │
│  - State management via playback machine        │
│  - Orchestrates 4 adapters                      │
│  - Returns explicit API                         │
└────────────────┬────────────────────────────────┘
         ┌───────┼───────┬────────┐
         │       │       │        │
         ↓       ↓       ↓        ↓
    ┌────────┬──────┬────────┬─────────┐
    │ Media  │Store │ URL    │Telemetry│
    │Adapter │Adapter Sync   │Adapter  │
    │        │      │Adapter │         │
    └────────┴──────┴────────┴─────────┘
      (250L)  (110L)  (80L)    (75L)
```

---

## Core Concepts

### 1. Event Guard Pattern (isActiveEvent)

The **event guard** is the security perimeter. It prevents background videos from corrupting the active card's state.

**Why critical:**

- Browsers fire `pause`, `timeupdate`, `waiting`, `error`, `ended` on off-screen videos
- Without guard, a scrolled-away video could:
  - Overwrite active card's progress tracking
  - Spam error/rebuffer telemetry
  - Desync the playback state machine

**Implementation:**

```typescript
const isActiveEvent = useCallback(
  (videoId: string) => isViewerOpen && videoId === resolvedActiveId,
  [isViewerOpen, resolvedActiveId]
);
```

Every media event checks this guard **first**:

```typescript
const handlePlay = useCallback(
  (videoId: string) => {
    if (!isActiveEvent(videoId)) return; // ← Guard
    dispatch({ type: "PLAY" });
    telemetry.emitVideoStarted(videoId);
  },
  [dispatch, isActiveEvent, telemetry]
);
```

**Invariant:** Only active video (isViewerOpen && videoId === resolvedActiveId) events are processed.

### 2. Adapter Pattern (4 Adapters)

Each adapter owns one responsibility:

#### Media Adapter (250 lines)

- Detect `<video>` events, delegate
- Enforce `isActiveEvent` guard on all 6 handlers
- Do NOT: track dedup, manage progress, construct KPI payloads

#### Storage Adapter (110 lines)

- Persist watch state to localStorage
- Debounce progress saves (5-second intervals)
- Force-flush on pause/ended
- Do NOT: emit KPI, guard events, manage active video

#### URL-Sync Adapter (80 lines)

- Sync `?video=...` searchParam ↔ active video
- One-way from URL to state (prevent ping-pong)
- Do NOT: modify URL, emit events outside OPEN/CLOSE, manage playback

#### Telemetry Adapter (75 lines)

- Track KPI dedup state and emit events
- `startedIds` Set: video_play_started at most once
- `playErrorKeys` Set: video_play_error at most once per error type
- `rebufferCountById` Map: track count, emit running total
- Do NOT: guard events, manage state, handle UI

### 3. State Machine

Pure, deterministic state machine enforces legal transitions:

```
           ┌──────────┐
           │  IDLE    │
           └────┬─────┘
                │ OPEN
                ↓
         ┌─────────────┐
         │   PLAYING   │
         └──┬──────┬───┘
    PAUSE  │      │  SEEK
         ┌─┴─┐  ┌─┴──┐
         ↓   ↓  ↓    ↓
      ┌──────────┐  ENDED
      │  PAUSED  │────→┐
      └──────────┘     │
         ↑             │
         └─────REPLAY──┤
                       │
              ┌────────↓────────┐
              │   COMPLETED     │
              └─────────────────┘
```

**Properties:**

- Deterministic: state + event → new state
- Invalid transitions return current state
- Only PLAYING → PAUSED/ENDED/IDLE
- COMPLETED is terminal

### 4. Video Sorting & Watch-State Freeze

The feed reorders videos by watch status so users always land on something they have not seen yet, but it **freezes that order while the viewer is open** so the active video never jumps mid-playback.

#### Sort algorithm: watch status ordering

`orderVideosByWatchStatus(videos, watchState, "deprioritize")` produces the order:

1. **Unwatched first** — videos where `isWatched(...)` is `false`
2. **Watched last** — videos marked watched move to the back
3. **Stable within each group** — original order is preserved inside both partitions

A video counts as watched once its progress crosses `WATCHED_THRESHOLD_PCT` (80%), which sets the `watched` boolean on its `WatchStatus`. The sort reads that flag via `isWatched(getWatchStatus(state, video.id))` — there is no separate `isCompleted` field.

```typescript
// src/lib/video-watch-state.ts
export function orderVideosByWatchStatus<T extends { id: string }>(
  videos: T[],
  state: WatchStateV1,
  mode: WatchOrderingMode
): T[] {
  if (mode === "hide") {
    return videos.filter((video) => !isWatched(getWatchStatus(state, video.id)));
  }
  const unwatched = videos.filter((video) => !isWatched(getWatchStatus(state, video.id)));
  const watched = videos.filter((video) => isWatched(getWatchStatus(state, video.id)));
  return [...unwatched, ...watched];
}
```

The `"deprioritize"` mode keeps watched videos in the list at the back; the alternative `"hide"` mode drops them entirely. The feed uses `"deprioritize"`.

#### Freezing the order during playback

The controller computes the sort with `useMemo`, but the **visible** order lives in separate state (`orderedVideos`) that only re-syncs when playback returns to `idle`:

```typescript
// src/components/use-shorts-feed-controller.ts
const watchOrder = useMemo(() => orderVideosByWatchStatus(videos, watchState, "deprioritize"), [videos, watchState]);
const [orderedVideos, setOrderedVideos] = useState<HomepageVideo[]>(watchOrder);
const [prevPlaybackState, setPrevPlaybackState] = useState<PlaybackState>(playbackState);

// Re-sync only on the transition back to idle (e.g. when the viewer closes).
if (playbackState !== prevPlaybackState) {
  setPrevPlaybackState(playbackState);
  if (playbackState === "idle") {
    setOrderedVideos(watchOrder);
  }
}
```

**Why freeze.** Without it the experience is jarring:

1. User opens video #3 (position 3)
2. Playback starts and progress crosses 80%
3. `watchState` updates, so `watchOrder` recomputes
4. The active video shifts to the back while the user is watching it
5. The card reflows and content jumps

By keeping `orderedVideos` frozen across every non-idle state, the active video stays put. While the viewer is open the user is only watching, so they never see the deprioritised order until they close it. On the idle transition `orderedVideos` re-syncs, and the just-watched video moves to the back for the next browse.

This freeze (commit `4985876`) replaced an earlier 300 ms `setTimeout` reorder-on-close (Phase 6d, see below): the timer approach still reordered against the raw `videos` list mid-playback, so freezing the order outright is both simpler and correct.

**Code map:**

- `src/lib/video-watch-state.ts` — `orderVideosByWatchStatus`, `isWatched`, `getWatchStatus`
- `src/components/use-shorts-feed-controller.ts` — `watchOrder` memo + `orderedVideos` freeze
- `src/components/shorts-feed.tsx` — renders `orderedVideos.map(...)`

---

## Data Flows

### Play Flow

```
User clicks play
  ↓
mediaHandlers.onPlay()
  ↓
Media adapter: check isActiveEvent(videoId) ✓
  ↓
Dispatch { type: "PLAY" }
  ↓
[In parallel]
├─ Telemetry: emit video_play_started (dedup)
├─ Storage: begin tracking progress
└─ UI: state updates
```

### Progress Tracking Flow

```
Video fires timeupdate
  ↓
Check isActiveEvent(videoId) ✓
  ↓
Storage: updateProgress(videoId, currentSecond, duration)
  ↓
Skip if not divisible by 5 (debounce)
  ↓
Skip if already persisted
  ↓
Save to localStorage
```

### Pause with Flush Flow

```
User clicks pause
  ↓
mediaHandlers.onPause()
  ↓
Check isActiveEvent(videoId) ✓
  ↓
[In parallel]
├─ Dispatch { type: "PAUSE" }
└─ Storage: flushProgress (no debounce—save immediately)
```

### Error + Telemetry Flow

```
Video error
  ↓
Check isActiveEvent(videoId) ✓
  ↓
Telemetry: emitVideoError(videoId, errorCode)
  ↓
Build key = "videoId:errorCode"
  ↓
Skip if in playErrorKeys (dedup)
  ↓
Add to set, emit video_play_error
```

### Rebuffer Flow

```
Video begins buffering (waiting)
  ↓
Check isActiveEvent(videoId) ✓
  ↓
Telemetry: addRebuffer(videoId)
  ↓
Skip if playback not started (startedIds check)
  ↓
Increment count, emit video_rebuffer_count with total
```

---

## Implementation Patterns

### Event Guard Template

All media handlers follow this pattern:

```typescript
const handleXxx = useCallback(
  (videoId: string) => {
    if (!isActiveEvent(videoId)) return; // ← FIRST

    // Safe: this is active video
    // Delegate to adapters or dispatch
  },
  [dispatch, isActiveEvent /* deps */]
);
```

### KPI Dedup

Three strategies in telemetry adapter:

```typescript
// 1. Play started: emit once per video
const startedIds = useRef<Set<string>>(new Set());
const emitVideoStarted = useCallback((videoId: string) => {
  if (startedIds.current.has(videoId)) return;
  startedIds.current.add(videoId);
  emitVideoKPIEvent("video_play_started", { videoId });
}, []);

// 2. Play error: emit once per (video, errorCode) pair
const playErrorKeys = useRef<Set<string>>(new Set());
const emitVideoError = useCallback((videoId: string, errorCode: number | string) => {
  const key = `${videoId}:${errorCode}`;
  if (playErrorKeys.current.has(key)) return;
  playErrorKeys.current.add(key);
  emitVideoKPIEvent("video_play_error", { videoId, errorCode });
}, []);

// 3. Rebuffer count: track count, emit total
const rebufferCountById = useRef<Map<string, number>>(new Map());
const addRebuffer = useCallback((videoId: string) => {
  if (!startedIds.current.has(videoId)) return;
  const current = rebufferCountById.current.get(videoId) ?? 0;
  const next = current + 1;
  rebufferCountById.current.set(videoId, next);
  emitVideoKPIEvent("video_rebuffer_count", { videoId, rebufferCount: next });
}, []);
```

### Storage Debouncing

```typescript
// updateProgress: ~30 events/sec → ~6 saves/min
const updateProgress = useCallback((videoId: string, currentSecond: number, duration: number | undefined) => {
  if (currentSecond <= 0) return;
  if (currentSecond % 5 !== 0) return; // Every 5 seconds

  const lastPersistedSecond = persistedProgressSecondById.current.get(videoId) ?? -1;
  if (lastPersistedSecond === currentSecond) return;

  persistedProgressSecondById.current.set(videoId, currentSecond);
  setWatchState((prev) => {
    const next = upsertProgress({ state: prev, videoId, currentTimeSec: currentSecond, durationSec: duration });
    if (next !== prev) saveWatchState(next);
    return next;
  });
}, []);

// flushProgress: save immediately on pause/ended
const flushProgress = useCallback((videoId: string, currentSecond: number, duration: number | undefined) => {
  if (currentSecond <= 0) return;

  const lastPersistedSecond = persistedProgressSecondById.current.get(videoId) ?? -1;
  if (lastPersistedSecond === currentSecond) return;

  persistedProgressSecondById.current.set(videoId, currentSecond);
  setWatchState((prev) => {
    const next = upsertProgress({ state: prev, videoId, currentTimeSec: currentSecond, durationSec: duration });
    if (next !== prev) saveWatchState(next);
    return next;
  });
}, []);
```

---

## KPI & Metrics

### Events Emitted

| Event                   | When          | Dedup              | Payload                    |
| ----------------------- | ------------- | ------------------ | -------------------------- |
| `video_feed_impression` | Feed loads    | Once per load      | `videoCount`               |
| `video_play_started`    | First play    | Set (by videoId)   | `videoId`                  |
| `video_play_error`      | Video error   | Set (videoId:code) | `videoId`, `errorCode`     |
| `video_rebuffer_count`  | Each rebuffer | None—emit total    | `videoId`, `rebufferCount` |

### Guarantees

**Within session:**

- `video_play_started`: once per video (survives pause/replay)
- `video_play_error`: once per error type per video
- `video_rebuffer_count`: on every rebuffer (count increases)

**On session reset:**

- Dedup state in memory (useRef)
- Closes/refreshes reset dedup
- Next session starts clean

---

## Accessibility (WCAG 2.1 AA)

### Keyboard Navigation

- Tab order follows DOM (cards left-to-right, top-to-bottom)
- Play/pause labeled with video title
- Skip buttons labeled (±5 seconds)
- Share button labeled with video title

### Screen Readers

- Video element has `title` attribute
- Playback buttons have `aria-label`
- Glyph badges labeled (e.g., "Status: ✓")
- Episode pill labeled (e.g., "Episode 1")

### Focus Management

- Focus visible on interactive elements
- No focus traps
- Respects `prefers-reduced-motion` media query

### Color & Contrast

- Episode pill colors: 4.5:1 minimum
- High contrast on accent backgrounds
- No color-only information

---

## Testing Strategy

### 1. Event Guard Tests

Verify background videos can't corrupt active state:

```typescript
describe("Event Guard", () => {
  it("guards onPlay from background", () => {
    handlers.onPlay("bg-video");
    expect(dispatch).not.toHaveBeenCalled();
  });

  it("allows active video to dispatch", () => {
    handlers.onPlay("active-video");
    expect(dispatch).toHaveBeenCalledWith({ type: "PLAY" });
  });
});
```

### 2. KPI Dedup Tests

Verify exact-once semantics:

```typescript
describe("Telemetry Dedup", () => {
  it("emits video_play_started once per video", () => {
    telemetry.emitVideoStarted("v1");
    telemetry.emitVideoStarted("v1");
    expect(emitVideoKPIEvent).toHaveBeenCalledTimes(1);
  });

  it("emits error once per (video, code)", () => {
    telemetry.emitVideoError("v1", 1);
    telemetry.emitVideoError("v1", 1); // Skip
    telemetry.emitVideoError("v1", 2); // Emit
    expect(emitVideoKPIEvent).toHaveBeenCalledTimes(2);
  });

  it("emits running rebuffer count", () => {
    telemetry.emitVideoStarted("v1");
    telemetry.addRebuffer("v1");
    expect(emitVideoKPIEvent).toHaveBeenCalledWith("video_rebuffer_count", { videoId: "v1", rebufferCount: 1 });
  });
});
```

### 3. Storage Adapter Tests

Verify progress and flush semantics:

```typescript
describe("Storage Adapter", () => {
  it("debounces to 5-second intervals", () => {
    storage.updateProgress("v1", 1, 100); // Skip
    storage.updateProgress("v1", 5, 100); // Save
    storage.updateProgress("v1", 10, 100); // Save
    expect(saveWatchState).toHaveBeenCalledTimes(2);
  });

  it("flushProgress saves immediately", () => {
    storage.updateProgress("v1", 1, 100);
    storage.flushProgress("v1", 1, 100);
    expect(saveWatchState).toHaveBeenCalledTimes(1);
  });
});
```

### 4. Integration Tests

Verify adapters work together:

```typescript
describe("Full Flow", () => {
  it("play → progress → pause → complete", () => {
    const controller = renderHook(() => useShortsFeedController({ videos: [v1, v2] }));

    act(() => controller.result.current.openViewer("v1"));
    act(() => controller.result.current.resumePlayback("v1"));

    mockVideoElement.currentTime = 5;
    const handlers = controller.result.current.mediaHandlers("v1");
    act(() => handlers.onTimeUpdate());

    act(() => controller.result.current.pausePlayback("v1"));
    expect(controller.result.current.playbackState).toBe("paused");
  });
});
```

---

## Known Limitations

### 1. Reflow on video close

Closing the viewer sets `display: none` on non-active cards → DOM reflow.

**Impact:** Visual jank on low-end devices.
**Future:** Use `visibility: hidden` or CSS containment.

> The three earlier limitations — implicit `pendingPlayId` writers, unprotected KPI events, and order jumping on first play — were resolved in Phase 6 (see below).

---

## Phase 6: Final polish & stability

Phase 6 closed the open stability gaps from the original "Known Limitations" list. All four parts are merged and covered by tests.

### Phase 6a: Unified control surface

Removed the native HTML5 `<video controls>` UI so the custom HUD is the single control surface, and swapped remaining Tailwind spacing for Aksel tokens.

- `src/components/shorts-feed.tsx` — the `<video>` element no longer sets `controls`
- `src/components/video-card-chrome.tsx` — Aksel spacing tokens instead of Tailwind `px-*/gap-*`
- `src/components/unified-video-hud.tsx` — one surface for play/pause/seek/fullscreen

**Why:** avoids duplicate (native + custom) controls and keeps styling consistent with Nav DS.
**Commit:** `6533981`

### Phase 6b: Error resilience for KPI events

`emitVideoKPIEvent` now wraps `window.dispatchEvent` in try/catch, logs the failure, and does not re-throw. Telemetry failure can no longer crash playback.

```typescript
// src/lib/video-kpi-events.ts
export function emitVideoKPIEvent(event: VideoKPIEventName, payload: VideoKPIEventPayload) {
  try {
    const entry: VideoKPIEvent = { event, payload };
    window.dispatchEvent(new CustomEvent<VideoKPIEvent>("video-kpi", { detail: entry }));
  } catch (error) {
    console.error("[KPI Event Error] Failed to emit video KPI event:", error, { event });
    // Intentionally don't re-throw; KPI telemetry failure should not crash playback.
  }
}
```

**Why:** telemetry should degrade gracefully, never break the media experience.
**Tests:** `src/lib/video-kpi-events.test.ts`.
**Commit:** `924426e`

### Phase 6c: State machine coordination

Centralised autoplay intent so play happens exactly once.

- Removed the redundant `resumePlayback()` call from `openViewer()` — opening flips `isViewerOpen`/`resolvedActiveId`, which re-runs the autoplay effect and plays the video once.
- Added `requestAutoplay`, the single writer of `pendingPlayId`. Both `openViewer` and the URL-sync adapter route through it.
- The autoplay effect stays the only reader/clearer of `pendingPlayId`.

```typescript
// src/components/use-shorts-feed-controller.ts
const requestAutoplay = useCallback((videoId: string) => {
  pendingPlayId.current = videoId;
}, []);
```

`pendingPlayId` is a `useRef`, not React state, so setting it never triggers a render — the autoplay effect simply reads `.current` on its next run and clears it.

**Why:** one writer + one clearer removes double-play and makes the handoff easy to reason about.
**Tests:** race-condition tests in `use-shorts-feed-controller.test.ts` (plays once, clears after autoplay, last-wins on rapid open/close).
**Commit:** `10994e7`

### Phase 6d: Smooth reordering on close

Phase 6d first added a 300 ms `setTimeout` in `closeViewer` (plus a `forceReorder` toggle) to defer the re-sort until the close animation finished.

That delay was **superseded** by the watch-state freeze (commit `4985876`, see [Video Sorting & Watch-State Freeze](#4-video-sorting--watch-state-freeze)). The current `closeViewer` carries no timer: the `CLOSE` event returns playback to `idle`, and `orderedVideos` re-syncs to the latest watch order on that transition. Same smooth result, no timer to clean up.

```typescript
// src/components/use-shorts-feed-controller.ts
const closeViewer = useCallback(() => {
  dispatch({ type: "CLOSE" });
  // Returning to idle re-syncs `orderedVideos` to the latest watch-status
  // order during the next render, so a just-watched video is deprioritized.
}, [dispatch]);
```

**Commit:** `64623dc` (initial delay), refined by `4985876` (freeze).

---

## Future improvements

### Long-term roadmap

1. Generalize adapter pattern (podcasts, live streams)
2. Server-side watch state (cross-device resume)
3. Adaptive bitrate (HLS/DASH)
4. Picture-in-picture support
5. Analytics dashboard (KPI trends)

---

## API Reference

### useShortsFeedController

```typescript
function useShortsFeedController({
  videos: HomepageVideo[];
  initialVideoId?: string;
}): ShortsFeedController
```

**Returns:**

```typescript
type ShortsFeedController = {
  orderedVideos: HomepageVideo[];
  resolvedActiveId: string;
  isViewerOpen: boolean;
  playbackState: PlaybackState; // idle | playing | paused | completed
  reducedMotion: boolean;
  scrollContainerRef: React.RefObject<HTMLDivElement | null>;
  setVideoNode: (videoId: string, node: HTMLVideoElement | null) => void;
  setCardNode: (videoId: string, node: HTMLDivElement | null) => void;
  mediaHandlers: (videoId: string) => ShortsFeedMediaHandlers;
  openViewer: (videoId: string) => void;
  onPrimaryAction: (videoId: string) => void;
  resumePlayback: (videoId: string) => void;
  pausePlayback: (videoId: string) => void;
  replayPlayback: (videoId: string) => void;
  seekPlayback: (videoId: string, deltaSeconds: number) => void;
  toggleFullscreen: (videoId: string) => void;
  handleCardKeyDown: (event: KeyboardEvent<HTMLDivElement>, videoId: string) => void;
};
```

**Example:**

```typescript
import { useShortsFeedController } from "./use-shorts-feed-controller";

function ShortsFeed({ videos }: { videos: HomepageVideo[] }) {
  const controller = useShortsFeedController({ videos });

  return (
    <div ref={controller.scrollContainerRef}>
      {controller.orderedVideos.map((video) => (
        <VideoCard
          key={video.id}
          video={video}
          active={video.id === controller.resolvedActiveId}
          handlers={controller.mediaHandlers(video.id)}
          onPlay={() => controller.resumePlayback(video.id)}
        />
      ))}
    </div>
  );
}
```

---

## Glossary

- **isActiveEvent** — Guard ensuring only active video drives state
- **Adapter** — Self-contained module with one responsibility
- **Dedup** — Emit KPI at most once (tracked via Set/Map)
- **Flush** — Force-save progress (on pause/ended)
- **Rebuffer** — Pause in playback while loading (`waiting` event)
- **Telemetry** — KPI emission (play_started, error, rebuffer_count)
- **Playback Machine** — Pure state machine enforcing legal transitions
- **Watch State** — Progress + completion tracking (localStorage)

---

## File Structure

```
apps/my-copilot/src/components/
├── use-shorts-feed-controller.ts             (220L)
├── use-shorts-feed-controller.test.ts
├── use-shorts-feed-media-adapter.ts          (250L)
├── use-shorts-feed-media-adapter.test.ts
├── use-shorts-feed-storage-adapter.ts        (110L)
├── use-shorts-feed-url-sync-adapter.ts       (80L)
├── use-shorts-feed-telemetry-adapter.ts      (75L)
├── use-shorts-feed-telemetry-adapter.test.ts
├── shorts-feed.tsx
├── unified-video-hud.tsx
└── video-overlay-components.tsx

apps/my-copilot/src/lib/
├── video-playback-machine.ts
├── video-watch-state.ts
├── video-kpi-events.ts
└── public-videos.ts
```

---

## UI Design Guide

The video player HUD system provides a professional, polished interface for displaying video metadata, controls, and overlays. The design is built on Aksel (Nav's design system) with careful attention to visual hierarchy, accessibility, and responsive behavior.

### Two-Layer System

```
┌─────────────────────────────────────────┐
│ Playback Controls (z-30)                │  Only when active & playing/paused
│ - Play/pause button (center)            │
│ - Skip forward/backward buttons         │
└─────────────────────────────────────────┘
         ↓ (on top of)
┌─────────────────────────────────────────┐
│ Decoration Layer (z-20)                 │  Always visible when showHud=true
│ ┌─────────────────────────────────────┐ │
│ │ Top Rail (z-20)                     │ │
│ │ - Episode pill (left)               │ │
│ │ - Glyph badges (left)               │ │
│ │ - Duration (right)                  │ │
│ │ - Share button (right)              │ │
│ └─────────────────────────────────────┘ │
│ ┌─────────────────────────────────────┐ │
│ │ Content Panel (idle only)           │ │  Hidden when playing
│ │ - Rules (headline)                  │ │
│ │ - Ladders (step sequences)          │ │
│ │ - Counters (before → after)         │ │
│ │ - Chips (tag groups)                │ │
│ │ - Result badges                     │ │
│ └─────────────────────────────────────┘ │
└─────────────────────────────────────────┘
         ↓ (on top of)
┌─────────────────────────────────────────┐
│ Video Player                            │
└─────────────────────────────────────────┘
```

### Component Library Reference

**Components available:**

1. **EpisodePill** — Small badge with episode number and accent color
2. **GlyphBadge** — Circular badge for status indicators (✓, !, ★)
3. **MicroChip** — Inline tag for labels and tokens
4. **ChipRow** — Labeled group of related tokens
5. **LadderRow** — Ordered sequence with highlighted steps
6. **CounterRow** — Before → After transition display
7. **ResultRow** — Outcome statement with checkmark
8. **RuleHeader** — Headline with divider lines
9. **ContentPanel** — Container with all content rows
10. **GlyphBadges** — Batch renderer for glyph badges
11. **UnifiedVideoHUD** — Complete HUD component

### Accent Colors (cycle by episode)

- Episode 1, 7, 13, ... → `#66d4cf` (cyan)
- Episode 2, 8, 14, ... → `#9af0a8` (green)
- Episode 3, 9, 15, ... → `#ffd485` (yellow)
- Episode 4, 10, 16, ... → `#c6a8ff` (purple)
- Episode 5, 11, 17, ... → `#7cc7ff` (light blue)
- Episode 6, 12, 18, ... → `#ff9db1` (pink)

### Spacing & Layout

**Tokens used:**

- `space-2` = 0.5rem gap between chips
- `space-4` = 1rem gap between icons/chips
- `space-8` = 2rem gap in content panel rows
- `space-16` = standard nav spacing

**Responsive behavior:**

- Mobile-first design
- HStack/VStack handle small screens
- Nowrap prevents overflow on chips
- Adaptive font sizes

### Component Props Reference

```tsx
interface UnifiedVideoHUDProps {
  // Data (required)
  overlays?: OverlayComponent[]; // Metadata overlays from video
  episodeLabel: string; // Episode number (e.g., "1")
  accent: string; // Color from accentForEpisode()
  durationLabel: string; // Formatted duration (e.g., "2:34")
  shareHref: string; // Share link URL
  shareTitle: string; // Video title for ARIA label

  // State (required)
  playing: boolean; // Is video currently playing?
  isActive: boolean; // Is this the active/hovered card?
  completed: boolean; // Has video finished?
  showHud: boolean; // Should HUD be visible? (fade on 1.8s during play)

  // Callbacks (required)
  onTogglePlayback: () => void; // Play/pause button clicked
  onSeekBackward: () => void; // Seek -5 seconds clicked
  onSeekForward: () => void; // Seek +5 seconds clicked
  title: string; // Video title for ARIA labels
}

interface OverlayComponent {
  kind: "episode-number" | "badge" | "chip" | "counter" | "ladder" | "rule-pill" | string;
  anchor:
    | "top-left"
    | "top-right"
    | "center-left"
    | "center-right"
    | "center"
    | "bottom-left"
    | "bottom-right"
    | "bottom-full";
  labels: string[]; // Content for the overlay
  highlightIndex?: number; // For ladder: which step is active (0-indexed)
  monospace?: boolean; // Use monospace font? (for code/commands)
}
```

### State Matrix

| Scenario          |         HUD         | Top Rail | Content | Controls |
| ----------------- | :-----------------: | :------: | :-----: | :------: |
| Idle (not active) |       Hidden        |    —     |    —    |    —     |
| Browsing (hover)  |       Visible       |    ✓     |    ✓    |    —     |
| Playing           | Visible (fade 1.8s) |    ✓     | Hidden  |    ✓     |
| Paused            |       Visible       |    ✓     | Hidden  |    ✓     |
| Completed         |       Hidden        |    —     |    —    |    —     |

### Z-Index Hierarchy

```
z-50: CompletedOverlay (separate component)
z-40: IdleCaption (separate component)
z-30: PlaybackControls (inside UnifiedVideoHUD)
z-20: Decoration layer (inside UnifiedVideoHUD)
      ├─ Top Rail
      ├─ Content Panel
      └─ CornerFullscreenButton
z-10: Video element
z-0:  Poster image
```

### Design System Alignment

- Uses HStack/VStack for layout
- Proper spacing tokens (never Tailwind p-/m- utilities)
- Proper heading hierarchy
- Button elements for interactive controls
- Span elements for static content
- Accessible icon rendering
- High contrast text (4.5:1 WCAG AA)

## Implementation Reference

### Quick Start - Using Controllers & Adapters

```typescript
// 1. Initialize adapters
const { watchState, updateProgress, markComplete, flushProgress } = useStorageAdapter();
const telemetry = useTelemetryAdapter({ videos });

// 2. Create guard
const isActiveEvent = useCallback(
  (videoId: string) => isViewerOpen && videoId === resolvedActiveId,
  [isViewerOpen, resolvedActiveId]
);

// 3. Initialize media adapter with guard
const media = useMediaAdapter({
  dispatch,
  isActiveEvent,
  telemetry,
  updateProgress,
  markComplete,
  flushProgress,
});

// 4. Sync URL
useUrlSyncAdapter({
  videos,
  initialActiveId,
  isViewerOpen,
  dispatch,
  setActiveId,
  setIsViewerOpen,
  onOpenViewer: (videoId) => {
    pendingPlayId.current = videoId;
  },
});

// 5. Return explicit API
return {
  orderedVideos,
  resolvedActiveId,
  isViewerOpen,
  playbackState,
  reducedMotion,
  scrollContainerRef,
  setVideoNode: media.setVideoNode,
  setCardNode: media.setCardNode,
  mediaHandlers: media.mediaHandlers,
  // ... action methods
};
```

### Common Patterns

**Preventing Double-emit:**

```typescript
// ✅ Good: use Set to track emitted events
const startedIds = useRef<Set<string>>(new Set());
const emitVideoStarted = useCallback((videoId: string) => {
  if (startedIds.current.has(videoId)) return;
  startedIds.current.add(videoId);
  emitVideoKPIEvent("video_play_started", { videoId });
}, []);
```

**Debouncing Progress:**

```typescript
// ✅ Good: check divisibility and track persisted second
const updateProgress = useCallback((videoId: string, currentSecond: number) => {
  if (currentSecond % 5 !== 0) return;  // Every 5 sec

  const lastPersistedSecond = persistedProgressSecondById.current.get(videoId) ?? -1;
  if (lastPersistedSecond === currentSecond) return;  // Already saved

  persistedProgressSecondById.current.set(videoId, currentSecond);
  saveWatchState(...);
}, []);
```

**Guard Pattern:**

```typescript
// ✅ Good: guard first
const handlePlay = useCallback(
  (videoId: string) => {
    if (!isActiveEvent(videoId)) return; // ← Guard

    dispatch({ type: "PLAY" });
    telemetry.emitVideoStarted(videoId);
  },
  [dispatch, isActiveEvent, telemetry]
);
```

### Adapter Details

**Media Adapter — Exports:**

```typescript
type ShortsFeedMediaHandlers = {
  onPlay: () => void;
  onPause: () => void;
  onTimeUpdate: () => void;
  onEnded: () => void;
  onError: () => void;
  onWaiting: () => void;
};

type UseMediaAdapterReturn = {
  videoRefs: React.MutableRefObject<Map<string, HTMLVideoElement>>;
  cardRefs: React.MutableRefObject<Map<string, HTMLDivElement>>;
  setVideoNode: (videoId: string, node: HTMLVideoElement | null) => void;
  setCardNode: (videoId: string, node: HTMLDivElement | null) => void;
  resumePlayback: (videoId: string) => void;
  pausePlayback: (videoId: string) => void;
  replayPlayback: (videoId: string) => void;
  seekPlayback: (videoId: string, deltaSeconds: number) => void;
  toggleFullscreen: (videoId: string) => void;
  mediaHandlers: (videoId: string) => ShortsFeedMediaHandlers;
};
```

**Storage Adapter — Exports:**

```typescript
type StorageAdapter = {
  watchState: WatchStateV1;
  updateProgress: (videoId: string, currentSecond: number, duration: number | undefined) => void;
  markComplete: (videoId: string, duration: number | undefined) => void;
  flushProgress: (videoId: string, currentSecond: number, duration: number | undefined) => void;
};
```

**Telemetry Adapter — Exports:**

```typescript
type TelemetryAdapter = {
  emitVideoStarted: (videoId: string) => void;
  emitVideoError: (videoId: string, errorCode: number | string) => void;
  addRebuffer: (videoId: string) => void;
};
```

### Troubleshooting

**Progress not saving:**

1. Check `updateProgress` called with `currentSecond % 5 === 0`
2. Verify `flushProgress` called on pause
3. Check localStorage is available
4. Inspect watch state: `localStorage.getItem('shorts:watchState')`

**Duplicate KPI events:**

1. Check `startedIds` Set being used in telemetry adapter
2. Verify `emitVideoStarted` checks before emitting
3. Confirm dedup state is not reset between plays

**Background video corruption:**

1. Check guard: `if (!isActiveEvent(videoId)) return;`
2. Verify `isActiveEvent` includes both `isViewerOpen && videoId === resolvedActiveId`
3. Confirm media adapter uses guard on all 6 handlers

### Performance Tips

1. **Memoize handlers** — Use `useCallback` on all media handlers
2. **Batch updates** — Storage adapter batches timeupdate → progress updates
3. **Dedup aggressively** — Sets/Maps prevent duplicate KPI emission
4. **Guard early** — Exit before any expensive operations
5. **Debounce storage** — 5-second intervals = ~6 saves/min vs 30/sec

### Accessibility Checklist

- [ ] EpisodePill has aria-label="Episode X"
- [ ] GlyphBadge has aria-label="Status: ✓"
- [ ] PlaybackControls buttons have aria-label with context
- [ ] Share button has aria-label with video title
- [ ] All decorative icons have aria-hidden="true"
- [ ] Focus rings visible on keyboard navigation
- [ ] Text contrast >= 4.5:1 WCAG AA
- [ ] Keyboard shortcut descriptions in tooltips

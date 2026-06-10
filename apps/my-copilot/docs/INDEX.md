# Video HUD Documentation Index

Complete documentation suite for the video playback system.

## Quick Start

1. **New to the system?** Start with [System Overview](VIDEO_HUD_ARCHITECTURE.md#system-overview)
2. **Need code examples?** Go to [Implementation Reference](../src/components/README_VIDEO_HUD.md)
3. **Building a feature?** Check [KPI & Metrics](VIDEO_HUD_ARCHITECTURE.md#kpi--metrics)
4. **Debugging?** See [Troubleshooting](../src/components/README_VIDEO_HUD.md#troubleshooting)

---

## Documentation Map

### Architecture & Design

- **[VIDEO_HUD_ARCHITECTURE.md](VIDEO_HUD_ARCHITECTURE.md)**
  - Complete system overview with diagrams
  - Core concepts: event guard, adapter pattern, state machine, video sorting & watch-state freeze
  - Data flows and implementation patterns
  - KPI dedup strategy
  - Testing strategy with code examples
  - Phase 6 changes, known limitations and future improvements

### Implementation Reference

- **[README_VIDEO_HUD.md](../src/components/README_VIDEO_HUD.md)** (413 lines)
  - Adapter API signatures and exports
  - Quick reference tables
  - Usage patterns with code
  - Event flow diagrams
  - Testing checklist
  - Common patterns and anti-patterns
  - Troubleshooting guide

### UI Components & Design

- **[HUD_DESIGN_GUIDE.md](../HUD_DESIGN_GUIDE.md)** (600+ lines)
  - UI component library
  - Design tokens and colors
  - Two-layer z-index system
  - All 11 component types documented
  - Usage patterns with full examples
  - Accessibility considerations

- **[HUD_COMPONENT_REFERENCE.md](../HUD_COMPONENT_REFERENCE.md)** (366 lines)
  - Quick reference for props
  - Component prop interfaces
  - Copy-paste examples
  - State matrix
  - Z-index hierarchy
  - Performance tips
  - Accessibility checklist

---

## Key Concepts at a Glance

### Event Guard (isActiveEvent)

```typescript
const isActiveEvent = (videoId: string) => isViewerOpen && videoId === resolvedActiveId;
```

Prevents background videos from corrupting the active card's state. Every media event checks this guard first.

### 4-Adapter Pattern

| Adapter   | Lines | Responsibility                                   |
| --------- | ----- | ------------------------------------------------ |
| Media     | 250   | Detect `<video>` events, guard all handlers      |
| Storage   | 110   | Persist progress to localStorage, debounce saves |
| URL-Sync  | 80    | Keep `?video=...` searchParam in sync            |
| Telemetry | 75    | Track KPI dedup state and emit events            |

### KPI Deduplication

- **video_play_started** — Set-based: emit once per video
- **video_play_error** — Set-based: emit once per (video, errorCode) pair
- **video_rebuffer_count** — Map-based: track count, emit running total

### State Machine

```
IDLE → PLAYING ← PAUSED → ENDED → COMPLETED
```

Pure, deterministic transitions prevent invalid state combinations.

---

## File Structure

```
apps/my-copilot/
├── docs/
│   ├── INDEX.md (this file)
│   └── VIDEO_HUD_ARCHITECTURE.md ⭐ Start here
│
├── HUD_DESIGN_GUIDE.md ⭐ UI designers
├── HUD_COMPONENT_REFERENCE.md
│
└── src/components/
    ├── README_VIDEO_HUD.md ⭐ Developers
    ├── use-shorts-feed-controller.ts (220L)
    ├── use-shorts-feed-media-adapter.ts (250L)
    ├── use-shorts-feed-storage-adapter.ts (110L)
    ├── use-shorts-feed-url-sync-adapter.ts (80L)
    ├── use-shorts-feed-telemetry-adapter.ts (75L)
    ├── shorts-feed.tsx
    └── unified-video-hud.tsx
```

---

## Common Tasks

### I need to...

| Task                            | Read                    | Key Section                                                                                         |
| ------------------------------- | ----------------------- | --------------------------------------------------------------------------------------------------- |
| Understand how the system works | ARCHITECTURE            | [System Overview](VIDEO_HUD_ARCHITECTURE.md#system-overview)                                        |
| Understand video sorting/order  | ARCHITECTURE            | [Video Sorting & Watch-State Freeze](VIDEO_HUD_ARCHITECTURE.md#4-video-sorting--watch-state-freeze) |
| Add a new KPI event             | README_VIDEO_HUD        | [KPI Events](../src/components/README_VIDEO_HUD.md#kpi-events-emitted)                              |
| Fix a bug in progress tracking  | ARCHITECTURE            | [Progress Tracking Flow](VIDEO_HUD_ARCHITECTURE.md#progress-tracking-flow)                          |
| Design a new UI overlay         | HUD_DESIGN_GUIDE        | [Component Library](../HUD_DESIGN_GUIDE.md#component-library)                                       |
| Write tests for guards          | README_VIDEO_HUD        | [Event Guard Tests](../src/components/README_VIDEO_HUD.md#media-adapter-tests)                      |
| Investigate duplicate KPI       | ARCHITECTURE            | [Deduplication Strategy](VIDEO_HUD_ARCHITECTURE.md#deduplication-strategy)                          |
| Migrate old component           | HUD_COMPONENT_REFERENCE | [Migration Guide](../HUD_COMPONENT_REFERENCE.md#migration-guide)                                    |
| Understand accessibility        | ARCHITECTURE            | [Accessibility](VIDEO_HUD_ARCHITECTURE.md#accessibility-wcag-21-aa)                                 |

---

## Testing

All documentation includes comprehensive testing strategies:

### By Scope

1. **Unit Tests** — Guard behavior, adapter isolation
2. **Integration Tests** — Adapter cooperation, state transitions
3. **E2E Tests** — Full play → pause → complete flow

See [Testing Strategy](VIDEO_HUD_ARCHITECTURE.md#testing-strategy) for code examples.

---

## System Status

✅ **Production-ready**

- 470+ tests passing
- WCAG 2.1 AA compliant
- Event guard prevents data corruption
- KPI dedup ensures accurate metrics
- Pure state machine for legal transitions

---

## Glossary

- **isActiveEvent** — Guard (only active video can affect state)
- **Adapter** — Self-contained module with one responsibility
- **Dedup** — Deduplication (emit KPI at most once)
- **Flush** — Force-save progress (on pause/ended)
- **Rebuffer** — Pause in playback while loading
- **Telemetry** — KPI events (play_started, error, rebuffer_count)
- **Playback Machine** — Pure state machine for legal transitions
- **Watch State** — Progress + completion (localStorage)

---

## How to Update These Docs

1. Keep architecture decisions in `VIDEO_HUD_ARCHITECTURE.md`
2. Add implementation details to `README_VIDEO_HUD.md`
3. Update UI specs in `HUD_DESIGN_GUIDE.md` and `HUD_COMPONENT_REFERENCE.md`
4. Run full TypeScript/ESLint checks before committing

---

## Related Files

- `src/lib/video-playback-machine.ts` — Pure state machine
- `src/lib/video-watch-state.ts` — WatchState type + helpers
- `src/lib/video-kpi-events.ts` — KPI event emission
- `src/lib/public-videos.ts` — Video data types

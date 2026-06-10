# Video page rewrite plan

## Goal

Rewrite the video experience in `my-copilot` as a dedicated, mobile- and desktop-friendly page with canonical direct links.

## Decision

- Canonical route: `/videos/[id]`
- No legacy `/?video=` support
- Homepage video feed stays as an entry point, not the primary viewing surface

## Why rewrite

The current shorts feed is built for browsing inside the homepage. It is not a good fit for direct linking, focused viewing, or a stable page structure for mobile and desktop.

The dedicated page should own:

- route-based navigation
- stable deep linking
- responsive player layout
- related videos
- clearer metadata and captions UX

## Scope

### In

- New dedicated video route
- New video detail data path
- Responsive page layout for mobile and desktop
- Shared player surface and HUD logic where practical
- Related videos section
- Empty/error/not-found states
- Tests for route, data loading, and core interaction

### Out

- Keeping `/?video=` alive
- Large redesign of the homepage beyond necessary link updates
- Changing the underlying video storage/manifest model unless required

## Proposed architecture

### Routing

- Add `apps/my-copilot/src/app/videos/[id]/page.tsx`
- Add `loading.tsx` and `not-found.tsx`
- Make `/videos/[id]` the only shareable direct-link format
- Update all share links to point to the canonical route

### Data loading

- Add a dedicated backend endpoint for one video:
  - `GET /public/v1/videos/{id}`
- Keep the feed endpoint for related videos and homepage browsing
- Fetch the selected video first, then a small related list

### UI structure

Mobile:

- player first
- title + metadata
- actions row
- related videos below

Desktop:

- two-column layout
- large player left
- metadata/actions/related rail right

### Playback

- Reuse playback machine, caption handling, fullscreen, and watch-state
- Dedicated page should behave like a normal video page, not a feed card
- Autoplay should be attempted but not required

### Accessibility

- visible focus states
- keyboard-friendly controls
- sensible heading order
- reduced-motion support
- screen reader labels for play, pause, share, captions, fullscreen

## Implementation phases

### Phase 1: Route skeleton

Files:

- `apps/my-copilot/src/app/videos/[id]/page.tsx`
- `apps/my-copilot/src/app/videos/[id]/loading.tsx`
- `apps/my-copilot/src/app/videos/[id]/not-found.tsx`

Deliverable:

- route exists
- page renders a placeholder structure
- not-found state works

### Phase 2: Data API

Files:

- `apps/copilot-api/video_handlers.go`
- `apps/copilot-api/video_manifest.go`
- `apps/copilot-api/handlers.go`
- `apps/my-copilot/src/lib/public-videos.ts`

Deliverable:

- single-video endpoint exists
- frontend helper fetches one video by id

### Phase 3: Page rewrite

Files:

- new dedicated page components under `apps/my-copilot/src/components/`
- route page wiring
- shared player wrapper if needed

Deliverable:

- responsive video page with player + metadata + related videos

### Phase 4: Share and navigation cleanup

Files:

- `apps/my-copilot/src/components/shorts-feed.tsx`
- any existing share helpers
- homepage references

Deliverable:

- share links use `/videos/[id]`
- homepage points to the canonical page

### Phase 5: Tests

Files:

- `*.test.ts`
- `*.test.tsx`

Deliverable:

- route/data/render tests
- interaction tests for play/pause, not-found, captions, and responsive layout

## Risks

- duplicating playback logic between homepage feed and dedicated page
- inconsistent watch-state behavior if the new page does not reuse the same state machine
- layout regressions on small screens if the player shell is too desktop-centric

## Preferred implementation order

1. Backend single-video endpoint
2. Frontend route skeleton
3. Shared player shell
4. Related videos and responsive layout
5. Tests and cleanup


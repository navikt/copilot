// Playback state machine for the homepage shorts feed.
//
// This module is intentionally free of React and DOM concerns: it is a pure
// description of *what state a single card can be in* and *which transitions are
// legal*. The controller hook (use-shorts-feed-controller) owns the refs and
// media side-effects and drives this machine by dispatching events. Presentation
// components only read the resulting state. Keeping the rules here makes the
// transitions trivially unit-testable and gives us a single source of truth for
// derived UI questions like "can the user pause right now?".

export type PlaybackState = "idle" | "playing" | "paused" | "completed";

export type PlaybackEvent =
  | { type: "OPEN" } // viewer opened for this card (ready, not yet playing)
  | { type: "PLAY" } // media element started playing
  | { type: "PAUSE" } // media element paused
  | { type: "END" } // media element reached the end
  | { type: "REPLAY" } // user asked to watch again from the start
  | { type: "CLOSE" }; // viewer closed (e.g. share link cleared)

export const INITIAL_PLAYBACK_STATE: PlaybackState = "idle";

// Explicit transition table. Anything not listed is a deliberate no-op so the
// machine never throws and stays in a coherent state even if the browser fires
// events in an unexpected order (e.g. a stray `pause` after `ended`).
export function playbackTransition(state: PlaybackState, event: PlaybackEvent): PlaybackState {
  switch (event.type) {
    case "OPEN":
      // Opening prepares playback. If we were already playing keep playing,
      // otherwise land in the paused/ready state (shared links open paused).
      return state === "playing" ? "playing" : "paused";
    case "PLAY":
      return "playing";
    case "PAUSE":
      // A `pause` event that arrives once the clip has completed must not drop
      // us out of the completed state (some browsers emit pause before ended).
      return state === "completed" ? "completed" : "paused";
    case "END":
      return "completed";
    case "REPLAY":
      return "playing";
    case "CLOSE":
      return "idle";
    default:
      return state;
  }
}

// The central action button always pauses while playing. Exposed as a helper so
// the UI and the controller agree on when pausing is meaningful.
export function canPause(state: PlaybackState): boolean {
  return state === "playing";
}

// Body overlay content (chips/ladders/rules) and the title caption belong to the
// idle browsing state only. Once a viewer is opened (paused/playing/completed),
// the playback HUD owns the surface and should stay visually stable.
export function isBodyContentVisible(state: PlaybackState): boolean {
  return state === "idle";
}

export function isCompleted(state: PlaybackState): boolean {
  return state === "completed";
}

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { HomepageVideo } from "@/lib/public-videos";
import { ShortsFeed } from "./shorts-feed";

const emitVideoKPIEvent = vi.fn();
const useSearchParamsMock = vi.fn();

vi.mock("next/navigation", () => ({
  useSearchParams: () => useSearchParamsMock(),
}));

vi.mock("@/lib/video-kpi-events", () => ({
  emitVideoKPIEvent: (...args: unknown[]) => emitVideoKPIEvent(...args),
}));

vi.mock("./video-overlay-components", () => ({
  accentForEpisode: (episode?: string) => {
    const accents = ["#66d4cf", "#9af0a8", "#ffd485", "#c6a8ff", "#7cc7ff", "#ff9db1"] as const;
    const n = Number.parseInt(episode ?? "", 10);
    if (Number.isFinite(n) && n > 0) {
      return accents[(n - 1) % accents.length];
    }
    return accents[0];
  },
  EpisodePill: ({ label }: { label: string }) => <span>{label}</span>,
  GlyphBadge: ({ label }: { label: string }) => <span>{label}</span>,
  ContentPanel: ({ overlays }: { overlays?: unknown[] }) =>
    overlays && overlays.length > 0 ? <div data-testid="overlay-renderer" /> : null,
  ChipRow: () => null,
  LadderRow: () => null,
  CounterRow: () => null,
}));

function createVideo(id: string, title: string): HomepageVideo {
  return {
    id,
    title,
    description: `${title} description`,
    category: "copilot",
    durationSec: 60,
    language: "nb",
    aspectRatio: "9:16",
    posterUrl: `/poster-${id}.jpg`,
    playUrl: `/play-${id}.m3u8`,
    metadata: {
      overlay: [{ kind: "chip", anchor: "bottom-left", labels: ["Test Chip"] }],
    },
  };
}

describe("ShortsFeed", () => {
  beforeAll(() => {
    Object.defineProperty(window, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation(() => ({
        matches: false,
        media: "(prefers-reduced-motion: reduce)",
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });

    Object.defineProperty(HTMLMediaElement.prototype, "play", {
      configurable: true,
      value: vi.fn().mockResolvedValue(undefined),
    });

    Object.defineProperty(HTMLMediaElement.prototype, "pause", {
      configurable: true,
      value: vi.fn(),
    });

    Object.defineProperty(HTMLVideoElement.prototype, "requestFullscreen", {
      configurable: true,
      value: vi.fn().mockResolvedValue(undefined),
    });
  });

  beforeEach(() => {
    emitVideoKPIEvent.mockClear();
    window.localStorage.clear();
    window.history.replaceState(null, "", "/");
    useSearchParamsMock.mockReturnValue({
      get: (key: string) => new URLSearchParams(window.location.search).get(key),
    });
  });

  afterEach(() => {
    window.history.replaceState(null, "", "/");
  });

  it("emits feed impression only once and only when videos exist", () => {
    const { rerender } = render(<ShortsFeed videos={[]} />);
    expect(emitVideoKPIEvent).not.toHaveBeenCalled();

    rerender(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);
    expect(emitVideoKPIEvent).toHaveBeenCalledTimes(1);
    expect(emitVideoKPIEvent).toHaveBeenCalledWith("video_feed_impression", { videoCount: 1 });

    rerender(<ShortsFeed videos={[createVideo("video-a", "Video A"), createVideo("video-b", "Video B")]} />);
    expect(emitVideoKPIEvent).toHaveBeenCalledTimes(1);
  });

  it("keeps the active video element mounted when watch-state reorder happens", async () => {
    render(<ShortsFeed videos={[createVideo("video-b", "Video B"), createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video B" })[0]);

    const before = document.querySelector('video[data-video-id="video-b"]') as HTMLVideoElement;
    expect(before).toBeInTheDocument();
    Object.defineProperty(before, "duration", { configurable: true, value: 60 });

    fireEvent.ended(before);

    await waitFor(() => {
      expect(document.querySelector('video[data-video-id="video-b"]')).toBeInTheDocument();
    });
    const after = document.querySelector('video[data-video-id="video-b"]');
    expect(after).toBe(before);
  });

  it("opens a shared video from the initial query param", () => {
    render(
      <ShortsFeed
        videos={[createVideo("video-a", "Video A"), createVideo("video-b", "Video B")]}
        initialVideoId="video-b"
      />
    );

    expect(document.querySelector('video[data-video-id="video-b"]')).toBeInTheDocument();
    // Overlay renderer should be visible (there are multiple videos, so use getAllByTestId)
    expect(screen.getAllByTestId("overlay-renderer").length).toBeGreaterThan(0);
  });

  it("toggles overlay state when the active video plays and pauses", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const video = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    expect(video).toBeInTheDocument();
    // Old HUD should be visible when paused
    expect(screen.getByTestId("overlay-renderer")).toBeInTheDocument();

    fireEvent.play(video);
    // Old HUD should NOT be visible when playing
    expect(screen.queryByTestId("overlay-renderer")).not.toBeInTheDocument();

    fireEvent.pause(video);
    // Old HUD should be visible again when paused
    expect(screen.getByTestId("overlay-renderer")).toBeInTheDocument();
  });

  it("keeps viewer state when opening without url video param", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const video = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    fireEvent.play(video);

    // HUD stays visible during playback
    expect(screen.getByText("1:00")).toBeInTheDocument();
  });

  it("shows a share link for the active card", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    expect(screen.getByRole("link", { name: "Del video: Video A" })).toHaveAttribute(
      "href",
      expect.stringMatching(/\/videos\/video-a$/)
    );
  });

  it("copies share link when pressing Del", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getByRole("link", { name: "Del video: Video A" }));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(expect.stringMatching(/\/videos\/video-a$/));
    });
  });

  it("shows a pause button while playing and pauses the active video", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const video = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    fireEvent.play(video);

    const pauseButton = screen.getByRole("button", { name: "Sett på pause: Video A" });
    expect(pauseButton).toBeVisible();

    fireEvent.click(pauseButton);
    expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
    fireEvent.pause(video);
    // After pausing, content panel should be rendered
    expect(screen.getByTestId("overlay-renderer")).toBeInTheDocument();
  });

  it("does not pause active playback when inactive videos are paused internally", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A"), createVideo("video-b", "Video B")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const activeVideo = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    const inactiveVideo = document.querySelector('video[data-video-id="video-b"]') as HTMLVideoElement;

    fireEvent.play(activeVideo);
    // When playing, HUD is visible with duration
    expect(screen.getAllByText("1:00").length).toBeGreaterThan(0);

    fireEvent.pause(inactiveVideo);
    // Active video should still be playing with HUD visible
    expect(screen.getAllByText("1:00").length).toBeGreaterThan(0);
  });

  it("does not switch HUD to playing when inactive videos fire play events", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A"), createVideo("video-b", "Video B")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const inactiveVideo = document.querySelector('video[data-video-id="video-b"]') as HTMLVideoElement;

    fireEvent.play(inactiveVideo);

    expect(screen.queryByRole("button", { name: "Sett på pause: Video A" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Spill av video: Video A" })).toBeInTheDocument();
  });

  it("shows a completed overlay with replay and copy actions", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const video = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    fireEvent.play(video);
    fireEvent.ended(video);

    // Check for replay and copy buttons (CompletedOverlay)
    expect(screen.getByRole("button", { name: "Spill av på nytt: Video A" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Kopier lenke for Video A" })).toBeInTheDocument();
  });

  it("shows a fullscreen action for the active video", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    expect(screen.getByRole("button", { name: "Gå til fullskjerm for Video A" })).toBeInTheDocument();
  });

  it("seeks 5 seconds backward and forward from transport controls", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const video = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    Object.defineProperty(video, "currentTime", { configurable: true, writable: true, value: 30 });
    Object.defineProperty(video, "duration", { configurable: true, value: 120 });
    fireEvent.play(video);

    fireEvent.click(screen.getByRole("button", { name: "Spol 5 sek tilbake for Video A" }));
    expect(video.currentTime).toBe(25);

    fireEvent.click(screen.getByRole("button", { name: "Spol 5 sek frem for Video A" }));
    expect(video.currentTime).toBe(30);
  });

  it("keeps skip controls visible while paused", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);
    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);

    expect(screen.getByRole("button", { name: "Spol 5 sek tilbake for Video A" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Spol 5 sek frem for Video A" })).toBeInTheDocument();
  });

  it("replays from the completed overlay (resets position and plays again)", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const video = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    Object.defineProperty(video, "currentTime", { configurable: true, writable: true, value: 60 });
    fireEvent.play(video);
    fireEvent.ended(video);

    (HTMLMediaElement.prototype.play as ReturnType<typeof vi.fn>).mockClear();
    fireEvent.click(screen.getByRole("button", { name: "Spill av på nytt: Video A" }));

    expect(video.currentTime).toBe(0);
    expect(HTMLMediaElement.prototype.play).toHaveBeenCalled();
    // REPLAY transition moves the HUD back into playing immediately.
    expect(screen.getByRole("button", { name: "Sett på pause: Video A" })).toBeInTheDocument();
  });

  it("flushes exact video position to storage when pausing (not just 5-sec boundary)", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const video = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    // Set to 17 seconds (not a 5-second boundary)
    Object.defineProperty(video, "currentTime", { configurable: true, writable: true, value: 17 });
    Object.defineProperty(video, "duration", { configurable: true, value: 60 });

    fireEvent.play(video);
    fireEvent.pause(video);

    // Verify that the exact position (17s) is stored in localStorage
    const watchState = JSON.parse(window.localStorage.getItem("my-copilot:shorts:watch-state:v1") || "{}");
    expect(watchState.videos?.["video-a"]?.lastPositionSec).toBe(17);
  });

  it("flushes video position and marks complete when video ends", () => {
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const video = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    Object.defineProperty(video, "currentTime", { configurable: true, writable: true, value: 60 });
    Object.defineProperty(video, "duration", { configurable: true, value: 60 });

    fireEvent.play(video);
    fireEvent.ended(video);

    // Verify that the position is flushed and video is marked as watched
    const watchState = JSON.parse(window.localStorage.getItem("my-copilot:shorts:watch-state:v1") || "{}");
    expect(watchState.videos?.["video-a"]?.watched).toBe(true);
    expect(watchState.videos?.["video-a"]?.lastPositionSec).toBe(60);
  });

  it("resumes from previously paused position after reloading", () => {
    // First render and pause at specific time
    const { unmount } = render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    fireEvent.click(screen.getAllByRole("button", { name: "Åpne video: Video A" })[0]);
    const video = document.querySelector('video[data-video-id="video-a"]') as HTMLVideoElement;
    Object.defineProperty(video, "currentTime", { configurable: true, writable: true, value: 23 });
    Object.defineProperty(video, "duration", { configurable: true, value: 60 });

    fireEvent.play(video);
    fireEvent.pause(video);

    unmount();

    // Re-render the component
    render(<ShortsFeed videos={[createVideo("video-a", "Video A")]} />);

    // Check localStorage was persisted with exact time
    const watchState = JSON.parse(window.localStorage.getItem("my-copilot:shorts:watch-state:v1") || "{}");
    expect(watchState.videos?.["video-a"]?.lastPositionSec).toBe(23);
  });
});

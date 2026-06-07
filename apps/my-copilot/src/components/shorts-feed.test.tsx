import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { HomepageVideo } from "@/lib/public-videos";
import { ShortsFeed } from "./shorts-feed";

const emitVideoKPIEvent = vi.fn();

vi.mock("@/lib/video-kpi-events", () => ({
  emitVideoKPIEvent: (...args: unknown[]) => emitVideoKPIEvent(...args),
}));

vi.mock("./video-overlay-renderer", () => ({
  VideoOverlayRenderer: () => <div data-testid="overlay-renderer" />,
}));

function createVideo(id: string, title: string): HomepageVideo {
  return {
    id,
    title,
    description: `${title} description`,
    category: "copilot",
    durationSec: 60,
    language: "nb",
    posterUrl: `/poster-${id}.jpg`,
    playUrl: `/play-${id}.m3u8`,
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
  });

  beforeEach(() => {
    emitVideoKPIEvent.mockClear();
    window.localStorage.clear();
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

    fireEvent.click(screen.getByRole("button", { name: "Åpne video: Video B" }));

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
});

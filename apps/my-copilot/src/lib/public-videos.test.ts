import { getPublicVideoFeed, fetchVideoById } from "./public-videos";
import videoFeedFixture from "./__fixtures__/video-feed-response.json";

describe("getPublicVideoFeed", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("preserves API ordering without frontend resorting", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify(videoFeedFixture), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );

    const result = await getPublicVideoFeed(10);

    expect(result.map((item) => item.id)).toEqual(["video-ordered-2", "video-ordered-1"]);
  });

  it("normalizes overlay highlight_index to highlightIndex", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify(videoFeedFixture), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );

    const result = await getPublicVideoFeed(10);

    const firstOverlay = result[0]?.metadata?.overlay?.[0];
    expect(firstOverlay?.kind).toBe("ladder");
    expect(firstOverlay?.highlightIndex).toBe(1);
    expect(firstOverlay && "highlight_index" in firstOverlay).toBe(false);
  });
});

describe("fetchVideoById", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("should fetch a single video by ID", async () => {
    const mockVideo = {
      id: "test-video-1",
      title: "Test Video",
      description: "Test Description",
      category: "nav-pilot",
      published_at: "2024-01-01T00:00:00Z",
      duration_sec: 120,
      aspect_ratio: "16:9",
      language: "no",
      poster_url: "https://example.com/poster.jpg",
      play_url: "https://example.com/video.m3u8",
    };

    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify(mockVideo), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );

    const result = await fetchVideoById("test-video-1");

    expect(result).not.toBeNull();
    expect(result?.id).toBe("test-video-1");
    expect(result?.title).toBe("Test Video");
    expect(result?.durationSec).toBe(120);
  });

  it("should return null for non-existent video (404)", async () => {
    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response("Not Found", {
        status: 404,
      })
    );

    const result = await fetchVideoById("non-existent");

    expect(result).toBeNull();
  });

  it("should encode special characters in video ID", async () => {
    const fetchSpy = vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify({}), {
        status: 200,
      })
    );

    await fetchVideoById("video/with/slashes");

    const callUrl = fetchSpy.mock.calls[0]?.[0];
    expect(callUrl).toContain("video%2Fwith%2Fslashes");
  });

  it("should return null for invalid video ID", async () => {
    const result = await fetchVideoById("");

    expect(result).toBeNull();
  });

  it("should normalize response snake_case to camelCase", async () => {
    const mockResponse = {
      id: "test-1",
      title: "Video Title",
      description: "Description",
      category: "nav-pilot",
      published_at: "2024-01-01T00:00:00Z",
      duration_sec: 300,
      aspect_ratio: "16:9",
      language: "no",
      poster_url: "https://example.com/poster.jpg",
      play_url: "https://example.com/play.m3u8",
      mp4_url: "https://example.com/video.mp4",
      captions_url: "https://example.com/captions.vtt",
      metadata: {
        series: "Test Series",
        season: 1,
        episode: 1,
        tags: ["test", "demo"],
      },
    };

    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify(mockResponse), {
        status: 200,
      })
    );

    const result = await fetchVideoById("test-1");

    expect(result).toBeDefined();
    expect(result?.id).toBe("test-1");
    expect(result?.durationSec).toBe(300);
    expect(result?.mp4Url).toBe("https://example.com/video.mp4");
    expect(result?.metadata?.tags).toEqual(["test", "demo"]);
  });
});

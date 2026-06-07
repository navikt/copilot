import { getPublicVideoFeed } from "./public-videos";
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

import { getPublicVideoFeed } from "./public-videos";

type PublicVideoFeedItemFixture = {
  id: string;
  published_at: string;
};

function makeItem(fixture: PublicVideoFeedItemFixture) {
  return {
    id: fixture.id,
    title: `Title ${fixture.id}`,
    description: `Desc ${fixture.id}`,
    category: "copilot",
    published_at: fixture.published_at,
    duration_sec: 60,
    aspect_ratio: "9:16",
    language: "nb",
    poster_url: `/poster-${fixture.id}.jpg`,
    play_url: `/play-${fixture.id}.m3u8`,
  };
}

describe("getPublicVideoFeed", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("sorts by newest published_at first and places invalid timestamps last", async () => {
    const payload = {
      items: [
        makeItem({ id: "older", published_at: "2026-01-01T10:00:00Z" }),
        makeItem({ id: "invalid", published_at: "not-a-date" }),
        makeItem({ id: "newer", published_at: "2026-02-01T10:00:00Z" }),
      ],
    };

    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify(payload), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );

    const result = await getPublicVideoFeed(10);

    expect(result.map((item) => item.id)).toEqual(["newer", "older", "invalid"]);
  });

  it("deduplicates duplicate IDs while preserving first occurrence", async () => {
    const payload = {
      items: [
        makeItem({ id: "dup", published_at: "2026-02-01T10:00:00Z" }),
        makeItem({ id: "unique", published_at: "2026-01-01T10:00:00Z" }),
        makeItem({ id: "dup", published_at: "2026-03-01T10:00:00Z" }),
      ],
    };

    vi.spyOn(global, "fetch").mockResolvedValue(
      new Response(JSON.stringify(payload), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    );

    const result = await getPublicVideoFeed(10);

    expect(result.map((item) => item.id)).toEqual(["dup", "unique"]);
    expect(result[0]?.title).toBe("Title dup");
  });
});

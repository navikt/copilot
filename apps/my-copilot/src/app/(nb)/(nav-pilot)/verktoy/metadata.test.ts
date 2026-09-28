import { generateMetadata } from "./page";

describe("/verktoy metadata", () => {
  it("bruker samme tittel i fanen og ved deling (#1096)", async () => {
    const meta = await generateMetadata({ searchParams: Promise.resolve({}) });
    expect(meta.openGraph?.title).toBe(meta.title);
    expect(meta.twitter?.title).toBe(meta.title);
  });
});

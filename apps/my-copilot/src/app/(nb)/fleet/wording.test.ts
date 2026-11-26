import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

// #3129: the actions are Publiser and Avpubliser; «Trekk tilbake» is gone from /fleet.
const SRC = path.resolve(__dirname, "../../..");
const DIRS = ["app/(nb)/fleet", "app/(nb)/(mcp)", "app/api/fleet", "lib", "components"];
const OLD = /trekk(e|er)? tilbake|trukket tilbake|tilbaketrukket|kommer i fleet/i;

const files = (dir: string): string[] =>
  readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? files(path.join(dir, e.name)) : /\.tsx?$/.test(e.name) ? [path.join(dir, e.name)] : []
  );

describe("fleet wording", () => {
  it("says Avpubliser, not «Trekk tilbake»", () => {
    const hits = DIRS.flatMap((d) => files(path.join(SRC, d)))
      .filter((f) => !f.includes(".test.") && OLD.test(readFileSync(f, "utf8")))
      .map((f) => path.relative(SRC, f));
    expect(hits).toEqual([]);
  });
});

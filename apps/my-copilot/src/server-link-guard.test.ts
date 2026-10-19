// Aksel `<Link as={NextLink}>` passes a function to a client component, so a
// server component that renders it throws at render time. `mise run smoke` only
// reaches public pages, so a private page with this pattern ships broken; this
// test catches it in every file under src/app that is not a client component.
import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

function serverFiles(dir: string): string[] {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) => {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) return serverFiles(p);
    if (!/\.tsx$/.test(e.name) || /\.test\.tsx$/.test(e.name)) return [];
    return /^\s*["']use client["']/.test(fs.readFileSync(p, "utf8")) ? [] : [p];
  });
}

describe("server components", () => {
  it("never render Aksel Link with as={NextLink}", () => {
    const offenders = serverFiles(path.join(__dirname, "app")).filter((f) =>
      fs.readFileSync(f, "utf8").includes("as={NextLink}")
    );
    expect(offenders.map((f) => path.relative(__dirname, f))).toEqual([]);
  });
});

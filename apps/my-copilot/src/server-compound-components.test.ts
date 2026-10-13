import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

// A server component that imports a client component gets a client reference,
// and its static fields (`Table.Row`, `Tabs.List`, …) are undefined. React then
// throws #130 («Element type is invalid … got: undefined»); this took down
// /nav-pilot/docs in #628. Use the wrappers in components/aksel-table.tsx, or move the
// markup into a "use client" file.
const root = join(__dirname);

function files(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? files(join(dir, e.name)) : e.name.endsWith(".tsx") ? [join(dir, e.name)] : []
  );
}

describe("server components", () => {
  it("do not render compound components through a client reference", () => {
    const offenders = files(root)
      .filter((f) => !/\.(test|stories)\.tsx$/.test(f))
      .flatMap((f) => {
        const source = readFileSync(f, "utf8");
        if (/^\s*["']use client["']/m.test(source)) return [];
        return [...source.matchAll(/<([A-Z]\w*)\.[A-Z]\w*/g)]
          .filter((m) => m[1] !== "React")
          .map((m) => `${f.slice(root.length + 1)}: ${m[0]}`);
      });
    expect(offenders).toEqual([]);
  });
});

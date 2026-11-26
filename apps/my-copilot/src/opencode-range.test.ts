// The tested OpenCode range on the site is a literal. This keeps it equal to
// OpenCode2TestedRange in nav-pilot's Go code.
import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

const repo = path.resolve(__dirname, "../../..");
const read = (p: string) => fs.readFileSync(path.join(repo, p), "utf8");
const goSrc = read("cli/nav-pilot/internal/provider/opencode_policy.go");
const goRange = (name: string) => goSrc.match(new RegExp(`${name}\\s*=\\s*"([^"]+)"`))?.[1];
const ranges = [goRange("OpenCode2TestedRange")];
const pages = "apps/my-copilot/src/app/(nb)/(nav-pilot)/nav-pilot";
const unescape = (s: string) => s.replaceAll("&quot;", '"').replaceAll("&gt;", ">").replaceAll("&lt;", "<");

describe("OpenCode tested range", () => {
  it("is found in the Go code", () => {
    expect(ranges).toEqual([expect.any(String)]);
  });

  // A tested range has a minor-version upper bound (<2.2); the package range
  // example ">=1.18.20,<2" on the referanse page does not.
  it.each(["klienter/page.tsx", "agentpakker/referanse/page.tsx"])("%s matches the Go code", (page) => {
    const onPage = [...unescape(read(`${pages}/${page}`)).matchAll(/>=\d+\.\d+\.\d+,<\d+\.\d+/g)].map((m) => m[0]);
    expect(new Set(onPage)).toEqual(new Set(ranges));
  });
});

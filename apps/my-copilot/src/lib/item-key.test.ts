import { findByItemKey, itemKey } from "./item-key";

describe("item keys (#1036)", () => {
  const items = [
    { type: "instruction", id: "security-owasp" },
    { type: "skill", id: "security-owasp" },
  ];

  it("finds each of two items that share an id", () => {
    expect(findByItemKey(items, itemKey(items[1]))).toBe(items[1]);
    expect(findByItemKey(items, "instruction:security-owasp")).toBe(items[0]);
  });

  it("still resolves a bare id from an older link", () => {
    expect(findByItemKey(items, "security-owasp")).toBe(items[0]);
    expect(findByItemKey(items, "agent:security-owasp")).toBeUndefined();
  });
});

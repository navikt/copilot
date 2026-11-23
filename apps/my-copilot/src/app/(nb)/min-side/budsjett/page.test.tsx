import { describe, expect, it, vi } from "vitest";

// No Authorization header: the real getUser() must send the reader to the login.
vi.mock("next/headers", () => ({ headers: async () => new Headers() }));
vi.mock("next/navigation", () => ({
  redirect: vi.fn((url: string) => {
    throw new Error(`redirect:${url}`);
  }),
  notFound: vi.fn(),
}));

import BudsjettPage from "./page";

describe("/min-side/budsjett", () => {
  it("redirects to the login without a token, even though Wonderwall lets the path through", async () => {
    await expect(BudsjettPage()).rejects.toThrow(/^redirect:/);
  });
});

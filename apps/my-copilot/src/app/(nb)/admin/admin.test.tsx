import { vi } from "vitest";

vi.mock("next/headers", () => ({ headers: vi.fn() }));
vi.mock("next/navigation", () => ({
  notFound: vi.fn(() => {
    throw new Error("NEXT_NOT_FOUND");
  }),
  redirect: vi.fn(),
}));

import AdminLayout from "./layout";
import AdminPage from "./page";
import CostPage from "./kostnad/page";

describe("/admin", () => {
  const group = "11111111-2222-3333-4444-555555555555";
  beforeEach(() => {
    vi.unstubAllEnvs();
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NAIS_TOKEN_INTROSPECTION_ENDPOINT", "");
    vi.stubEnv("COPILOT_ADMIN_GROUP_ID", group);
    vi.stubEnv("DEV_USER_GROUPS", "other");
  });

  it("gives a non-admin 404 in the layout, the index and /admin/kostnad", async () => {
    await expect(AdminLayout({ children: null })).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(AdminPage()).rejects.toThrow("NEXT_NOT_FOUND");
    await expect(CostPage()).rejects.toThrow("NEXT_NOT_FOUND");
  });

  it("lets an admin through the layout", async () => {
    vi.stubEnv("DEV_USER_GROUPS", `other,${group}`);
    await expect(AdminLayout({ children: "ok" })).resolves.toBe("ok");
  });
});

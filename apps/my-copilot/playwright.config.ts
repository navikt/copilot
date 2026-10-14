import { defineConfig, devices } from "@playwright/test";

// Runs against two production builds made by hack/e2e-build.sh (mise run e2e).
export default defineConfig({
  testDir: "e2e",
  workers: 1,
  timeout: 60_000,
  reporter: process.env.CI ? [["list"], ["github"]] : "list",
  use: { ...devices["Desktop Chrome"], trace: "retain-on-failure" },
});

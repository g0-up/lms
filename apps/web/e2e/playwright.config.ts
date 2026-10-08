import { defineConfig, devices } from "@playwright/test";

// E2E trên stack container thật (compose profile "full": web nginx :8081 → api); không dùng webServer.
// Mọi spec dùng chung một DB seed: chạy tuần tự một worker, không retry (trừ job webkit trên CI).
const CI = !!process.env.CI;
const E2E_SPECS = /(content-versioning|content-rules|invitations|auth|learning|reports|lesson-editor)\.spec\.ts$/;

export default defineConfig({
  testDir: ".",
  globalSetup: "./global-setup.ts",
  workers: 1,
  fullyParallel: false,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  outputDir: "test-results",
  snapshotPathTemplate: "{testDir}/__screenshots__/{arg}{ext}",
  reporter: CI ? [["github"], ["html", { open: "never", outputFolder: "playwright-report" }], ["list"]] : [["list"]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:8081",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    locale: "vi-VN",
    timezoneId: "Asia/Ho_Chi_Minh",
  },
  projects: [
    { name: "e2e", testMatch: E2E_SPECS, use: { ...devices["Desktop Chrome"] } },
    // Cùng bộ spec trên WebKit: chỉ chạy ở job CI riêng (sau chromium), là nơi duy nhất cho phép retry.
    { name: "webkit", testMatch: E2E_SPECS, retries: CI ? 1 : 0, use: { ...devices["Desktop Safari"] } },
    { name: "a11y", testMatch: /a11y\.spec\.ts$/, use: { ...devices["Desktop Chrome"] } },
    // So với prototype tĩnh (:8090); không nằm trong test:e2e.
    { name: "render", testMatch: /render-check\.spec\.ts$/, use: { ...devices["Desktop Chrome"] } },
  ],
});

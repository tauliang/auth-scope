export default {
  testDir: "./playwright",
  outputDir: "./test-results",
  timeout: 90000,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"], ["html", { outputFolder: "./playwright-report", open: "never" }]],
  use: {
    baseURL: process.env.AUTH_SCOPE_FRONTEND_URL ?? process.env.FRONTEND_URL ?? "http://127.0.0.1:3000",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "demo-chromium",
      use: {
        browserName: "chromium",
        viewport: { width: 1440, height: 960 },
      },
    },
  ],
};

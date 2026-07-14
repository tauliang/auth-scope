import { expect, test } from "@playwright/test";

const principal = { subject: "alice@example.com", issuer: "https://idp.example.com" };

test.beforeEach(async ({ page }) => {
  await page.route(/\/api\/v1\//, async (route) => {
    const path = new URL(route.request().url()).pathname.replace(/^\/api/, "");
    let body: unknown;
    if (path === "/v1/admin/session") {
      body = { principal, capabilities: { approve: true }, api_version: "v1" };
    } else if (path === "/v1/operations/summary") {
      body = { missions_total: 0, missions_by_state: {}, pending_proposals: 0, pending_expansions: 0, active_containments: 0, active_agents: 0, active_projections: 0, recent_event_count: 0, service_capabilities: {} };
    } else {
      body = { items: [], total: 0 };
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      headers: { "x-request-id": "req-e2e" },
      body: JSON.stringify(body),
    });
  });
});

test("authenticates, navigates, and keeps the bearer credential ephemeral", async ({ page }, testInfo) => {
  await page.goto("/");
  await page.getByLabel("Bearer token").fill("e2e-admin-token");
  await page.getByRole("button", { name: "Open console" }).click();
  await expect(page.getByRole("heading", { name: "Authority overview" })).toBeVisible();

  if (testInfo.project.name === "mobile") {
    await page.getByRole("button", { name: "Open navigation" }).click();
  }
  await page.getByRole("link", { name: "Missions" }).click();
  await expect(page.getByRole("heading", { name: "Missions" })).toBeVisible();
  await expect(page.getByText("No missions found")).toBeVisible();

  const persisted = await page.evaluate(() => ({
    local: Object.keys(localStorage),
    session: Object.keys(sessionStorage),
    url: location.href,
  }));
  expect(persisted.local).toEqual([]);
  expect(persisted.session).toEqual([]);
  expect(persisted.url).not.toContain("e2e-admin-token");
});

test("rejects empty administrator credentials", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Open console" }).click();
  await expect(page.getByRole("alert")).toContainText("Administrator token is required");
});

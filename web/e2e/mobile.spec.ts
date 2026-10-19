import { expect, type Page, test } from "@playwright/test";
import { receiptPng, shot } from "./helpers";

async function login(page: Page) {
  const setup = await page.request.post("/api/setup", {
    data: { token: "e2e-setup-token", username: "user-a", name: "User A", password: "correct horse battery" },
  });
  if (!setup.ok()) {
    const res = await page.request.post("/api/login", { data: { username: "user-a", password: "correct horse battery" } });
    expect(res.ok()).toBeTruthy();
  }
}

async function noHorizontalScroll(page: Page) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow, `horizontal overflow on ${page.url()}`).toBeLessThanOrEqual(0);
}

/** Main pages are usable at 360 px without horizontal scrolling (P19, A21, A26). */
test("mobile layout and receipt capture", async ({ page }) => {
  await login(page);
  for (const [path, name] of [
    ["/", "m-01-dashboard"],
    ["/trips", "m-02-trips"],
    ["/trips/new", "m-03-trip-form"],
    ["/outlays", "m-04-outlays"],
    ["/claims", "m-05-claims"],
    ["/settings", "m-06-settings"],
  ] as const) {
    await page.goto(path);
    await expect(page.getByRole("navigation", { name: "Hauptnavigation" })).toBeVisible();
    await noHorizontalScroll(page);
    await shot(page, name);
  }

  // Capture via the camera button in the bottom bar, then confirm in the inbox.
  await page.goto("/");
  await page.locator('nav input[type="file"]').setInputFiles({ name: "photo.png", mimeType: "image/png", buffer: receiptPng(42) });
  await expect(page).toHaveURL(/\/receipts\?focus=/);
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "Lesbar – bestätigen" })).toBeVisible();
  await noHorizontalScroll(page);
  await shot(page, "m-07-receipt-check");
  await dialog.getByRole("button", { name: "Lesbar – bestätigen" }).click();
  await expect(page.getByText(/Beleg B-\d{4}-\d{4} bestätigt/)).toBeVisible();

  // Claim detail of F01 (if present) also fits.
  await page.goto("/claims");
  const claim = page.getByText("Q2 2026");
  if (await claim.count()) {
    await claim.first().click();
    await noHorizontalScroll(page);
    await shot(page, "m-08-claim");
  }
});

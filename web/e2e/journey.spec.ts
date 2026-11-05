import { expect, test } from "@playwright/test";

/** Journeys store exactly the entered total distance (C14); there is no round-trip toggle. */
test("journey distance is stored and shown as entered", async ({ page }) => {
  const setup = await page.request.post("/api/setup", {
    data: { token: "e2e-setup-token", username: "user-a", name: "User A", password: "correct horse battery" },
  });
  if (!setup.ok()) {
    const res = await page.request.post("/api/login", {
      data: { username: "user-a", password: "correct horse battery" },
    });
    expect(res.ok()).toBeTruthy();
  }

  await page.goto("/trips/new");
  await page.getByLabel("Anlass", { exact: true }).fill("Fahrtentest");
  await page.getByLabel("Abfahrt", { exact: true }).fill("2026-05-04");
  await page.getByLabel("Rückkehr", { exact: true }).fill("2026-05-04");
  await page.getByRole("button", { name: "Fahrt hinzufügen" }).click();
  await expect(page.getByText("Hin und zurück")).toHaveCount(0);
  await page.getByLabel("Strecke gesamt (km)", { exact: true }).fill("120");
  await page.getByRole("button", { name: "Reise speichern" }).click();
  await expect(page.getByRole("heading", { name: "Fahrtentest" })).toBeVisible();
  await expect(page.getByText(/120 km/)).toBeVisible();
  await expect(page.getByText("36,00 €").first()).toBeVisible();

  await page.goto(`${page.url()}/edit`);
  await expect(page.getByLabel("Strecke gesamt (km)", { exact: true })).toHaveValue("120");
  await expect(page.getByText("Hin und zurück")).toHaveCount(0);
});

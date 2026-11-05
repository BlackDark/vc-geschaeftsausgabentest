import { expect, test } from "@playwright/test";
import { loginWithEmployer } from "./helpers";

/** Journeys store exactly the entered total distance (C14); there is no round-trip toggle. */
test("journey distance is stored and shown as entered", async ({ page }) => {
  await loginWithEmployer(page);

  await page.goto("/trips/new");
  await page.getByLabel("Anlass", { exact: true }).fill("Fahrtentest");
  await page.getByLabel("Abfahrt", { exact: true }).fill("2026-09-07");
  await page.getByLabel("Rückkehr", { exact: true }).fill("2026-09-07");
  await page.getByRole("button", { name: "Fahrt hinzufügen" }).click();
  const km = page.getByLabel("Strecke gesamt (km)", { exact: true });
  await expect(km).toBeVisible();
  const row = km.locator("xpath=ancestor::div[contains(@class,'rounded-lg')][1]");
  await expect(row.getByRole("checkbox")).toHaveCount(0);
  await km.fill("120");
  await page.getByRole("button", { name: "Reise speichern" }).click();
  await expect(page.getByRole("heading", { name: "Fahrtentest" })).toBeVisible();
  await expect(page.getByText(/120 km/)).toBeVisible();
  await expect(page.getByText("36,00 €").first()).toBeVisible();

  await page.goto(`${page.url()}/edit`);
  const editKm = page.getByLabel("Strecke gesamt (km)", { exact: true });
  await expect(editKm).toHaveValue("120");
  const editRow = editKm.locator("xpath=ancestor::div[contains(@class,'rounded-lg')][1]");
  await expect(editRow.getByRole("checkbox")).toHaveCount(0);
});

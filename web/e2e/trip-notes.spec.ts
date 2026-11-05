import { expect, test } from "@playwright/test";
import { loginWithEmployer, noHorizontalScroll } from "./helpers";

/** Trip notes show on the read-only detail view, and only when present. */
test("trip detail shows notes", async ({ page }) => {
  await loginWithEmployer(page);
  await page.goto("/trips/new");
  await page.getByLabel("Anlass", { exact: true }).fill("Notizen-Reise");
  await page.getByLabel("Ziel", { exact: true }).fill("Köln");
  await page.getByLabel("Abfahrt", { exact: true }).fill("2026-06-08");
  await page.locator("#startTime").fill("08:00");
  await page.getByLabel("Rückkehr", { exact: true }).fill("2026-06-08");
  await page.locator("#endTime").fill("18:00");
  await page.getByRole("button", { name: "Reise speichern" }).click();
  await expect(page.getByRole("heading", { name: "Notizen-Reise" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Notizen", exact: true })).toHaveCount(0);

  await page.getByRole("link", { name: "Bearbeiten" }).click();
  await page.getByText("Weitere Angaben", { exact: true }).click();
  await page.getByLabel("Notizen", { exact: true }).fill("Parkticket im Auto");
  await page.getByRole("button", { name: "Änderungen speichern" }).click();
  await expect(page.getByRole("heading", { name: "Notizen-Reise" })).toBeVisible();
  await expect(page.getByText("Parkticket im Auto")).toBeVisible();

  await page.getByRole("link", { name: "Bearbeiten" }).click();
  await page.getByText("Weitere Angaben", { exact: true }).click();
  await page.getByLabel("Notizen", { exact: true }).fill("");
  await page.getByRole("button", { name: "Änderungen speichern" }).click();
  await expect(page.getByRole("heading", { name: "Notizen-Reise" })).toBeVisible();
  await expect(page.getByText("Parkticket im Auto")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Notizen", exact: true })).toHaveCount(0);

  // Whitespace-only notes are treated as empty.
  await page.getByRole("link", { name: "Bearbeiten" }).click();
  await page.getByText("Weitere Angaben", { exact: true }).click();
  await page.getByLabel("Notizen", { exact: true }).fill("   ");
  await page.getByRole("button", { name: "Änderungen speichern" }).click();
  await expect(page.getByRole("heading", { name: "Notizen-Reise" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Notizen", exact: true })).toHaveCount(0);

  // A long unbroken note wraps instead of scrolling the page sideways at 360 px.
  const unbroken = `https://example.com/${"a".repeat(320)}?q=${"b".repeat(40)}`;
  await page.setViewportSize({ width: 360, height: 740 });
  await page.getByRole("link", { name: "Bearbeiten" }).click();
  await page.getByText("Weitere Angaben", { exact: true }).click();
  await page.getByLabel("Notizen", { exact: true }).fill(unbroken);
  await page.getByRole("button", { name: "Änderungen speichern" }).click();
  await expect(page.getByText(unbroken)).toBeVisible();
  await noHorizontalScroll(page);
});

import { expect, test } from "@playwright/test";
import { receiptPng, shot } from "./helpers";

/**
 * Mandatory first working flow (docs/ACCEPTANCE.md, fixture F01): setup,
 * employer, domestic trip with hotel and breakfasts, outlays, advance,
 * quarterly claim (182.80), export, unlock, correction and version 2 (192.80).
 */
test("F01 first working flow", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/setup/);
  await page.getByLabel("Einrichtungs-Token", { exact: true }).fill("e2e-setup-token");
  await page.getByLabel("Name", { exact: true }).fill("User A");
  await page.getByLabel("Benutzername", { exact: true }).fill("user-a");
  await page.getByLabel("Passwort", { exact: true }).fill("correct horse battery");
  await page.getByRole("button", { name: "Konto anlegen" }).click();

  await expect(page).toHaveURL(/\/settings/);
  await page.getByLabel("Firmenname", { exact: true }).fill("Example Company");
  await page.getByLabel("Anschrift", { exact: true }).first().fill("Musterstraße 1\n10115 Berlin");
  await page.getByRole("button", { name: "Speichern" }).first().click();
  await expect(page.getByText("Gespeichert.")).toBeVisible();
  await shot(page, "01-settings");

  // Trip with two privately paid hotel nights.
  await page.goto("/trips/new");
  await page.getByLabel("Anlass", { exact: true }).fill("Kundentermin");
  await page.getByLabel("Ziel", { exact: true }).fill("Hamburg");
  await page.getByLabel("Abfahrt", { exact: true }).fill("2026-04-13");
  await page.locator("#startTime").fill("08:00");
  await page.getByLabel("Rückkehr", { exact: true }).fill("2026-04-15");
  await page.locator("#endTime").fill("18:00");
  await page.getByLabel("Nacht nach Mo., 13.04.", { exact: true }).selectOption("private_actual");
  await page.getByLabel("Nacht nach Di., 14.04.", { exact: true }).selectOption("private_actual");
  await shot(page, "02-trip-form");
  await page.getByRole("button", { name: "Reise speichern" }).click();
  await expect(page.getByRole("heading", { name: "Kundentermin" })).toBeVisible();

  // Hotel invoice with breakfast, receipt captured in the dialog.
  await page.getByRole("button", { name: "Ausgabe", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await dialog
    .locator('input[type="file"]')
    .first()
    .setInputFiles({ name: "hotel.png", mimeType: "image/png", buffer: receiptPng(1) });
  await expect(dialog.getByRole("button", { name: /unbestätigt/ })).toBeVisible();
  await dialog.getByLabel("Kostenart", { exact: true }).selectOption("lodging");
  await dialog.getByLabel("Datum", { exact: true }).fill("2026-04-15");
  await dialog.getByLabel("Betrag brutto", { exact: true }).fill("178,00");
  await dialog.getByLabel("Lieferant / Händler", { exact: true }).fill("Hotel Hafen");
  await dialog.getByLabel("Frühstück im Preis enthalten", { exact: true }).check();
  await shot(page, "03-expense-dialog");
  await dialog.getByRole("button", { name: "Speichern" }).click();
  await expect(page.getByText("Hotel Hafen")).toBeVisible();
  await expect(page.getByText("44,80 €").first()).toBeVisible();
  await expect(page.getByText("222,80 €").first()).toBeVisible();
  await shot(page, "04-trip-detail");

  // Standalone outlays with receipts.
  for (const [i, o] of [
    { type: "tools_equipment", date: "2026-04-16", amount: "40", text: "Akkuschrauber" },
    { type: "software_services", date: "2026-04-17", amount: "20", text: "Software-Abo" },
  ].entries()) {
    await page.goto("/outlays");
    await page.getByRole("button", { name: "Auslage" }).first().click();
    const d = page.getByRole("dialog");
    await d
      .locator('input[type="file"]')
      .first()
      .setInputFiles({ name: `outlay-${i}.png`, mimeType: "image/png", buffer: receiptPng(10 + i) });
    await expect(d.getByRole("button", { name: /unbestätigt/ })).toBeVisible();
    await d.getByLabel("Kostenart", { exact: true }).selectOption(o.type);
    await d.getByLabel("Datum", { exact: true }).fill(o.date);
    await d.getByLabel("Betrag brutto", { exact: true }).fill(o.amount);
    await d.getByLabel("Zweck", { exact: true }).fill(o.text);
    await d.getByRole("button", { name: "Speichern" }).click();
    await expect(page.getByText(o.text)).toBeVisible();
  }
  await shot(page, "05-outlays");

  // Confirm all three receipts in the inbox.
  await page.goto("/receipts");
  for (let i = 0; i < 3; i++) {
    await page.getByRole("tabpanel").getByRole("button").first().click();
    await page.getByRole("button", { name: "Lesbar – bestätigen" }).click();
    await expect(page.getByText(/Beleg B-\d{4}-\d{4} bestätigt/).last()).toBeVisible();
    await page.keyboard.press("Escape");
  }
  await shot(page, "06-receipts");

  // Advance and quarterly claim.
  await page.goto("/claims");
  await page.getByRole("button", { name: "Vorschuss erfassen" }).click();
  const adv = page.getByRole("dialog");
  await adv.getByLabel("Datum", { exact: true }).fill("2026-04-01");
  await adv.getByLabel("Betrag", { exact: true }).fill("100");
  await adv.getByRole("button", { name: "Speichern" }).click();
  await expect(page.getByText("Vorschuss gespeichert.")).toBeVisible();

  await page.getByRole("button", { name: "Abrechnung" }).first().click();
  const nc = page.getByRole("dialog");
  await nc.getByLabel("Von", { exact: true }).fill("2026-04-01");
  await nc.getByLabel("Bis", { exact: true }).fill("2026-06-30");
  await nc.getByLabel("Bezeichnung (optional)", { exact: true }).fill("Q2 2026");
  await nc.getByRole("button", { name: "Abrechnung erstellen" }).click();
  await expect(page.getByRole("heading", { name: /Q2 2026/ })).toBeVisible();
  await expect(page.getByText("182,80 €").first()).toBeVisible();
  await shot(page, "07-claim-draft");
  await page.getByRole("button", { name: "Einreichen" }).click();
  await expect(page.getByText(/Abrechnung A-\d{4}-\d{3} eingereicht/)).toBeVisible();
  await expect(page.getByText("Eingereicht", { exact: true }).first()).toBeVisible();
  await shot(page, "08-claim-submitted");
  const pdfHref = await page.getByRole("link", { name: "PDF öffnen" }).getAttribute("href");
  const pdf1 = await page.request.get(pdfHref!);
  expect(pdf1.headers()["content-type"]).toBe("application/pdf");
  const pdf1Bytes = await pdf1.body();

  // Unlock, correct the software outlay and resubmit.
  await page.getByRole("button", { name: "Zur Korrektur entsperren" }).click();
  await page.getByRole("dialog").getByLabel("Begründung", { exact: true }).fill("Softwarebetrag korrigiert");
  await page.getByRole("dialog").getByRole("button", { name: "Bestätigen" }).click();
  await expect(page.getByText("Entwurf", { exact: true }).first()).toBeVisible();
  await page.goto("/outlays");
  await page.getByText("Software-Abo").click();
  await page.getByRole("dialog").getByLabel("Betrag brutto", { exact: true }).fill("30");
  await page.getByRole("dialog").getByRole("button", { name: "Speichern" }).click();
  await page.goto("/claims");
  await page.getByText("Q2 2026").click();
  await expect(page.getByText("192,80 €").first()).toBeVisible();
  await page.getByRole("button", { name: "Korrigierte Fassung einreichen" }).click();
  await expect(page.getByText("Version 2").first()).toBeVisible();
  await expect(page.getByText("ersetzt")).toBeVisible();
  await shot(page, "09-claim-v2");

  const v1 = await page.request.get(pdfHref!.replace(/versions\/\d+/, "versions/1"));
  expect(Buffer.compare(await v1.body(), pdf1Bytes)).toBe(0);
});

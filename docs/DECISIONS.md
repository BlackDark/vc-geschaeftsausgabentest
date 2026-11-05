# Decisions

Scope decisions, working defaults and open verification items. Hard-to-reverse trade-offs are
recorded as [ADRs](adr/); vocabulary is in [GLOSSARY.md](../GLOSSARY.md); what is implemented is
in [STATUS.md](STATUS.md).

## Owner decisions 2026-10-19

These decisions take precedence over conflicting statements in [SPECIFICATION.md](SPECIFICATION.md) and [ACCEPTANCE.md](ACCEPTANCE.md).

| Topic | Decision |
| --- | --- |
| Purpose | Personal tool for one employed managing director; single-user UI on a multi-user-capable model, see [ADR 0004](adr/0004-multi-user-model-single-user-ui.md). |
| Stack | See [ARCHITECTURE.md](ARCHITECTURE.md) and [ADR 0001](adr/0001-go-huma-hey-api-stack.md). |
| Employers | One employer profile per user in the UI (P03 reduced); see [ADR 0004](adr/0004-multi-user-model-single-user-ui.md). |
| Login | Password and OIDC (Pocket ID). Trusted-proxy header login (A19) is deferred. The password login stays as an emergency route when the identity provider is unavailable. |
| FX | Manual rate or actual EUR debit only. Automatic ECB lookup is deferred. |
| Rate tables | Inherited 2024–2026 CSVs are seeded; a further year is imported through a validated CLI command instead of an admin UI. Admin draft overrides remain. |
| Export | See [ADR 0005](adr/0005-export-pdf-with-all-receipt-pages-and-zip.md). |
| Capture | Mobile camera capture and desktop upload are equally important. |
| Trip types | Domestic multi-day, foreign, one-day and private-car mileage are all needed. |
| Priorities | Ease of use and a well-structured, readable PDF are the owner's main quality goals. |
| License | AGPL-3.0-only. |

## Implementation decisions 2026-10-19

Ordinary implementation choices made without changing scope or financial results.

| Topic | Decision |
| --- | --- |
| Name | The application is called "Spesen"; binary `spesen`, image `ghcr.io/blackdark/vc-geschaeftsausgabentest`. |
| Numbers | Receipts `B-<year>-<0001>` per user and confirmation year; claims `A-<year>-<001>` per user, employer and year of first submission. Allocated inside the committing transaction, so failed attempts consume nothing. |
| Revision check | The preview returns a revision hash of the claim's inputs and results; submission is refused when it changed (warnings are acknowledged for exactly what was shown). Transactional submission: [ADR 0002](adr/0002-single-connection-sqlite-transactional-submission.md). |
| Nights | Each night of a multi-day trip needs an explicit arrangement; an unset night is a blocker. A two-date trip with "no overnight stay" is treated as one-day travel (C05). |
| Hotel breakfast | A lodging invoice marked "breakfast included" sets a provided breakfast on the following day once (origin recorded, editable, audited). |
| Receipt validation | Uploads are detected by content and then rendered by Typst; a file Typst cannot read is rejected. The rendered PNG is the preview (no EXIF). |
| PDF | Inter font (OFL) bundled; summary, per-trip calculation, notes, sources, confirmation lines, receipt index and every receipt page. |
| Generated code | Hey API runs with TypeScript 6 in `web/tools/codegen` because TypeScript 7.0 lacks the compiler API; the app itself is type-checked with TypeScript 7. |
| pnpm | 12, pinned in `mise.toml` and `packageManager`. Its default one-day minimum release age stays on; `minimumReleaseAgeExclude` in `pnpm-workspace.yaml` lists versions taken in on purpose before that day passed. |
| Theme | Dark is the default for new accounts and before login (owner request); light and system remain selectable in the settings. The claim PDF stays light for printing. |
| Dependency updates | Renovate with the owner's agent-presets template (`.github/renovate.json5`): weekly, grouped, patch/minor automerged after CI, majors by hand. The codegen package stays on TypeScript 6 (Hey API needs the compiler API); Typst bumps are manual because the Dockerfile pins checksums. |

## Working defaults

| Topic | Decision |
| --- | --- |
| Compression | JPEG/PNG derivatives with visual checks; no mandatory AVIF/WebP encoder or automated cleanup. An earlier benchmark (one receipt fixture plus an upscaled version, not reproduced) put AVIF at 4.4 s per receipt and 112 s per A4 page. |
| Accounting | Preserve and report VAT data and evidence flags; defer automatic input-tax eligibility and hospitality deductibility claims. Reimbursement and employer tax accounting are distinct. |
| Difficult itineraries | Confirmed manual day schedules for special flight/ship/date-line cases (C09); ordinary foreign travel is Core. |
| Three-month limit | A screening warning with explicit exclusion/assessment, not an automated eligibility engine (C11). |
| Domain names | English terms in code and API, mapped to German specialist terms in [GLOSSARY.md](../GLOSSARY.md). |

## Open before production use

1. Validate the rate datasets and the detailed tax treatment of reimbursed own meals and hospitality, lodging eligibility, foreign VAT and unusual itineraries. Full gross reimbursement is not always tax-free.
2. Define a retention, deletion, storage-location and operating policy (see [ADR 0003](adr/0003-keep-original-receipt-files.md)). The earlier specification and operations guide disagreed on the numbering start year (receipt date versus confirmation year); the implementation uses the confirmation year.
3. Measure original-file storage cost and privacy on representative receipts on target hardware before deciding on compression targets or native codecs.
4. Consider PDF/A if operational needs warrant it.

These do not block use with synthetic data. They limit production claims.

## Tax and source verification

[§ 9 EStG](https://www.gesetze-im-internet.de/estg/__9.html) (meal-allowance thresholds, supplied-meal deductions, three-month and interruption rules) and R 9.7 LStR (lodging: actual or allowance; employer-provided lodging excluded) support the corresponding rules in [CALCULATION_RULES.md](CALCULATION_RULES.md). Other legal details and the complete annual datasets were not independently revalidated; the unverified items are listed in [STATUS.md](STATUS.md#unverified-rules-and-data). The 2024–2026 foreign-rate CSVs are described in [data/README.md](../data/README.md). Before production use, record sources and effective dates for every enabled rate and supported rule.

Starting points:

- [§ 3 EStG](https://www.gesetze-im-internet.de/estg/__3.html): employer reimbursement exemption.
- [§ 9 EStG](https://www.gesetze-im-internet.de/estg/__9.html): qualifying travel and meal rules.
- [Official 2026 foreign-rate letter](https://www.bundesfinanzministerium.de/Content/DE/Downloads/BMF_Schreiben/Steuerarten/Lohnsteuer/2025-12-05-steuerliche-behandlung-reisekosten-2026.pdf?__blob=publicationFile): foreign-rate values and special cases. Retrieve the corresponding official 2024/2025 letters for those datasets.
- [BMF wage-tax guidance](https://lsth.bundesfinanzministerium.de/): detailed travel, lodging and provided-meal guidance, especially the 2020 travel-expense letter and R 9.6/R 9.7.
- [§ 33 UStDV](https://www.gesetze-im-internet.de/ustdv_1980/__33.html), [§ 15 UStG](https://www.gesetze-im-internet.de/ustg_1980/__15.html): invoices and input-tax requirements.
- [§ 147 AO](https://www.gesetze-im-internet.de/ao_1977/__147.html): retention; also review the applicable GoBD and operator process.
- [ECB Data API](https://data.ecb.europa.eu/help/api/data): exchange-rate interface and publication conventions.

This documentation does not certify GoBD compliance or replace the employer's tax review.

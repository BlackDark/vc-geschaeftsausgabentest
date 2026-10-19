# Decisions and provenance

Baseline date: 2026-10-18. These documents describe a fresh build. They make no claim that any requirement has been implemented.

## Owner-confirmed direction

- Derive a compact, independent brief from the existing documentation, assuming an empty application; write English documents and filenames, retaining useful German specialist terms.
- Keep the previously chosen technology stack, while reconsidering PDF/image compression.
- Include the proposed travel/expenses/calculation/export Core **plus OIDC and trusted-proxy login**.
- After Core, implement **S3 → mail import → AI receipt reading**, in that order.
- Keep one interface with data isolated per user; no separate role-specific application surfaces.
- Include company-paid travel costs, privately paid actual-cost/eligible-allowance lodging and standalone business outlays such as tools/software in a common quarterly/custom claim.
- Add a small shared fixture package and first working flow; keep original files and use PDF+ZIP for Core.

## Inherited product decisions retained

Employer reimbursement is the primary use case. Users own separate records, even when an admin shares the instance. Employers are separate per-user objects. The original restriction to personally paid, trip-bound expenses is superseded: record the payer explicitly and support standalone outlays. Claims are versioned documents with locking, reasoned unlocking and advances. German/English UI, date-based rate tables, explicit VAT portions and documented FX remain part of the product. Go/SQLite in one application container and Svelte/Vite remain the stack.

## Domain update: travel and standalone outlays

The employed managing director can pay a travel charge personally or through the company. Record the payer and reimbursement method separately: employer-paid lodging is informational with zero lodging payout; privately paid lodging can use actual-cost reimbursement or an eligible allowance, never both. Provided meals are tracked regardless of payer.

Use **Sonstige Auslage / Standalone outlay** for a personally paid purchase without a trip. Tools and software are examples. A single Abrechnung/Reimbursement claim groups trips, standalone outlays and advances, with separate subtotals and one payout. Trips are selected by end date, outlays by expense date. Outlay-only claims are supported. No banking integration or fixed-asset/subscription accounting is implied.

This deliberately supersedes the earlier glossary/model restriction that every expense belongs to a trip. GLOSSARY.md remains the sole domain glossary; no duplicate CONTEXT.md is introduced.

## New defaults for this experiment

These decisions combine the owner-confirmed scope above with explicit working defaults. The owner has accepted originals plus PDF/ZIP, a small fixture package and a first working flow; the remaining detailed defaults can be revised explicitly.

| Topic | Baseline and reason |
| --- | --- |
| Receipt evidence | Keep original uploaded files indefinitely; previews/export images are derivatives. This removes irreversible image processing from the critical path. It costs more storage. |
| Compression | JPEG/PNG derivatives with visual checks; no mandatory AVIF/WebP encoder or automated cleanup. The source benchmark used one receipt fixture plus an upscaled version and reported AVIF at 4.4 s per receipt/112 s per A4 page; WebP was 3.6/105 s. This was not independently reproduced and does not establish performance of other encoders. |
| Export | Core PDF summary plus complete evidence ZIP. PDF/A-3b, embedded originals and appended visible receipt pages are extensions. The owner accepted this reduced export requirement. Retain Typst. |
| Retention | No deletion workflow for confirmed evidence/submitted versions in the experiment. Legal production retention/deletion policy needs separate review; indefinite test retention is not a production GDPR policy. |
| Accounting | Preserve/report VAT data and evidence flags; defer automatic input-tax eligibility and hospitality deductibility claims. Reimbursement and employer tax accounting are distinct. |
| Calendar-day cap | One meal allowance per user/day across employers, with no configuration switch allowing duplicates. The old brief listed owner confirmation as pending. Use this conservative baseline; validate complicated cross-employer allocation before production. |
| Difficult itineraries | Confirmed manual day schedules for special flight/ship/date-line cases are acceptable. Block unresolved schedules. Ordinary foreign travel remains Core. |
| Three-month limit | A visible screening warning and explicit exclusion/assessment, not a complete automated eligibility engine. |
| Domain names | English code/API terminology mapped to German specialist terms in GLOSSARY.md. This replaces the previous mandatory German identifier convention. |
| Delivery | Working application, container builds, tests and restore evidence before elaborate release automation. No migration from the previous application's data is required. |

## Questions to settle before production

1. The owner accepted PDF summary plus evidence ZIP for Core; consider PDF/A and visible receipt pages later if operational needs warrant it.
2. Validate rate datasets and the detailed tax treatment of reimbursed own meals/hospitality, lodging eligibility, foreign VAT and unusual itineraries. Do not infer that full gross reimbursement is always tax-free.
3. Define legally appropriate retention, deletion, storage location and operating procedures. The old spec and operations guide use different starting-year definitions (receipt date versus confirmation year), so neither was adopted as authoritative.
4. Confirm original-file storage costs/privacy and any eventual compression targets using representative real receipts on target hardware. Consider native codecs only after measuring their packaging/performance trade-offs.
5. Choose a project license before distributing code. No license choice is inferred from the source project's visibility.

These questions do not prevent a synthetic-data Core experiment. They limit production claims. Changes to Core scope or expected financial behavior must be recorded here and applied equally to compared agents.

## Source documents reviewed

Source root: `/Users/marbaced/projects/vc-reisekostenabrechnung`. This path documents provenance; an agent implementing this brief does not need access to the source checkout.

| Source group | Role in this brief |
| --- | --- |
| `docs/SPEC.md`, `GLOSSARY.md`, `docs/adr/0001` through `0005` | Domain, original scope, business rules and explicit architecture decisions. ADR 0005 remains a proposed image strategy in the source. |
| `README.md`, `docs/features.md`, `docs/README.md`, `docs/roadmap.md`, `docs/MILESTONES.md` | Intended user journey, later scope and historical status. Completed milestones/releases were discarded as implementation evidence. |
| `docs/authentication.md`, `docs/configuration.md`, `docs/installation.md`, `docs/development.md` | Authentication and deployment constraints; old endpoint paths, environment-variable inventories and patch pins were not copied as requirements. |
| `docs/operations.md`, `docs/betrieb.md`, `docs/verfahrensdokumentation.md` | Evidence preservation, backup/restore and operational policy. Conflicting retention/processing claims were replaced by explicit experiment defaults. |
| `docs/tax-rules.md`, `docs/research/steuer-reisekosten.md` | Rule families and source leads; unresolved legal details remain verification work. |
| `docs/research/stack.md`, `docs/research/beleg-kompression.md` | Prior technical trade-offs and reported experiments; historical version/benchmark claims are not newly verified facts. |
| `docs/ideas/email-receipt-intake.md`, `docs/ideas/code-health.md` | Follow-up intake constraints and failure modes. Code-specific cleanup tasks were excluded. |

No source implementation, generated API, golden-test output, release status or database schema is normative for Reise2. Historical changelog entries, assets and executable research spikes are not requirements and were not audited as a new implementation.

## Tax/source verification status

[§ 9 EStG](https://www.gesetze-im-internet.de/estg/__9.html) was successfully read during this derivation; its meal-allowance thresholds, supplied-meal deductions and three-month/interruption rules support the corresponding core rules. Other legal details and complete annual datasets were not independently revalidated. The existing 2024–2026 foreign-rate CSVs are now included with provenance and structural checks in data/README.md. They are the experiment baseline and do not need to be reconstructed or legally re-audited by each agent. Initial fetches of the official 2026 foreign-rate letter and the small-invoice provision failed. Treat their inherited details as verification tasks, not newly certified legal facts.

A follow-up official-source search located the 2026 BMF foreign-rate guidance and R 9.7 lodging guidance. The latter distinguishes actual/allowance reimbursement and excludes employer-provided lodging from tax-free lodging allowances. Full-page retrieval remained unavailable, so no complete dataset revalidation is claimed.

Starting points for production verification:

- [§ 3 EStG](https://www.gesetze-im-internet.de/estg/__3.html): employer reimbursement exemption.
- [§ 9 EStG](https://www.gesetze-im-internet.de/estg/__9.html): qualifying travel and meal rules.
- [Official 2026 foreign-rate letter](https://www.bundesfinanzministerium.de/Content/DE/Downloads/BMF_Schreiben/Steuerarten/Lohnsteuer/2025-12-05-steuerliche-behandlung-reisekosten-2026.pdf?__blob=publicationFile): foreign-rate values and special cases. Retrieve the corresponding official 2024/2025 letters for those datasets.
- [BMF wage-tax guidance](https://lsth.bundesfinanzministerium.de/): detailed travel, lodging and provided-meal guidance, especially the 2020 travel-expense letter and R 9.6/R 9.7.
- [§ 33 UStDV](https://www.gesetze-im-internet.de/ustdv_1980/__33.html), [§ 15 UStG](https://www.gesetze-im-internet.de/ustg_1980/__15.html): invoices and input-tax requirements.
- [§ 147 AO](https://www.gesetze-im-internet.de/ao_1977/__147.html): retention; also review the applicable GoBD and operator process.
- [ECB Data API](https://data.ecb.europa.eu/help/api/data): exchange-rate interface and publication conventions.

This brief does not certify GoBD compliance or replace the employer's tax review. Before production use, record sources/effective dates for every enabled rate and supported rule. The experiment may use the inherited datasets and clearly labeled synthetic fixtures without a fresh legal audit. Missing/invalid required values still block submission; a completed experiment does not establish production tax compliance.

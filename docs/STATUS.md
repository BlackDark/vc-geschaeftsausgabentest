# Implementation status

State of 2026-10-19 on branch `claude/sharp-mayer-6yryxy`. "Pass" means an automated test
exercises the scenario; "partial" names what is missing; "unverified" means implemented but not
exercised by a test. Scope reductions decided by the owner are in
[DECISIONS.md](DECISIONS.md#owner-decisions-2026-10-19).

## Acceptance coverage

| ID | State | Evidence / gap |
| --- | --- | --- |
| F01 first flow | **pass** | `internal/app` `TestF01FirstWorkingFlow` (service level, real SQLite and Typst) and `web/e2e/f01.spec.ts` (browser): 182.80 → unlock → 192.80, v1 PDF bytes unchanged. |
| F02 foreign lodging | pass (calculation) | `TestSharedFixtures/F02` = 510.00; company-paid allowance rejected. Not driven through the UI. |
| F03 outlay-only | pass (calculation) | `TestSharedFixtures/F03` = 80.00; outlay-only claims work in the UI (no dummy trip) but have no dedicated E2E test. |
| A01–A14 | pass | `internal/calc/calc_test.go`; A12 includes JPY (exponent 0) and actual debit; A14 missing year blocks. |
| A15 three-month limit | partial | Per-day exclusion with reason works (C11 exclusion). The screening warning needs work-location identity, which is not modelled yet. |
| A16 foreign itineraries | partial | Multi-stop rate location and return-from-abroad rule tested. Special flight/ship/date-line schedules (C09) are not implemented; they currently use the ordinary location logic instead of blocking. |
| A17 password login | pass | `internal/api` `TestA17…`: single-use setup token, rate limit, deactivation ends sessions, last admin protected, logout. |
| A18 OIDC | **pass** | `internal/api` `TestA18OIDC` against a mock provider (discovery, JWKS, RS256, PKCE check): refusal outside allowed groups, provisioning, admin group re-evaluated per login, explicit linking, forged nonce and foreign state rejected with a redirect to the login page. |
| A19 trusted proxy | deferred | Owner decision 2026-10-19. |
| A20 ownership | pass | `TestA20Ownership`: lists, details, writes, nested references, receipt files, exports and audit for another user and an admin. |
| A21 receipts | partial | Content-type detection, Typst-based validation and previews, SHA-256 duplicates per user, permanent numbers, camera capture at 360 px (`mobile.spec.ts`). `TestA21UploadLimits`: multipage PDF accepted with page count; size, page and pixel limits, encrypted and broken PDFs, truncated images and unsupported formats rejected without leaving files. Duplicate warning UI has no automated test. |
| A22 claim selection and export | **pass** | Suggestions by end date / expense date, advances by date, warnings acknowledgement, ZIP manifest and evidence checked, server-side locks. `TestA22ExportMatchesPreview`: snapshot totals (the PDF source) equal the preview, and the CSV extracts add up to meals, mileage, trip expenses, outlays, travel and payout. |
| A23 failure and concurrency | partial | Submission runs in one write transaction (number, files, version, frozen days); failed exports leave a draft and remove staged files; startup removes unpublished exports. No fault-injection or concurrency test yet. |
| A24 unlock/resubmit/paid | pass | F01 tests: same number, version 2, v1 bytes unchanged, paid/undo, unlock blocked while paid. |
| A25 daily cap across claims | **pass** | `TestFrozenAllocation` (calculation) and `TestA25DailyCapAcrossClaims` (two same-day trips in two claims pay the day once, also after unlock and resubmission). |
| A26 de/en, mobile, a11y | partial | German E2E on desktop and 360 px without horizontal scroll; English catalog complete but not E2E-tested; theme switch and form preservation implemented, keyboard navigation not systematically tested. |
| A27 operations | partial | Fresh migration, backup → restore → `spesen verify` (incl. tamper detection) tested. Multi-arch image build and read-only container start run in CI only (no Docker daemon in the authoring sandbox). No upgrade fixture yet (single migration). |
| A28 rate tables | partial | Inherited CSVs seeded with schema/year/key/number validation (`internal/rates` tests); CLI import of further years. Admin draft overrides (P17) not implemented. |
| A29 foreign lodging | pass (calculation) | See F02. |
| A30 tools/software | pass | F01 combined claim; F03 calculation; a second claim cannot reuse an outlay. |

## Not yet implemented (Core items)

- C09 confirmed day-by-day schedule for special itineraries; C11 attendance screening warning.
- Warnings for potentially unsuitable invoice recipient (C17), unusually expensive meals, and the
  provided-meal suggestion from hospitality (C20/P08).
- Admin rate overrides with revision and reason (P17); employer management beyond one default
  employer (reduced by decision).
- PDF previews of further pages in the UI (the PDF export contains all pages).
- Metadata stripping for receipts embedded in the claim PDF (previews are re-rendered and carry no
  EXIF; the PDF embeds the original images).

## Unverified rules and data

- Rate values in `data/*.csv` are inherited and not re-audited (see DECISIONS.md).
- Domestic 14/28/20 EUR and mileage 0.30/0.20 EUR/km for 2024–2026 are seeded from the brief;
  the employer-reimbursement basis of the mileage rates still needs verification.
- Tax treatment of reimbursed meals/hospitality and foreign VAT is reported, not assessed.

## Commands run for this state

```text
go vet ./... ; go test ./...                       # calc, rates, app (F01, backup/restore), api (A17, A20)
pnpm --dir web check                               # Biome, tsc (TypeScript 7), Vitest
pnpm --dir web build ; go build ./cmd/spesen
SPESEN_BIN=… pnpm --dir web exec playwright test   # F01 desktop, 360 px mobile
spesen backup DIR ; spesen verify                  # CLI smoke test
```

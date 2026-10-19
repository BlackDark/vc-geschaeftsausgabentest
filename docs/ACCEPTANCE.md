# Acceptance scenarios

Core completion requires all scenarios below, plus the product requirements they reference. Fixtures are synthetic, contain no real personal data and use fixed rates/timestamps. Financial fixtures exercise the rules, not the previous implementation's output. Test rate values below are not production rate data.

## Mandatory first working flow

Use F01 in [fixtures/core-cases.json](../fixtures/core-cases.json). Complete this before expanding the implementation:

1. Sign in and configure the employer in one interface.
2. Record the domestic trip, personally paid hotel and included breakfasts.
3. Upload/confirm synthetic evidence, then record separate tools and software outlays with no trip.
4. Create a quarterly claim including both groups and the advance. Show a payout of EUR 182.80 and export PDF+ZIP.
5. Unlock with a reason, change the software outlay and resubmit. Version 2 pays EUR 192.80; version 1 remains unchanged.

Then use F02 for company-paid/actual-cost/allowance foreign lodging and F03 for an outlay-only claim. These are the complete shared fixture package: three small JSON scenarios, not a large generated dataset. Existing financial examples below remain targeted checks for completing Core. OIDC/proxy login remain Core even though the first slice can use password login.

## Financial examples

Unless stated otherwise: EUR, Europe/Berlin, no advances or additional costs, and domestic partial/full rates of 14/28. Show the explanation as well as the final total.

| ID | Input | Expected result |
| --- | --- | --- |
| A01 | One-day trip 07:00–17:30; lunch provided | Meal allowance 14 − 11.20 = **2.80**. |
| A02 | One-day trip 08:00–16:00; then a separate variant ending 16:01 | Exactly eight hours: **0**. Eight hours and one minute: **14**. |
| A03 | Monday 08:00–Wednesday 18:00; two hotel nights totaling 178; breakfast Tuesday/Wednesday | Meals 14 + 22.40 + 8.40 = **44.80**; lodging **178**; total **222.80**. Hotel breakfast is not subtracted twice. |
| A04 | Tuesday 19:30–Wednesday 06:00; no overnight lodging | One **14** allowance on Wednesday, zero Tuesday. |
| A05 | Two same-day trips, 06:30–10:00 and 13:00–18:00 | Combined 8.5 hours: **14 once**, attributed to the later-ending editable trip. Results are independent of entry order. |
| A06 | Foreign fixture F: full 70, partial 47; one-day trip >8h with provided lunch | **47 − 28 = 19**. A meal in another country still uses this day's full rate. |
| A07 | Two-day domestic trip; lunch and dinner provided on arrival, breakfast on departure | Arrival **0**, departure **8.40**. No negative daily result. |
| A08 | One-day >8h; lunch provided; user contribution 5 | Deduction **6.20**, remaining meal allowance **7.80**. Contribution 15 instead makes the lunch deduction zero, not negative. |
| A09 | Lodging fixture: domestic allowance 20; first night eligible for allowance, second provided | Lodging total **20**. Paying actual cost plus allowance for the same night is rejected; retaining a non-payable invoice as context is allowed. |
| A10 | Car: total 284 km × 0.30; other motor vehicle: 38 km × 0.20 | Mileage **85.20 + 7.60 = 92.80**. Round-trip helper does not double an already-total distance. |
| A11 | Expense gross 133.17; portions 117.70 at 7% and 15.47 at 19% | Net **110 + 13 = 123**, VAT **7.70 + 2.47 = 10.17**, gross **133.17**. A receipt can also be split across two expenses without adding its total again. |
| A12 | USD 48.50; fixture rate 1 EUR = 1.1650 USD, preceding Friday for a Saturday receipt | EUR **41.63**, Friday publication date recorded. Actual-debit override **42.37** remains exact. Test a currency with a non-two-decimal exponent separately. |
| A13 | Reimbursement total 412.40; advance 300, variant advance 500 | Payout **112.40**, variant **−87.60** labeled repayment. An advance cannot be reused. |
| A14 | December 31 full day, January 1 departure; synthetic rate tables Y full 47, Y+1 partial 39 | Total **86**, with each date's own year/rate revision. Missing Y+1 blocks submission; later overrides do not change a submitted total. |
| A15 | Repeated attendance spanning over three months without four-week interruption | Screening warning, user assessment/exclusion recorded; excluded days earn **0** meal allowance. This is not a claim of complete statutory detection. |
| A16 | Foreign trip with multiple stops and return to Germany; separate unresolved long-flight case | Ordinary itinerary uses the proper foreign rate location; unresolved special-day schedule blocks submission until confirmed with reasons. |

Also test conservation of VAT/FX totals including rounding residuals, strict ownership of linked entities, empty/zero claims, DST transitions and a confirmed special itinerary. Business hospitality must validate required participants/occasion and reimburse its recorded gross once; missing evidence and unconfirmed evidence must behave differently as specified.

## Complete workflows and failure cases

| ID | Scenario | Completion criterion |
| --- | --- | --- |
| A17 | First admin, password login, logout and account deactivation | Setup is protected and single-use. Wrong credentials are limited. Deactivated users lose existing-session access. |
| A18 | OIDC login against a test provider | Validate state/nonce/PKCE, token audience/issuer and groups; disallowed provisioning fails. Subject-based identity survives an email change. Group-derived admin changes on a new login. Email similarity does not auto-link accounts. |
| A19 | Trusted-proxy login and combined login modes | Trusted peer plus configured headers authenticates correctly; forged headers from other peers fail. Client-injected forwarded headers cannot establish trust. Logout/session behavior is documented and tested. |
| A20 | User A attempts User B's list/detail/write/link/download/export/audit routes; repeat as admin A | No travel data leakage or cross-owner mutation. Validate nested IDs, not just the top-level ID. |
| A21 | Mobile receipt capture → inbox → confirmation → expense; multipage PDF upload on desktop | Preserved original checksum and permanent receipt number. Exact duplicate warning is within the owner boundary. Preview/confirmation is usable at 360 px. Invalid/oversized input fails recoverably. |
| A22 | Select trips by employer/end date and standalone outlays by employer/expense date, include advance, acknowledge warnings, submit | Preview and PDF/CSV/JSON totals agree. Downloadable ZIP contains all referenced original evidence and a valid manifest. Submitted trip and standalone-outlay edits fail server-side. |
| A23 | Export failure, repeated submission, restart during generation and concurrent submission | No duplicate assignment/version, partial published export, lost draft input or permanently orphaned lock. Retry eventually produces one valid submitted version. |
| A24 | Unlock with reason, edit, resubmit, mark paid, undo payment | Same claim number, newer version, older snapshot/export bytes retained. Payment metadata does not alter existing export bytes. |
| A25 | Same-day trip added after another claim is submitted; two submissions racing for daily entitlement | Existing snapshot remains unchanged; only remaining entitlement is available. Overallocation blocks submission and points to the claim requiring correction. |
| A26 | German/English desktop/mobile use | Both languages complete the main flow, including errors and export headings; currency/date formatting is unambiguous. Keyboard navigation, theme and form preservation work. |
| A27 | Empty database, upgrade fixture, persistent restart, backup and clean restore | Migrations preserve financial records. Restored submitted claim and all evidence verify by checksum. Both target images build; nonroot container starts with read-only root filesystem. |
| A28 | Rate tables for 2024–2026, admin override and next-year import | Use the inherited CSV datasets in data/README.md with provenance and schema/key/year/number validation; malformed/duplicate imports fail clearly. Drafts use a valid new revision; submitted snapshots retain old values. Synthetic fixture rates remain separate. Full legal-source re-auditing is not an experiment gate. |
| A29 | Foreign lodging: company-paid, privately paid actual cost, privately paid eligible allowance (F02) | Actual costs remain visible; company-paid cost adds zero to payout. The privately paid allowance night excludes its actual cost from payout. Company-paid breakfast affects meals. Payout is EUR 510.00. |
| A30 | Tools/software in a combined quarterly claim (F01), and outlays without travel (F03) | Separate travel/outlay subtotals, common payout and evidence export. F03 pays EUR 80.00 without a dummy trip. A second claim cannot reuse an outlay; correction retains the prior version. |

## Test approach and comparison

Use focused Go tests for rules and invariants, real SQLite integration tests for transactions/ownership/migrations, API tests for authentication/security, and Playwright for the complete claim flow. Use a mock OIDC provider and a controlled proxy. Avoid live ECB/OIDC services in deterministic acceptance runs. Inspect a representative PDF visually; file existence alone does not establish a readable export.

For each agent run record pass/fail/unverified per requirement/scenario, elapsed time, budget/tool access, commands actually executed and deviations from the brief. Evaluate financial correctness, evidence/version preservation, authentication/ownership, mobile usability and reproducible operation separately. An unrun scenario is unverified. Extra features do not compensate for a failed Core scenario. No fixed code layout, line count or coverage percentage is a comparison criterion.

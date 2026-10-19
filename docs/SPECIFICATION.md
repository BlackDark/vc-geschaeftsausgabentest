# Product specification

## Goal and scope

Build a self-hosted application for an employee or managing director to record business travel and other personally funded business purchases and request reimbursement from an employer. The initial user is an employed managing director of their own company; German travel-expense rules apply to the travel part. Foreign destinations are supported; other tax jurisdictions and self-employment calculations are outside scope.

The Core must support the complete journey: sign in, configure an employer, record travel, company-paid travel costs and personal outlays with or without a trip, attach receipts, inspect calculations, submit a claim, download its export and record payment. Priorities are correct calculations, preserved evidence, fast mobile entry and simple operation.

**Core** means required for this experiment. **Extension** means deliberately deferred. The architecture document fixes the technology choices; it does not prescribe the previous application's structure.

## Core requirements

| ID | Requirement |
| --- | --- |
| P01 | Support multiple users with strict ownership checks on every object, linked object, file download and export. Admins manage users and rate tables but cannot read other users' travel data. |
| P02 | Provide protected first-admin setup, password login, OIDC login and trusted-proxy header login, logout, persistent secure sessions, admin-created accounts and account deactivation. No public registration. Deactivation invalidates access, including existing sessions. Authentication modes can be combined; startup requires at least one enabled mode. See the architecture's authentication contract. |
| P03 | Let each user maintain multiple employers, with name/address and one active default, and an optional employee number in their profile. Historical claims retain their employer details. |
| P04 | Record trip purpose, employer, start/end with IANA zones, stops and optional project/notes. End must follow start. Explain that times represent absence, not working hours. The user determines whether this is qualifying business travel. |
| P05 | Derive travel days and show day type, applicable country/city, allowance, meal deductions, lodging and an understandable calculation breakdown. Users can correct a rate location with a recorded reason. |
| P06 | Capture travel expenses in all five travel cost types, with date, supplier, description, payer (user/employer), original currency/gross amount, VAT portions and reimbursable amount in EUR. Each travel expense and mileage journey belongs to one trip. Company-paid costs remain visible with zero payout. |
| P07 | Record private-vehicle mileage, vehicle class and explicit total driven distance. Support actual transport expenses separately and warn about possible duplicate fuel/mileage reimbursement. |
| P08 | Record provided breakfast/lunch/dinner, meal contributions and lodging per night. Suggest provided meals from reimbursed hotel breakfast or hospitality; show and allow correction of the suggestion. Keep its origin and correction in the audit trail. |
| P09 | Support EUR and foreign-currency expenses using a documented ECB rate or user-entered rate/actual EUR debit. Preserve the selected conversion when a claim is submitted. |
| P10 | Support receipts from camera/file input on mobile and upload on desktop. Core formats: JPEG, PNG and PDF, including multipage PDFs. Preserve original bytes. Show previews, require visual confirmation, and detect exact duplicates within the user's records by SHA-256. |
| P11 | Provide a receipt inbox and many-to-many receipt/expense assignment. A shared receipt must not silently duplicate an expense amount. Accept an Eigenbeleg with date, amount, payee, purpose, issuer and missing-receipt reason. |
| P12 | Record business hospitality with occasion, place, date, participants including at least one external participant, and the user's confirmation. Reimburse the recorded gross cost; accounting tax-deductibility calculations are deferred. |
| P13 | Create a claim for one employer and an inclusive custom date range, with month/quarter shortcuts. Suggest unclaimed trips by local end date and unclaimed standalone outlays by expense date in the period; allow deselection. Support claims containing either group or both, without inventing a dummy trip. Include only that employer's unassigned advances dated no later than period end. |
| P14 | Before submission show blockers, warnings, totals, advance deductions and a draft preview. Require acknowledgement of the current warnings. Freeze inputs, rates, results, evidence links and user/employer details on successful submission. |
| P15 | Enforce Draft → Submitted → Paid. Submitted trips and all their calculation inputs and evidence assignments are locked. Only the owner can unlock a Submitted claim with a reason; older snapshots and exports remain accessible. Undo Paid with a reason before unlocking. |
| P16 | Produce a readable PDF summary and a ZIP containing that PDF, a versioned JSON snapshot, expenses/travel-days/mileage CSVs (including standalone outlays and payer/reimbursable fields), original evidence and a SHA-256 manifest. All formats must agree with the snapshot. The Core PDF may refer to evidence in the ZIP; embedding every receipt page and PDF/A are extensions. |
| P17 | Support annual rate data for 2024–2026, with source references, admin overrides and a validated way to import another year. Use the included inherited CSVs as the initial foreign-rate datasets; missing or invalid required rates block submission. A fresh full source audit is not a Core implementation gate. Overrides affect draft calculations only. |
| P18 | Keep an append-only application audit trail for financial/evidence changes, rate changes and claim transitions, including actor, time and reason where required. Record the event in the transaction committing the change. |
| P19 | Provide German and English UI and claim summaries, responsive forms usable at 360 px, keyboard-accessible controls, explicit save/error feedback and a choice of light/dark/system theme. Preserve entered form values on recoverable errors. |
| P21 | Record standalone outlays for one employer with no trip: date, supplier, purpose, tools/equipment, software/services or other category, personally paid amount, currency/FX, VAT portions and evidence. Claim/export them alongside travel or independently. Depreciation, asset accounting and recurring subscription management are outside scope. |
| P22 | Per lodging night, choose actual personally paid cost, eligible allowance or no reimbursement. Record company-paid/provided lodging separately from privately paid lodging. Employer-paid hotel costs yield zero personal lodging payout; retain provided-meal information. Actual cost and allowance for the same night are mutually exclusive. |
| P20 | Run as one application container on linux/amd64 and linux/arm64, with SQLite and local persistent files. Supply configuration validation, health checks, backup/restore instructions and a verified restore procedure. |

## Integrity and submission

- A trip belongs to one user, one of their employers and at most one claim, including draft claims. A standalone outlay has the same ownership/employer constraints and can be assigned to at most one claim. Trip-linked expenses enter through their trip only; never select them again as standalone outlays. Lock standalone outlays and their evidence assignments on submission, and unlock them with the same correction flow as trips. An advance is assigned to at most one claim for its employer. Verify ownership of every supplied foreign key.
- A confirmed receipt's evidence bytes are immutable. Corrections create new evidence or metadata versions. Confirmed evidence can be voided when unused by a locked trip; it stays retained with its number and reason.
- Receipt numbers are unique per user/year; claim numbers per user/employer/year. Allocate sequentially on confirmation/first successful submission. Retain voided numbers and reuse a claim's number when resubmitting. An unexplained numbering gap is an error; a failed attempt must not consume a number.
- Claim versions increase on resubmission. Never overwrite an older snapshot/export. Mark an old version as superseded only when a replacement submission succeeds.
- Submission temporarily freezes affected data while generating files. Concurrent edits fail with an actionable conflict. Publish a Submitted version only when every required artifact exists and verifies; failure leaves an editable Draft and no published partial export. Retries and restarts must not double-assign trips/advances or publish duplicate versions.
- Payment is mutable claim metadata recorded in the audit trail, with payment date and optional note. It does not rewrite an earlier PDF. Claims that have ever been submitted cannot be deleted.
- A later trip or meal edit can affect a day already claimed elsewhere. Preserve old snapshots and block further submission if the daily entitlement is overallocated; require correction through the affected claims.
- For the experiment, keep confirmed evidence, submitted versions and audit records indefinitely. There is no automatic retention deletion. Production retention policy is an explicit follow-up decision.

## Entry and payout model

Use one application interface with clear navigation for Trips, Standalone outlays, Receipts and Claims; a separate admin application or role-specific dashboard is unnecessary. Show available functions in the same shell while enforcing privileged operations and data ownership on the server.

Record actual payments and reimbursement separately. A company-paid hotel or transport invoice documents the trip but adds zero to the employee payout. A personally paid hotel can use actual-cost reimbursement or an eligible allowance; choosing the allowance leaves the actual invoice as context, not another payable line. Keep evidence and meal flags independent of the payer. Partial/mixed payment of one charge is outside Core; the user can split it into separately documented portions.

Show one combined claim with distinct sections: Travel reimbursement, Standalone outlays, Advances and Payout. Quarterly and custom periods are required. “Submit” finalizes the reimbursement document; “Paid” records a transfer made outside the app. Initiating a bank payment, tax return filing and accounting for tools/software are outside scope.

## Validation

Block submission for invalid trip chronology, invalid ownership, inconsistent monetary totals, unavailable rates, absent FX conversion, unconfirmed linked receipts, incomplete hospitality data, invalid lodging double claims or a conflicting locked daily allowance. Missing receipts are warnings when no evidence has been linked; the user can supply an Eigenbeleg or acknowledge the gap. A linked receipt awaiting confirmation is a blocker.

Warn about duplicate evidence, potentially ineligible invoice recipient, repeated attendance at a work location, provided meals without an allowance, unusually expensive meals, fuel plus private mileage and expenses outside the travel period. Acknowledgements are tied to the current input revision and stored in the submitted snapshot. Warnings do not certify tax eligibility.

## Ordered follow-up stages

After the Core passes, implement these stages in the owner-confirmed order. They are not part of Core acceptance. Each stage must preserve ownership, immutable evidence and snapshot behavior.

1. **S3 storage:** S3-compatible local/object-store choice, the same storage contract tests for both, conditional creation, checksum verification and backup guidance for database plus bucket. Default operator guidance is EU/EEA hosting; any other location needs a separately reviewed policy.
2. **Mail import:** forward receipts to a mailbox polled over TLS. Use revocable per-user secret recipient addresses, explicit sender rules and durable message/attachment deduplication across restarts. Preserve original `.eml` and attachments with provenance. Reuse normal upload validation and file into the owner's inbox. Only trust authentication verdicts from the configured receiving infrastructure; arbitrary sender-supplied headers are not verification. Failed/unmapped messages remain recoverable. No automatic expense creation or deletion of source mail on failure.
3. **AI receipt reading:** an operator-configured OpenAI-compatible endpoint, disabled by default and requiring user opt-in with destination disclosure. Extract suggested supplier/date/currency/amount/VAT fields. Validate responses, require human confirmation, retain confirmed text/fields as versions and keep manual entry available on failure. Bound processing time and concurrency; redact credentials and receipt contents from logs.

## Other extensions

Litestream replication; XML e-invoices and ZUGFeRD parsing; PDF/A-3b with embedded originals and visible receipt pages; receipt crop/perspective correction and advanced compression; templates and project autocomplete; PWA installation and offline capture; general-purpose personal upload tokens; routing-based distance calculation; employer accounting deductions and DATEV; sophisticated three-month-limit detection; release automation and supply-chain attestation.

Other tax regimes, payroll calculations, approval workflows, private-trip cost allocation, company-car logs and horizontal scaling are outside this baseline.

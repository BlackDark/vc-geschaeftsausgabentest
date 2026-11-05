# Spesen

Self-hosted tool for recording German business-travel costs and other personally paid business purchases, and claiming their reimbursement from an employer. English terms are canonical in code and API names; German UI text uses the `_German_` term of each entry. Rules and calculations live in `docs/CALCULATION_RULES.md` and `docs/SPECIFICATION.md`, not here.

## Language

### People

**User**:
A person recording and claiming their own business expenses.
_German_: Nutzer
_Avoid_: Employee

**Admin**:
A user who also manages accounts and shared rate tables, without access to anyone else's records.
_German_: Admin

**Employer**:
The organization that reimburses a user. Employers belong to individual users.
_German_: Arbeitgeber
_Avoid_: Company, client

### Trips

**Trip**:
A business-related absence from home and the first place of work, from departure to return. Ordinary commuting is not a trip.
_German_: Reise
_Avoid_: Journey, travel

**Travel day**:
A local calendar date touched by a trip, with its day type, rate location, provided meals and allowance result.
_German_: Reisetag

**Stop**:
A recorded arrival during a trip, with local time, country, rate location and whether business activity took place.
_German_: Ortswechsel
_Avoid_: Leg, waypoint

**Work location**:
A place where business activity occurs; repeated attendance can affect meal allowance eligibility.
_German_: Tätigkeitsstätte

**First place of work**:
The employment-related location that separates commuting from business travel, as declared by the user.
_German_: Erste Tätigkeitsstätte
_Avoid_: Office, home base

**Mileage journey**:
Distance driven in a private motor vehicle. Not the same as the overall trip.
_German_: Fahrt
_Avoid_: Trip, drive

**Provided meal**:
A breakfast, lunch or dinner supplied by or for the employer, including a qualifying reimbursed meal.
_German_: Gestellte Mahlzeit
_Avoid_: Free meal

**Meal contribution**:
The user's own payment towards a provided meal, which reduces that meal's deduction.
_German_: Zuzahlung

**Lodging**:
The arrangement for the night after a travel day: actual cost, allowance, provided, in transit or none.
_German_: Unterkunft
_Avoid_: Hotel, accommodation

### Allowances and rates

**Allowance**:
A prescribed amount claimable without proving the exact expenditure. An allowance is not an expense.
_German_: Pauschale
_Avoid_: Flat rate

**Meal allowance**:
The daily allowance for additional meal costs of qualifying travel.
_German_: Verpflegungspauschale
_Avoid_: Per diem

**Meal deduction**:
The reduction of a meal allowance for a provided meal.
_German_: Mahlzeitenkürzung

**Lodging allowance**:
An eligible fixed overnight amount used instead of reimbursing actual lodging cost.
_German_: Übernachtungspauschale

**Mileage allowance**:
Reimbursement from eligible distance and the vehicle rate.
_German_: Kilometerpauschale

**Rate location**:
A country or named city/region for which a rate table defines allowances.
_German_: Satzort
_Avoid_: Destination

**Applicable country**:
The country whose meal allowance applies to a travel day; not necessarily where a meal was eaten.
_German_: Maßgebliches Land

**Rate table**:
A dated set of allowance values and thresholds, with sources and revision history.
_German_: Satztabelle
_Avoid_: Tariff

**Three-month limit**:
The restriction on meal allowances for longer-term work at the same location.
_German_: Dreimonatsfrist

### Expenses

**Expense**:
A recorded purchase or charge with its payer and employer. It belongs to a trip or is a standalone outlay.
_German_: Ausgabe
_Avoid_: Cost, spending

**Standalone outlay**:
A personally paid business expense without a trip, such as tools or software.
_German_: Sonstige Auslage
_Avoid_: Other expense, misc

**Payer**:
Who paid a charge: the user or the employer. A company card counts as the employer, whoever used it.
_German_: Bezahlt von

**Reimbursable amount**:
The amount payable to the user for an item. Distinct from the gross amount; zero for employer-paid expenses.
_German_: Erstattungsfähiger Betrag

**Cost type**:
The classification of an expense, such as transport, meals or lodging for trips, or tools, software or other for standalone outlays.
_German_: Kostenart
_Avoid_: Category

**VAT portion**:
Part of an expense with one VAT rate and tax country.
_German_: Steueranteil

**Business hospitality**:
An expense for entertaining business partners, documented with occasion, place and participants.
_German_: Bewirtung

### Evidence

**Receipt**:
Documentary evidence for expenses. One receipt can support several expenses and the reverse.
_German_: Beleg
_Avoid_: Invoice, voucher

**Receipt inbox**:
Receipts captured but not yet assigned to expenses.
_German_: Belegeingang

**Receipt number**:
The permanent human-readable reference given to a receipt when it is confirmed.
_German_: Belegnummer

**Original file**:
The exact file the user uploaded. For a paper receipt, the uploaded capture, not the paper.
_German_: Originaldatei

**Archived receipt**:
A confirmed receipt kept immutably, including its original files. Previews and export images are derivatives of it.
_German_: Archivbeleg

**Self-issued receipt**:
A user-written substitute for unavailable third-party evidence, recording what is missing and why.
_German_: Eigenbeleg
_Avoid_: Own receipt, substitute receipt

### Claims

**Claim**:
A document grouping a user's selected trips, standalone outlays and advances for one employer and period. Submitting it freezes a version. Distinct from a trip, an expense and its export.
_German_: Abrechnung
_Avoid_: Report, statement, expense report

**Claim period**:
The inclusive date range of a claim.
_German_: Abrechnungszeitraum

**Claim number**:
The permanent human-readable reference given to a claim on its first successful submission.
_German_: Abrechnungsnummer

**Claim status**:
Draft, Submitted or Paid. Submitted does not mean approved by or delivered to the employer.
_German_: Abrechnungsstatus

**Advance**:
An amount an employer already paid, offset against one claim.
_German_: Vorschuss

**Reimbursement total**:
The reimbursable expenses and allowances of a claim before advances.
_German_: Erstattungsbetrag

**Payout amount**:
The reimbursement total minus advances. Negative means repayment to the employer.
_German_: Auszahlungsbetrag
_Avoid_: Balance

**Snapshot**:
The frozen inputs, rules, rates, results and evidence references of a submitted claim version.
_German_: Festschreibungsstand

**Export**:
Files generated from a snapshot. A preview is not an export.
_German_: Export

**Unlock**:
The owner's recorded return of a Submitted claim to Draft for correction; earlier versions remain.
_German_: Entsperrung
_Avoid_: Reopen

**Warning**:
A condition the user must acknowledge before submission.
_German_: Warnung

**Blocker**:
A condition that prevents submission until resolved.
_German_: Blocker
_Avoid_: Error

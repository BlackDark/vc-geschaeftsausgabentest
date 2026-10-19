# Glossary

Definitions describe the business domain. English is the default for code and API names; use the English terms below consistently. German UI text uses the matching German terms. Technical storage details do not change these definitions.

| English term | German term | Definition |
| --- | --- | --- |
| User | Nutzer | A person recording and claiming their own business travel expenses. |
| Admin | Admin | A user who also manages accounts and shared rate tables. This role grants no access to another user's travel records. |
| Employer | Arbeitgeber | The organization reimbursing a user's business travel. Employers belong to individual users. |
| Business trip | Reise | A business-related absence from home and the first place of work, from departure to return. Ordinary commuting is excluded. |
| Travel day | Reisetag | A local calendar date touched by a trip, with its day type, applicable rate location, provided meals and allowance result. |
| Stop | Ortswechsel | A recorded arrival during a trip, identifying local time, country, rate location and whether business activity took place there. |
| Work location | Tätigkeitsstätte | A place where business activity occurs; repeated attendance may affect meal allowance eligibility. |
| First place of work | Erste Tätigkeitsstätte | The employment-related location used to distinguish ordinary commuting from qualifying business travel. Eligibility is declared by the user. |
| Rate location | Satzort | A country or named city/region for which an annual rate table defines allowances. |
| Applicable country | Maßgebliches Land | The country whose meal allowance applies to a travel day; it need not be the country where a meal was eaten. |
| Lodging | Unterkunft | The arrangement for the night following a travel day: actual cost, allowance, provided, in transit or none. |
| Expense | Ausgabe | A recorded purchase or charge, with its payer and employer. It may belong to a trip or be a standalone outlay. Its gross amount is distinct from its reimbursable amount. |
| Standalone outlay | Sonstige Auslage | A personally paid business expense without a trip, such as tools or software, reimbursed through the same claim. |
| Payer | Bezahlt von | Whether the user or employer paid the charge. A company card belongs to the employer regardless of who used it. |
| Reimbursable amount | Erstattungsfähiger Betrag | The amount payable to the user for an item; employer-paid expenses have zero payable amount. |
| Lodging reimbursement method | Erstattungsart der Übernachtung | Actual personally paid cost, eligible lodging allowance, or no reimbursement. A company-paid/provided night has no lodging payout. |
| Cost type | Kostenart | The classification of an expense: travel uses transport, meals, lodging, incidental travel costs or business hospitality; standalone outlays use tools/equipment, software/services or other business outlay. |
| VAT portion | Steueranteil | A part of an expense carrying one VAT rate and tax country, with net, VAT and gross amounts. |
| Mileage journey | Fahrt | Distance driven in a private motor vehicle and reimbursed using a mileage allowance. It is distinct from the overall business trip. |
| Business hospitality | Bewirtung | An expense for entertaining business partners, documented with occasion, place and participants. |
| Provided meal | Gestellte Mahlzeit | Breakfast, lunch or dinner provided by or on behalf of the employer, including a qualifying reimbursed meal. |
| Meal contribution | Zuzahlung | The user's own payment towards a provided meal, reducing that meal's allowance deduction. |
| Allowance | Pauschale | A prescribed amount that can be claimed without proving that exact expenditure. |
| Meal allowance | Verpflegungspauschale | The daily allowance for additional meal costs arising from qualifying travel. |
| Meal deduction | Mahlzeitenkürzung | Reduction of a meal allowance for a provided meal. |
| Lodging allowance | Übernachtungspauschale | An eligible fixed overnight amount used instead of reimbursing actual lodging costs. |
| Mileage allowance | Kilometerpauschale | Reimbursement based on eligible distance and the applicable vehicle rate. |
| Rate table | Satztabelle | A dated set of allowance values and thresholds, with sources and revision history. |
| Receipt | Beleg | Documentary evidence supporting expenses; one receipt may support several expenses and vice versa. |
| Original file | Originaldatei | The exact file uploaded by the user. For a paper receipt photo, this is the uploaded capture, not the paper itself. |
| Archived receipt | Archivbeleg | The confirmed evidence retained immutably. In this baseline it includes the original uploaded files; previews and export images are derivatives. |
| Self-issued receipt | Eigenbeleg | A user-authored substitute when third-party evidence is unavailable; it records the missing evidence and reason. |
| Receipt inbox | Belegeingang | Receipts captured before they have been assigned to expenses. |
| Receipt number | Belegnummer | A permanent human-readable reference assigned when evidence is confirmed. |
| Expense claim | Abrechnung | A document grouping a user’s selected trips, standalone outlays and advances for one employer and period; submission freezes a version. “Abrechnung” is the common UI name for the combined reimbursement claim. |
| Claim period | Abrechnungszeitraum | An inclusive date range; trips are selected by local end date and standalone outlays by expense date. |
| Claim number | Abrechnungsnummer | A permanent human-readable reference allocated on first successful submission. |
| Advance | Vorschuss | An amount already paid by an employer and offset against one claim for that employer. |
| Reimbursement total | Erstattungsbetrag | Total reimbursable expenses and allowances before subtracting advances. |
| Payout amount | Auszahlungsbetrag | Reimbursement total minus assigned advances. A negative value means repayment to the employer. |
| Claim status | Abrechnungsstatus | Draft, Submitted or Paid. Submitted does not imply approval or delivery to the employer. |
| Unlock | Entsperrung | An owner's recorded transition from Submitted to Draft for correction, retaining earlier submitted versions. |
| Snapshot | Festschreibungsstand | The frozen inputs, rules, rates, results and evidence references of a submitted claim version. |
| Export | Export | Immutable files generated from a submitted snapshot. A preview is not a submitted export. |
| Warning | Warnung | A condition requiring informed acknowledgement before submission. |
| Blocker | Blocker | A condition that prevents submission until resolved. |
| Three-month limit | Dreimonatsfrist | A restriction on meal allowances for longer-term work at the same location, subject to interruption rules. |

“Trip”, “expense” and “claim” must remain distinct. An allowance is not an expense paid by the user; an export is not the claim itself. A checksum proves byte identity, not legal compliance.

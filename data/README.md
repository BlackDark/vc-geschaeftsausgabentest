# Initial rate data

The three foreign-rate CSVs are copied byte-for-byte from the previous project's extracted annual BMF tables. They are inputs for the new application, not an implementation dependency. Filenames are English; the inherited headers and German country/city names remain unchanged.

| File | Data rows | SHA-256 |
| --- | --- | --- |
| `foreign-rates-2024.csv` | 223 | `cde52bca7ece4a93241308cecebe340d2b402e7405fe1c24a1e19603ded5a543` |
| `foreign-rates-2025.csv` | 224 | `b7ba73e6204c3fc0a28421e6418961a6a3734fdf5e5587d3dfe1ff1ca419874a` |
| `foreign-rates-2026.csv` | 214 | `d9cbfe7a798dd01123df1382c343cda0828d9eef7f0191fdc25ce467e96169f4` |

Columns: `jahr`, `land`, `ort`, `vma_24h_eur`, `vma_an_abreise_8h_eur`, `uebernachtung_ag_pauschal_eur`. Amounts are whole EUR, not cents. Empty `ort` denotes the country's general entry. `country-codes.csv` maps BMF names to ISO codes; validate that every referenced country can be mapped and retain named city rates. The application can normalize these headers internally.

Provenance: source checkout `docs/research/data/auslandspauschalen_2024.csv`, `_2025.csv`, `_2026.csv`; country mapping from `satztabellen/laender.csv`. The source research attributes these datasets to the annual foreign-travel letters of 2023-11-21 (2024), 2024-12-02 (2025) and 2025-12-05 (2026).

- [2024 letter, archived IHK mirror](https://www.ihk-muenchen.de/ihk/documents/Recht-Steuern/Steuerrecht/Einkommensteuer/2023-11-21-steuerliche-behandlung-reisekosten-reisekostenverguetungen-2024.pdf)
- [Official 2025 letter](https://www.bundesfinanzministerium.de/Content/DE/Downloads/BMF_Schreiben/Steuerarten/Lohnsteuer/2024-12-02-steuerliche-behandlung-reisekosten-2025.pdf?__blob=publicationFile)
- [Official 2026 guidance](https://lsth.bundesfinanzministerium.de/lsth/2026/B-Anhaenge/Anhang-25/I/inhalt.html)

Validation performed for this brief: expected header, matching year, unique year/country/city keys and nonnegative integer rate values. No duplicate keys or malformed rate numbers were found. A complete value-by-value legal-source comparison was not repeated. Use these inherited datasets for the experiment; each compared agent need not research or rebuild them. Additional domestic/mileage rules and thresholds live in [CALCULATION_RULES.md](../docs/CALCULATION_RULES.md) and must be seeded separately.

An import validates schema, country mapping, year, unique keys and numbers; failures identify the row. Preserve source and revision information. Missing years/locations and invalid data block calculations as specified; a generic “not independently audited” flag does not block the supplied experimental datasets. Production-source review is a release task, not a benchmark completion gate.

Annual CSVs encode amounts only. Country selection, supplied-meal reductions, lodging eligibility and special-itinerary rules still need application logic. Future official years are imported rather than guessed. Synthetic scenario rates under fixtures/ are separate and must not enter production seed data.

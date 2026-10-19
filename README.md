<div align="center">

<h1>Spesen: self-hosted travel expense and reimbursement claims</h1>

**Self-hosted app for German business travel expenses (Reisekostenabrechnung) and out-of-pocket outlays, built for an employed managing director.**<br>
Record trips, receipts and purchases on your phone, let it calculate meal and lodging allowances and mileage, and submit one clean PDF to your employer.<br>
One small container, SQLite, no cloud, login with password or Pocket ID.

[![CI](https://github.com/BlackDark/vc-geschaeftsausgabentest/actions/workflows/ci.yml/badge.svg)](https://github.com/BlackDark/vc-geschaeftsausgabentest/actions/workflows/ci.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue)](LICENSE)

</div>

## 💡 Why Spesen?

Spreadsheets and paper forms get the per-diem rules wrong (8-hour threshold, arrival days, provided breakfast, foreign rates) and lose receipts.

- **Correct allowances**: German meal allowances incl. meal deductions, foreign rates 2024–2026, lodging actual cost vs. allowance, mileage; every line shows its rule ID and rate source.
- **Receipts that stay original**: photo or PDF upload, visual check, permanent receipt numbers, SHA-256 checksums, duplicate detection.
- **One readable PDF**: summary, per-day calculation and all receipt pages in one document, plus a ZIP with originals, CSV and JSON for the employer.
- **Corrections without losing history**: submitted claims are frozen; unlocking with a reason creates a new version, the old PDF stays byte-identical.

Runs on any Linux server (amd64/arm64) behind a reverse proxy.

## 🚀 Install

```bash
mkdir spesen && cd spesen
curl -O https://raw.githubusercontent.com/BlackDark/vc-geschaeftsausgabentest/main/compose.yaml
mkdir secrets && openssl rand -hex 24 > secrets/setup_token
# edit SPESEN_BASE_URL in compose.yaml, then:
docker compose up -d
```

Open the URL, enter the setup token and create your account. Configuration, Pocket ID, backup and restore: [docs/OPERATIONS.md](docs/OPERATIONS.md).

## ✨ Features

- Trips with departure/return times, stops abroad, provided meals, contributions, lodging per night and private-car journeys
- Company-paid costs documented with zero payout; hotel breakfast suggested as provided meal
- Standalone outlays (tools, software) in the same claim as travel
- Receipt inbox with mobile camera capture, multipage PDFs and self-issued receipts (Eigenbeleg)
- Quarterly/monthly/custom claims with advances, blockers and warnings, versioned submission, payment tracking
- German and English UI, dark theme by default (light and system selectable), usable from 360 px
- Password and OIDC login (Pocket ID), per-user data isolation, audit trail

## 📚 Documentation

- [Operations](docs/OPERATIONS.md): configuration, Pocket ID, backup/restore, rate import
- [Status](docs/STATUS.md): acceptance coverage, open gaps and unverified rules
- [Calculation rules](docs/CALCULATION_RULES.md), [Decisions](docs/DECISIONS.md), [Architecture](docs/ARCHITECTURE.md), [Original brief](docs/BRIEF.md)

## 🛠️ Development

Tools come from [`mise.toml`](mise.toml) (Go, Node, pnpm, Typst):

```bash
mise install
pnpm --dir web install
mise run check        # gofmt, go vet, go test, Biome, tsc, Vitest
mise run dev          # API on :8080 (setup token: dev-setup-token)
pnpm --dir web dev    # UI on :5173 with proxy to :8080
mise run gen          # after API changes: OpenAPI + TypeScript client
mise run build        # SPA + single binary in bin/spesen
SPESEN_BIN=$PWD/bin/spesen pnpm --dir web e2e   # Playwright
docker build -f docker/Dockerfile .
```

---

<div align="center">

<sub>License: <a href="LICENSE">AGPL-3.0</a> · © BlackDark and contributors<br>
Changed versions you pass on or run for others must offer their source code under the AGPL.<br>
Not tax advice. Rates are provided as data with their sources; the employer decides on the payroll tax treatment.</sub>

</div>

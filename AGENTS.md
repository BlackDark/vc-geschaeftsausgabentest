# Agent instructions

Read what touches your change before implementing:

1. [GLOSSARY.md](GLOSSARY.md): domain vocabulary; use its English terms in code and docs.
2. [docs/adr/](docs/adr/): hard-to-reverse decisions and why.
3. [docs/SPECIFICATION.md](docs/SPECIFICATION.md): required behavior and scope (P01–P22).
4. [docs/CALCULATION_RULES.md](docs/CALCULATION_RULES.md): arithmetic and tax rules (C01–C22).
5. [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): technology choices and technical constraints.
6. [docs/ACCEPTANCE.md](docs/ACCEPTANCE.md): scenarios A01–A30 and fixtures F01–F03 in `fixtures/core-cases.json`.
7. [docs/DECISIONS.md](docs/DECISIONS.md): owner scope decisions, open verification items.
8. [docs/STATUS.md](docs/STATUS.md): what is implemented, verified and still open.

[CLAUDE.md](CLAUDE.md) has the commands and conventions.

- Keep the technology choices in `docs/ARCHITECTURE.md`.
- Rate data comes from the CSV datasets described in `data/README.md`, validated on import. Synthetic fixture rates stay separate from them.
- Record material assumptions in `docs/DECISIONS.md`. Ask about choices that change scope or financial results; proceed with the documented defaults for ordinary implementation decisions.

## Agent skills

### Issue tracker

Issues live in GitHub Issues (`BlackDark/vc-geschaeftsausgabentest`), via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical labels, identity mapping: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

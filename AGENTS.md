# Agent instructions

Read docs/BRIEF.md and all six documents in its reading order before implementation. They define the Core; docs/DECISIONS.md distinguishes inherited decisions from new defaults and records the owner's decisions of 2026-10-19 (stack, scope). docs/STATUS.md lists what is implemented, verified and still open. CLAUDE.md has the commands and conventions.

1. Keep the technology choices in docs/ARCHITECTURE.md.
2. Complete the mandatory first flow in docs/ACCEPTANCE.md using fixtures/core-cases.json: login, trip/hotel/meal entry, independent outlays, an advance, export and a correction version. Then expand until every Core acceptance scenario passes. Use one interface; preserve per-user data isolation.
3. Keep calculation rules in one server-side calculation module. Use stable rule identifiers in its results. Use GLOSSARY.md for domain terminology.
4. Reuse the inherited CSV datasets described in data/README.md; validate schema, dates, keys and numeric values on import. A fresh legal review is not a prerequisite for this implementation experiment. Keep synthetic test rates separate from these datasets and document remaining production-source checks.
5. Record material assumptions in docs/DECISIONS.md. Ask about choices that change scope or financial results; proceed with the documented defaults for ordinary implementation decisions.
6. Provide actual setup, build, test, container and backup/restore instructions as the implementation takes shape. Finish with acceptance coverage, commands run, remaining gaps and any unverified rules.

The brief described a new build; docs/STATUS.md is the record of what this implementation verifies.

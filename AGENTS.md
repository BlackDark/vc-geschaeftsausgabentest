# Agent instructions

Read README.md and all six documents in its reading order before implementation. They define the Core; docs/DECISIONS.md distinguishes inherited decisions from new defaults.

1. Start from an empty application. Design the schema, API and UI for this brief. Keep the required technology choices in docs/ARCHITECTURE.md.
2. Complete the mandatory first flow in docs/ACCEPTANCE.md using fixtures/core-cases.json: login, trip/hotel/meal entry, independent outlays, an advance, export and a correction version. Then expand until every Core acceptance scenario passes. Use one interface; preserve per-user data isolation.
3. Keep calculation rules in one server-side calculation module. Use stable rule identifiers in its results. Use GLOSSARY.md for domain terminology.
4. Reuse the inherited CSV datasets described in data/README.md; validate schema, dates, keys and numeric values on import. A fresh legal review is not a prerequisite for this implementation experiment. Keep synthetic test rates separate from these datasets and document remaining production-source checks.
5. Record material assumptions in docs/DECISIONS.md. Ask about choices that change scope or financial results; proceed with the documented defaults for ordinary implementation decisions.
6. Provide actual setup, build, test, container and backup/restore instructions as the implementation takes shape. Finish with acceptance coverage, commands run, remaining gaps and any unverified rules.

The brief describes a new build. Historical implementation claims in the source project do not establish completion here.

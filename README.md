# Reise2: implementation brief

This directory is a standalone starting point for a new travel and out-of-pocket reimbursement application. Nothing is implemented. Build from the requirements here; the previous application's code, schema, API, UI and release history are not the specification.

The purpose is to compare agents implementing the same useful product with the same scope. Documentation and filenames are English. German domain terms remain available in the glossary. The application itself supports German and English.

## Reading order

1. [Product specification](docs/SPECIFICATION.md): required behavior and scope.
2. [Glossary](GLOSSARY.md): shared domain language.
3. [Calculation rules](docs/CALCULATION_RULES.md): arithmetic and tax-related behavior.
4. [Architecture](docs/ARCHITECTURE.md): retained stack and technical constraints.
5. [Acceptance scenarios](docs/ACCEPTANCE.md): observable completion criteria.
6. [Decisions and provenance](docs/DECISIONS.md): assumptions, deliberate reductions and unresolved questions.

The agreed scope is the **Core** in the specification, including password, OIDC and trusted-proxy login. The owner confirmed this scope on 2026-10-18. Subsequent stages are **S3 → mail import → AI receipt reading**, in that order. The owner also accepted preserving originals, PDF+ZIP for Core, a small shared fixture set and a mandatory first working flow. Use one interface while keeping each user’s data separate. Core includes standalone outlays and company-paid travel costs; the claim combines travel reimbursement and other outlays. The export prepares a payout; bank integration is outside scope.

The inherited foreign-rate CSVs are included in [data/](data/README.md). Three small synthetic scenarios in [fixtures/core-cases.json](fixtures/core-cases.json) define common comparison inputs. Implement the first working flow in docs/ACCEPTANCE.md before expanding to all Core scenarios.

## Starting an agent run

Give each agent a separate copy of this directory and the same prompt:

> Implement the Core described in README.md and its linked documents from an empty application. Follow AGENTS.md. Preserve the chosen stack, implement in working vertical slices, and verify against docs/ACCEPTANCE.md. Record material assumptions and provide reproducible setup and test commands. Report incomplete requirements and any tax rules or rate data that remain unverified. Extensions are optional only after the Core passes.

Use the same brief revision, tool access and time budget for comparable runs. Record those conditions externally, so one agent's decisions do not become another agent's starting point.

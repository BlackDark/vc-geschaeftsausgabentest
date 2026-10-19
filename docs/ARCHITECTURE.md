# Architecture

## Technology choices

Decided with the owner on 2026-10-19 (see [DECISIONS.md](DECISIONS.md#owner-decisions-2026-10-19)). The previous Go/Svelte/sqlc/OpenAPI-first stack was replaced by the owner's default stack where it fits, keeping Go for a small, static backend.

| Area | Choice |
| --- | --- |
| Backend | Go (version pinned in `go.mod`/`mise.toml`), standard-library `net/http` with **Huma v2** for typed operations, request validation and a generated OpenAPI 3.1 document (code first, no hand-written spec). Structured logging with `log/slog`. One authoritative calculation package (`internal/calc`) without network/database dependencies. |
| Persistence | SQLite in WAL mode with the cgo-free `modernc.org/sqlite` driver, foreign keys on, transactions for domain changes. Plain `database/sql` queries; migrations with **goose** embedded in the binary. No sqlc (one fewer generator for ~20 tables). |
| Authentication | Password (Argon2id via `alexedwards/argon2id`) and OIDC (Pocket ID; `zitadel/oidc` relying party with PKCE, state and nonce). Own server-side session table, `http.CrossOriginProtection` for CSRF. |
| Frontend | React SPA built with Vite: TanStack Router (file-based), TanStack Query, TanStack Form where a form grows, shadcn/ui (Base UI) copied into the repo, Tailwind CSS 4, lucide icons. Go embeds and serves the built SPA. |
| API contract | Huma generates `openapi.json`; **Hey API** (`@hey-api/openapi-ts`) generates the TypeScript client, Zod schemas and TanStack Query hooks. Generated files are never edited; CI regenerates and fails on a diff. Structured error codes are translated in the UI. |
| Localization | Paraglide JS with `messages/{de,en}.json`; locale-aware amounts and dates. German is the default UI language. |
| PDF | Typst CLI as a subprocess with fixed local templates and bundled fonts. Typst embeds receipt images and PDF pages, so the claim PDF can append every receipt page. |
| Tooling | `mise.toml` pins Go, Node and pnpm. pnpm, Biome, `tsc --noEmit`, Vitest, Playwright for the frontend; `gofmt`, `go vet`, `go test` for Go. |
| Deployment | One multi-stage Dockerfile, nonroot runtime, linux/amd64 and linux/arm64, published to GHCR. Persistent data volume, read-only root filesystem, TLS at the reverse proxy. Docker Compose is the documented way to run it. |

Keep the backend build cgo-free. Typst is an additional runtime binary; "one application container" does not mean literally one executable.

## Authentication contract

- Passwords use Argon2id. Sessions use random tokens, HttpOnly cookies, SameSite policy, Secure in HTTPS deployments, expiry and server-side revocation. Protect unsafe browser requests against CSRF and bound login attempts. Setup requires an operator-supplied secret or one-time setup token; disable setup after an admin exists.
- OIDC must work with Pocket ID and conforming providers. Validate issuer, signature, audience, expiry, state and nonce; use Authorization Code with PKCE. Stable external identity is `(issuer, subject)`, not email. Disable automatic email-based linking by default. Allow explicit operator-authorized identity linking with an audit event.
- Configure allowed/admin group claims and mapping. Provision an OIDC user only when allowed. Reevaluate group-derived roles on login; distinguish them from local admin membership. Local deactivation takes precedence over all login modes. Protect the last active administrator from accidental removal.
- Trusted-proxy header login is deferred (owner decision 2026-10-19); Pocket ID is used through OIDC directly. Do not merge accounts because display names/emails happen to match. Logout must end local access and document the OIDC provider session behavior.
- Configure password and OIDC modes independently via environment variables with file-based secret alternatives. Invalid combinations fail at startup; at least one login mode must remain usable. Test each mode and the combination.

## Application scope

Use one UI shell; administrative functions can appear in the same navigation/settings. The initial owner uses the complete interface. Keep authorization and per-user data isolation even when functions are visible.

An expense has an employer and payer and may have a trip. Enforce a single selection path: a travel expense enters a claim through its trip, a standalone outlay enters directly. Snapshot/locking/export behavior covers both. Company-paid travel costs are informational; the reimbursement method determines which lodging amount, if any, is payable. Avoid separate duplicate expense stores for travel and non-travel purchases.

## Data and concurrency

Keep ownership explicit and enforce it server-side, including list filters and nested resource references. A normal user's missing/foreign object lookup should not reveal another user's object. Admin operational views reveal system/account state, not receipt content.

Use optimistic concurrency for editable records. Couple financial mutations and audit events in the same transaction. Keep rate provenance and claim snapshots independent of later edits to master data. Serialize the final daily-allocation check with submission so two requests cannot spend the same daily entitlement.

Persist submission work and recover it after restart. Files and SQLite do not share a transaction: stage files, verify checksums, commit their publication with the snapshot/version, and recover or clean up unpublished staging files. Define retry identity and failure behavior before implementing submission. Broad background-job infrastructure is optional; durable submission behavior is required.

Use portable SQL where practical. Postgres support, dual-engine code generation, event sourcing and a generic repository framework are outside Core.

## Receipt storage and PDF default

Preserve uploaded JPEG/PNG/PDF bytes and SHA-256 under generated storage keys. A browser camera capture produces an uploaded image; the application makes no promise to preserve a device's separate full-resolution camera file. Make this distinction clear to users. Originals may contain EXIF/GPS and must receive the same access protection as financial records. Strip metadata from displayed/export derivatives where possible.

Generate modest JPEG/PNG previews and, when needed, JPEG export derivatives. Defaults should favor legibility; no universal byte-size or SSIM threshold is specified. The original remains authoritative. Automatic corner detection, perspective correction, shadow removal, AVIF/WebP encoders and disposal of originals are outside Core.

Core PDF: one document with employer/user identity, claim number/version/period, travel/outlay subtotals, totals and advances, trip overview, standalone outlays, daily calculation details, mileage, expenses/VAT/FX, warnings, source references and receipt numbers, followed by every receipt page (images and PDF pages) labelled with its receipt number. The ZIP additionally carries all referenced originals. Use versioned English JSON/CSV filenames and fields, UTF-8, ISO dates, currency codes and unambiguous decimal amounts. Document the CSV dialect. Expose payer, actual gross and reimbursable EUR amount separately, and distinguish trip-linked from standalone expenses. Provide clear receipt-to-expense references and a manifest for every payload file except the manifest itself.

Use temporary directories, subprocess deadlines and fixed templates; pass user content as data. Export requires no network access. Typst 0.15 was verified to embed PDF pages as images (2026-10-19); PDF/A remains an extension. Content consistency is required; byte-identical fresh PDF generation across tool versions is not. Downloading an existing version returns its stored bytes.

## Security and operation

Validate type by file content, cap upload bytes/pixels/PDF pages, reject malformed inputs and protect previews/subprocesses against resource exhaustion. Baseline limits: 25 MiB per file, 40 megapixels per image, 50 PDF pages, configurable downward. Reject unsupported/encrypted PDFs with a recoverable explanation. Use safe generated paths and reject path traversal. Treat uploaded PDFs as untrusted content when serving or previewing them.

Keep secrets, receipt bodies and passwords out of logs. Serve a strict browser security policy. Do not cache authenticated API/file responses in a service worker. Stop cross-user access even when an attacker guesses a storage key.

Provide documented environment configuration, startup validation and an example without real secrets. Run with a read-only root filesystem, writable data volume and bounded temporary storage. Health endpoints reveal no secrets. Multi-arch builds must include usable Typst binaries on both targets; prefer native/cross builds without QEMU where compatible.

For Core backup, stop/quiesce writes or use a documented consistent snapshot procedure covering both SQLite and referenced files. Restore into a clean location and verify a known submitted claim and its checksums. A copy of a live SQLite main file alone is not a complete backup.

CI must check generation drift, formatting/types, backend and relevant frontend tests, critical E2E flows, migrations, both target builds and container startup. Keep release publishing separate from Core completion. Add dependency update automation and production security scans when preparing a release.

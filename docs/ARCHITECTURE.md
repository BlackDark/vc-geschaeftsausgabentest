# Architecture

## Retained choices

| Area | Constraint |
| --- | --- |
| Backend | Go, standard-library `net/http` routing, structured logging. One authoritative calculation module with no network/database dependencies. |
| Persistence | SQLite in WAL mode using the cgo-free `modernc.org/sqlite` driver; `sqlc` for queries and `goose` for migrations. Enable foreign keys and use transactions for domain changes. |
| Frontend | Svelte 5, TypeScript, Vite SPA, shadcn-svelte and Tailwind CSS 4. Go embeds and serves the built SPA. |
| API | OpenAPI as the contract, generated Go types/server support and TypeScript client/types. Structured error codes with UI translations. |
| Localization | German/English message catalogs; retain Paraglide unless a verified compatibility issue warrants a documented replacement. Use locale-aware amounts/dates. |
| PDF | Typst for the claim summary, with local templates and fonts. Receipt rasterization, codecs and PDF/A tooling remain replaceable. |
| Tooling | pnpm and Node 24 LTS for frontend builds. Biome for supported formatting/linting, `svelte-check` for types, Go formatting and `golangci-lint`. No ESLint requirement. |
| Verification | Go tests, Vitest for meaningful frontend logic, Playwright for complete user flows against the built application. |
| Deployment | One multi-stage Dockerfile, nonroot runtime, linux/amd64 and linux/arm64. A persistent data volume and temporary working space. TLS terminates at the reverse proxy. |

Resolve actual compatible package/tool versions at implementation time and pin them in manifests/lockfiles. Historical patch versions and version-compatibility claims are not requirements. Keep the backend build cgo-free. Typst is an additional runtime binary; “one application container” does not mean literally one executable.

## Authentication contract

- Passwords use Argon2id. Sessions use random tokens, HttpOnly cookies, SameSite policy, Secure in HTTPS deployments, expiry and server-side revocation. Protect unsafe browser requests against CSRF and bound login attempts. Setup requires an operator-supplied secret or one-time setup token; disable setup after an admin exists.
- OIDC must work with Pocket ID and conforming providers. Validate issuer, signature, audience, expiry, state and nonce; use Authorization Code with PKCE. Stable external identity is `(issuer, subject)`, not email. Disable automatic email-based linking by default. Allow explicit operator-authorized identity linking with an audit event.
- Configure allowed/admin group claims and mapping. Provision an OIDC user only when allowed. Reevaluate group-derived roles on login; distinguish them from local admin membership. Local deactivation takes precedence over all login modes. Protect the last active administrator from accidental removal.
- Proxy login accepts configured identity/group headers only from explicitly trusted proxy CIDRs. The proxy must strip client-supplied identity headers before setting its own. Determine trust from the direct network peer; forwarded headers do not establish trust. Reject untrusted identity headers, and ensure an alternate password/OIDC route cannot accidentally adopt them.
- Bind proxy users to a stable configured proxy identity namespace and subject; apply allowed/admin groups and controlled provisioning. Do not merge accounts because display names/emails happen to match. Logout must end local access and document any external provider/proxy session behavior.
- Configure password, OIDC and proxy modes independently via environment variables with file-based secret alternatives. Invalid combinations fail at startup; at least one login mode must remain usable. Test each mode and combinations.

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

Core PDF: employer/user identity, claim number/version/period, travel/outlay subtotals, totals and advances, trip overview, standalone outlays, daily calculation details, mileage, expenses/VAT/FX, warnings, source references and receipt filenames/numbers. The ZIP carries all referenced originals. Use versioned English JSON/CSV filenames and fields, UTF-8, ISO dates, currency codes and unambiguous decimal amounts. Document the CSV dialect. Expose payer, actual gross and reimbursable EUR amount separately, and distinguish trip-linked from standalone expenses. Provide clear receipt-to-expense references and a manifest for every payload file except the manifest itself.

Use temporary directories, subprocess deadlines and fixed templates; pass user content as data. Export requires no network access. Verify actual Typst capabilities before promising a PDF standard or PDF receipt embedding. Content consistency is required; byte-identical fresh PDF generation across tool versions is not. Downloading an existing version returns its stored bytes.

## Security and operation

Validate type by file content, cap upload bytes/pixels/PDF pages, reject malformed inputs and protect previews/subprocesses against resource exhaustion. Baseline limits: 25 MiB per file, 40 megapixels per image, 50 PDF pages, configurable downward. Reject unsupported/encrypted PDFs with a recoverable explanation. Use safe generated paths and reject path traversal. Treat uploaded PDFs as untrusted content when serving or previewing them.

Keep secrets, receipt bodies and passwords out of logs. Serve a strict browser security policy. Do not cache authenticated API/file responses in a service worker. Stop cross-user access even when an attacker guesses a storage key.

Provide documented environment configuration, startup validation and an example without real secrets. Run with a read-only root filesystem, writable data volume and bounded temporary storage. Health endpoints reveal no secrets. Multi-arch builds must include usable Typst binaries on both targets; prefer native/cross builds without QEMU where compatible.

For Core backup, stop/quiesce writes or use a documented consistent snapshot procedure covering both SQLite and referenced files. Restore into a clean location and verify a known submitted claim and its checksums. A copy of a live SQLite main file alone is not a complete backup.

CI must check generation drift, formatting/types, backend and relevant frontend tests, critical E2E flows, migrations, both target builds and container startup. Keep release publishing separate from Core completion. Add dependency update automation and production security scans when preparing a release.

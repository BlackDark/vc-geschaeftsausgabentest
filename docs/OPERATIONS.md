# Operations

Spesen runs as one container (server binary plus the Typst CLI) with one persistent volume. TLS
terminates at your reverse proxy. The image runs as `nonroot` and works with a read-only root
filesystem and a tmpfs on `/tmp` (see [`compose.yaml`](../compose.yaml)).

## Configuration

All settings are environment variables. Secrets can also be read from a file: append `_FILE` to
the variable name. Invalid combinations stop the server at startup with a list of all problems.

| Variable | Default | Meaning |
| --- | --- | --- |
| `SPESEN_BASE_URL` | `http://localhost:8080` | Public URL. `https://` enables Secure cookies and is required for the OIDC redirect. |
| `SPESEN_DATA_DIR` | `/data` | SQLite database (`spesen.db`), originals (`files/`), exports (`exports/`). |
| `SPESEN_LISTEN` | `:8080` | Listen address. |
| `SPESEN_SETUP_TOKEN` | – | Token for creating the first administrator. Without it, a random token is written to `$SPESEN_DATA_DIR/setup-token` while no administrator exists. Setup is disabled once an administrator exists. |
| `SPESEN_PASSWORD_LOGIN` | `true` | Password login. Keep it as an emergency route even with OIDC. |
| `SPESEN_OIDC_ISSUER` | – | Enables OIDC, e.g. `https://id.example.com` (Pocket ID). |
| `SPESEN_OIDC_CLIENT_ID` / `_CLIENT_SECRET` | – | Client credentials. |
| `SPESEN_OIDC_LABEL` | `Pocket ID` | Button text on the login page. |
| `SPESEN_OIDC_SCOPES` | `openid,profile,email,groups` | Requested scopes. |
| `SPESEN_OIDC_GROUPS_CLAIM` | `groups` | Claim holding the user's groups. |
| `SPESEN_OIDC_ALLOWED_GROUPS` | – | Unknown identities in one of these groups get an account automatically. Empty: no automatic accounts, link identities in the settings instead. |
| `SPESEN_OIDC_ADMIN_GROUPS` | – | Members get admin rights; re-evaluated on every OIDC login. |
| `SPESEN_TYPST` | `typst` | Typst binary (the image sets `/usr/local/bin/typst`). |
| `SPESEN_MAX_UPLOAD_MB` | `25` | Upload limit per file (1–25). |
| `SPESEN_MAX_MEGAPIXELS` | `40` | Image size limit (1–40). |
| `SPESEN_MAX_PDF_PAGES` | `50` | PDF page limit (1–50). |
| `SPESEN_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. Logs never contain passwords, tokens or receipt contents. |

## First start

1. Start the container with `SPESEN_SETUP_TOKEN` (or read `/data/setup-token`).
2. Open the URL, enter the token and create your account. You land on the settings page:
   enter your employer and your details (they appear on every claim).
3. Optional: link Pocket ID under *Settings → Sign-in* (see below).

## Pocket ID

1. In Pocket ID create an OIDC client "Spesen" with callback URL
   `https://<your-host>/auth/oidc/callback`. Pocket ID clients use PKCE; Spesen always sends it.
2. Set `SPESEN_OIDC_ISSUER`, `SPESEN_OIDC_CLIENT_ID` and `SPESEN_OIDC_CLIENT_SECRET(_FILE)`.
3. Log in with your password once and click *Settings → Sign-in → Link*. From then on the
   *Log in with Pocket ID* button signs you in. The identity is bound to issuer and subject,
   never to the email address.
4. For additional users either create accounts as admin and let them link, or set
   `SPESEN_OIDC_ALLOWED_GROUPS` to provision members of a group automatically.

Logout ends the Spesen session only; the Pocket ID session stays active, so the next click on
*Log in with Pocket ID* signs in again without a password prompt.

## Backup and restore

A copy of the live SQLite file alone is not a backup. Use the built-in command; it writes a
consistent database snapshot (`VACUUM INTO`) and then copies the originals and exports. It is safe
while the server runs.

```bash
docker compose exec spesen spesen backup /data/backup-$(date +%F)
# copy /data/backup-YYYY-MM-DD to your backup storage, then remove it from the volume
```

Restore into a clean location and verify every checksum before switching over:

```bash
docker compose down
# empty volume or new directory, then copy the backup's contents to its root:
#   spesen.db, files/, exports/
docker compose run --rm spesen verify
docker compose up -d
```

`spesen verify` recomputes the SHA-256 of every receipt original and every exported PDF/ZIP and
compares it with the database; it exits non-zero on any mismatch. The procedure is exercised by
`TestF01FirstWorkingFlow` (backup, restore into a new directory, verify, detect a tampered file).

## Further rate years

The 2024–2026 domestic and foreign rates are seeded on first start. Import a new official year
from a CSV in the format of `data/foreign-rates-2026.csv`:

```bash
docker compose exec spesen spesen import-rates 2027 /data/foreign-rates-2027.csv "BMF-Schreiben Reisekosten 2027"
docker compose restart spesen
```

The import validates header, year, country mapping, unique keys and amounts and names the failing
row. It creates a new revision; submitted claims keep the rates frozen in their snapshot.
Domestic rates for a new year are seeded in `internal/rates` (they rarely change).

## Health

`GET /healthz` returns `ok` when the database is reachable (no secrets). The image's
`HEALTHCHECK` runs `spesen healthcheck`.

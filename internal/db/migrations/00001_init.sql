-- +goose Up
CREATE TABLE users (
    id              INTEGER PRIMARY KEY,
    username        TEXT UNIQUE,
    name            TEXT NOT NULL,
    email           TEXT NOT NULL DEFAULT '',
    password_hash   TEXT,
    is_admin        INTEGER NOT NULL DEFAULT 0,
    oidc_admin      INTEGER NOT NULL DEFAULT 0,
    active          INTEGER NOT NULL DEFAULT 1,
    oidc_issuer     TEXT,
    oidc_subject    TEXT,
    locale          TEXT NOT NULL DEFAULT 'de',
    theme           TEXT NOT NULL DEFAULT 'system',
    employee_number TEXT NOT NULL DEFAULT '',
    address         TEXT NOT NULL DEFAULT '',
    iban            TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    UNIQUE (oidc_issuer, oidc_subject)
);

CREATE TABLE sessions (
    token_hash  TEXT PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    method      TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    expires_at  TEXT NOT NULL
);
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE login_failures (
    key TEXT NOT NULL,
    at  TEXT NOT NULL
);
CREATE INDEX login_failures_key ON login_failures(key, at);

CREATE TABLE employers (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    name        TEXT NOT NULL,
    address     TEXT NOT NULL DEFAULT '',
    is_default  INTEGER NOT NULL DEFAULT 1,
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL
);
CREATE INDEX employers_user ON employers(user_id);

CREATE TABLE claims (
    id              INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    employer_id     INTEGER NOT NULL REFERENCES employers(id),
    title           TEXT NOT NULL DEFAULT '',
    period_start    TEXT NOT NULL,
    period_end      TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'submitted', 'paid')),
    number          TEXT,
    number_year     INTEGER,
    number_seq      INTEGER,
    current_version INTEGER NOT NULL DEFAULT 0,
    paid_at         TEXT,
    paid_note       TEXT NOT NULL DEFAULT '',
    submitting      INTEGER NOT NULL DEFAULT 0,
    version         INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE (user_id, employer_id, number_year, number_seq)
);
CREATE INDEX claims_user ON claims(user_id);

CREATE TABLE trips (
    id             INTEGER PRIMARY KEY,
    user_id        INTEGER NOT NULL REFERENCES users(id),
    employer_id    INTEGER NOT NULL REFERENCES employers(id),
    claim_id       INTEGER REFERENCES claims(id),
    purpose        TEXT NOT NULL,
    destination    TEXT NOT NULL DEFAULT '',
    start_at       TEXT NOT NULL,
    start_tz       TEXT NOT NULL,
    end_at         TEXT NOT NULL,
    end_tz         TEXT NOT NULL,
    origin_country TEXT NOT NULL DEFAULT 'DE',
    project        TEXT NOT NULL DEFAULT '',
    notes          TEXT NOT NULL DEFAULT '',
    version        INTEGER NOT NULL DEFAULT 1,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);
CREATE INDEX trips_user ON trips(user_id, end_at);

CREATE TABLE trip_stops (
    id         INTEGER PRIMARY KEY,
    trip_id    INTEGER NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    arrival_at TEXT NOT NULL,
    tz         TEXT NOT NULL,
    country    TEXT NOT NULL,
    city       TEXT NOT NULL DEFAULT '',
    business   INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE trip_days (
    trip_id          INTEGER NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    date             TEXT NOT NULL,
    breakfast        INTEGER NOT NULL DEFAULT 0,
    lunch            INTEGER NOT NULL DEFAULT 0,
    dinner           INTEGER NOT NULL DEFAULT 0,
    contrib_breakfast INTEGER NOT NULL DEFAULT 0,
    contrib_lunch    INTEGER NOT NULL DEFAULT 0,
    contrib_dinner   INTEGER NOT NULL DEFAULT 0,
    breakfast_origin TEXT NOT NULL DEFAULT '',
    override_country TEXT,
    override_city    TEXT,
    override_reason  TEXT NOT NULL DEFAULT '',
    excluded         INTEGER NOT NULL DEFAULT 0,
    exclude_reason   TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (trip_id, date)
);

CREATE TABLE trip_nights (
    trip_id INTEGER NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    date    TEXT NOT NULL,
    kind    TEXT NOT NULL,
    PRIMARY KEY (trip_id, date)
);

CREATE TABLE journeys (
    id          INTEGER PRIMARY KEY,
    trip_id     INTEGER NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    date        TEXT NOT NULL,
    vehicle     TEXT NOT NULL,
    distance_m  INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE expenses (
    id               INTEGER PRIMARY KEY,
    user_id          INTEGER NOT NULL REFERENCES users(id),
    employer_id      INTEGER NOT NULL REFERENCES employers(id),
    trip_id          INTEGER REFERENCES trips(id),
    claim_id         INTEGER REFERENCES claims(id),
    date             TEXT NOT NULL,
    cost_type        TEXT NOT NULL,
    payer            TEXT NOT NULL CHECK (payer IN ('user', 'employer')),
    supplier         TEXT NOT NULL DEFAULT '',
    description      TEXT NOT NULL DEFAULT '',
    currency         TEXT NOT NULL DEFAULT 'EUR',
    gross_minor      INTEGER NOT NULL,
    fx_rate          TEXT NOT NULL DEFAULT '',
    fx_date          TEXT,
    eur_debit        INTEGER,
    vat_unknown      INTEGER NOT NULL DEFAULT 1,
    vat_json         TEXT NOT NULL DEFAULT '[]',
    nights_json      TEXT NOT NULL DEFAULT '[]',
    includes_breakfast INTEGER NOT NULL DEFAULT 0,
    fuel             INTEGER NOT NULL DEFAULT 0,
    hospitality_json TEXT,
    version          INTEGER NOT NULL DEFAULT 1,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    CHECK (trip_id IS NULL OR claim_id IS NULL)
);
CREATE INDEX expenses_user ON expenses(user_id, date);
CREATE INDEX expenses_trip ON expenses(trip_id);

CREATE TABLE receipts (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id),
    kind         TEXT NOT NULL DEFAULT 'file' CHECK (kind IN ('file', 'self_issued')),
    status       TEXT NOT NULL DEFAULT 'inbox' CHECK (status IN ('inbox', 'confirmed', 'void')),
    sha256       TEXT NOT NULL DEFAULT '',
    filename     TEXT NOT NULL DEFAULT '',
    mime         TEXT NOT NULL DEFAULT '',
    size         INTEGER NOT NULL DEFAULT 0,
    pages        INTEGER NOT NULL DEFAULT 1,
    storage_key  TEXT NOT NULL DEFAULT '',
    preview_key  TEXT NOT NULL DEFAULT '',
    self_json    TEXT,
    number       TEXT,
    number_year  INTEGER,
    number_seq   INTEGER,
    confirmed_at TEXT,
    void_reason  TEXT NOT NULL DEFAULT '',
    note         TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    UNIQUE (user_id, number_year, number_seq)
);
CREATE INDEX receipts_user ON receipts(user_id, status);
CREATE INDEX receipts_sha ON receipts(user_id, sha256);

CREATE TABLE receipt_links (
    receipt_id INTEGER NOT NULL REFERENCES receipts(id),
    expense_id INTEGER NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    PRIMARY KEY (receipt_id, expense_id)
);

CREATE TABLE advances (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    employer_id INTEGER NOT NULL REFERENCES employers(id),
    claim_id    INTEGER REFERENCES claims(id),
    date        TEXT NOT NULL,
    amount      INTEGER NOT NULL CHECK (amount > 0),
    note        TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);

CREATE TABLE claim_versions (
    id            INTEGER PRIMARY KEY,
    claim_id      INTEGER NOT NULL REFERENCES claims(id),
    version       INTEGER NOT NULL,
    snapshot_json TEXT NOT NULL,
    payout        INTEGER NOT NULL,
    pdf_key       TEXT NOT NULL,
    pdf_sha256    TEXT NOT NULL,
    zip_key       TEXT NOT NULL,
    zip_sha256    TEXT NOT NULL,
    submitted_at  TEXT NOT NULL,
    superseded_at TEXT,
    unlock_reason TEXT NOT NULL DEFAULT '',
    UNIQUE (claim_id, version)
);

-- Meal allowance frozen per trip and date by a submitted claim (C07).
CREATE TABLE frozen_days (
    trip_id  INTEGER NOT NULL REFERENCES trips(id),
    date     TEXT NOT NULL,
    claim_id INTEGER NOT NULL REFERENCES claims(id),
    cents    INTEGER NOT NULL,
    PRIMARY KEY (trip_id, date)
);

CREATE TABLE meal_rates (
    id         INTEGER PRIMARY KEY,
    year       INTEGER NOT NULL,
    country    TEXT NOT NULL,
    city       TEXT NOT NULL DEFAULT '',
    label      TEXT NOT NULL DEFAULT '',
    full_day   INTEGER NOT NULL,
    partial    INTEGER NOT NULL,
    lodging    INTEGER NOT NULL,
    source     TEXT NOT NULL,
    revision   INTEGER NOT NULL DEFAULT 1,
    active     INTEGER NOT NULL DEFAULT 1,
    reason     TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE (year, country, city, revision)
);

CREATE TABLE mileage_rates (
    id           INTEGER PRIMARY KEY,
    vehicle      TEXT NOT NULL,
    valid_from   TEXT NOT NULL,
    cents_per_km INTEGER NOT NULL,
    source       TEXT NOT NULL,
    revision     INTEGER NOT NULL DEFAULT 1,
    active       INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT NOT NULL
);

CREATE TABLE countries (
    iso      TEXT PRIMARY KEY,
    bmf_name TEXT NOT NULL
);

CREATE TABLE audit_log (
    id         INTEGER PRIMARY KEY,
    owner_id   INTEGER,
    actor_id   INTEGER,
    entity     TEXT NOT NULL,
    entity_id  INTEGER,
    action     TEXT NOT NULL,
    reason     TEXT NOT NULL DEFAULT '',
    data_json  TEXT NOT NULL DEFAULT '{}',
    at         TEXT NOT NULL
);
CREATE INDEX audit_owner ON audit_log(owner_id, at);

-- +goose Down
SELECT 1;

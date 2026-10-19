package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
)

// User is an account. Admin is effective admin (local or OIDC group).
type User struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Admin          bool   `json:"admin"`
	LocalAdmin     bool   `json:"localAdmin"`
	Active         bool   `json:"active"`
	Locale         string `json:"locale"`
	Theme          string `json:"theme"`
	EmployeeNumber string `json:"employeeNumber"`
	Address        string `json:"address"`
	IBAN           string `json:"iban"`
	OIDCLinked     bool   `json:"oidcLinked"`
	HasPassword    bool   `json:"hasPassword"`
}

const userCols = `id, coalesce(username, ''), name, email, is_admin, oidc_admin, active, locale, theme, employee_number, address, iban, oidc_subject IS NOT NULL, password_hash IS NOT NULL`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var u User
	var local, oidcAdmin bool
	if err := row.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &local, &oidcAdmin, &u.Active, &u.Locale, &u.Theme, &u.EmployeeNumber, &u.Address, &u.IBAN, &u.OIDCLinked, &u.HasPassword); err != nil {
		return nil, notFound(err)
	}
	u.LocalAdmin = local
	u.Admin = local || oidcAdmin
	return &u, nil
}

// UserByID loads a user.
func (a *App) UserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(a.DB.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// NeedsSetup reports whether no administrator exists yet.
func (a *App) NeedsSetup(ctx context.Context) (bool, error) {
	var n int
	err := a.DB.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE is_admin = 1`).Scan(&n)
	return n == 0, err
}

// PasswordPolicy validates a new password.
func PasswordPolicy(pw string) error {
	if len([]rune(pw)) < 10 {
		return Invalid("password.too_short", "password must have at least 10 characters")
	}
	return nil
}

func hashPassword(pw string) (string, error) {
	return argon2id.CreateHash(pw, argon2id.DefaultParams)
}

// Setup creates the first administrator. It is single-use and requires the
// operator's setup token (P02).
func (a *App) Setup(ctx context.Context, token, wantToken, username, name, password string) (*User, error) {
	if wantToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(wantToken)) != 1 {
		return nil, Invalid("setup.token_invalid", "setup token is invalid")
	}
	var id int64
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE is_admin = 1`).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return &Error{Status: 409, Code: "setup.done", Message: "setup has already been completed"}
		}
		var err error
		id, err = a.insertUser(ctx, tx, username, name, password, true)
		if err != nil {
			return err
		}
		return a.audit(ctx, tx, id, id, "user", id, "setup", "", map[string]any{"username": username})
	})
	if err != nil {
		return nil, err
	}
	return a.UserByID(ctx, id)
}

func (a *App) insertUser(ctx context.Context, tx *sql.Tx, username, name, password string, admin bool) (int64, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" || strings.TrimSpace(name) == "" {
		return 0, Invalid("user.required", "username and name are required")
	}
	if err := PasswordPolicy(password); err != nil {
		return 0, err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO users(username, name, password_hash, is_admin, created_at) VALUES (?, ?, ?, ?, ?)`, username, strings.TrimSpace(name), hash, boolInt(admin), a.stamp())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, Invalid("user.username_taken", "username already exists")
		}
		return 0, err
	}
	return res.LastInsertId()
}

// CreateUser lets an admin create an account (no public registration).
func (a *App) CreateUser(ctx context.Context, actor *User, username, name, password string, admin bool) (*User, error) {
	if !actor.Admin {
		return nil, ErrForbidden
	}
	var id int64
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var err error
		if id, err = a.insertUser(ctx, tx, username, name, password, admin); err != nil {
			return err
		}
		return a.audit(ctx, tx, id, actor.ID, "user", id, "create", "", map[string]any{"username": username, "admin": admin})
	})
	if err != nil {
		return nil, err
	}
	return a.UserByID(ctx, id)
}

// AccountSummary is what an admin sees about an account: no travel data.
type AccountSummary struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Admin      bool   `json:"admin"`
	Active     bool   `json:"active"`
	OIDCLinked bool   `json:"oidcLinked"`
}

// ListUsers lists accounts for administrators.
func (a *App) ListUsers(ctx context.Context, actor *User) ([]AccountSummary, error) {
	if !actor.Admin {
		return nil, ErrForbidden
	}
	rows, err := a.DB.QueryContext(ctx, `SELECT id, coalesce(username, ''), name, is_admin OR oidc_admin, active, oidc_subject IS NOT NULL FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AccountSummary{}
	for rows.Next() {
		var s AccountSummary
		if err := rows.Scan(&s.ID, &s.Username, &s.Name, &s.Admin, &s.Active, &s.OIDCLinked); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetActive activates or deactivates an account; deactivation ends its
// sessions immediately. The last active administrator is protected.
func (a *App) SetActive(ctx context.Context, actor *User, id int64, active bool, reason string) error {
	if !actor.Admin {
		return ErrForbidden
	}
	return a.tx(ctx, func(tx *sql.Tx) error {
		if !active {
			var others int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE active = 1 AND (is_admin = 1 OR oidc_admin = 1) AND id <> ?`, id).Scan(&others); err != nil {
				return err
			}
			var isAdmin bool
			if err := tx.QueryRowContext(ctx, `SELECT is_admin OR oidc_admin FROM users WHERE id = ?`, id).Scan(&isAdmin); err != nil {
				return notFound(err)
			}
			if isAdmin && others == 0 {
				return Invalid("user.last_admin", "the last active administrator cannot be deactivated")
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
				return err
			}
		}
		res, err := tx.ExecContext(ctx, `UPDATE users SET active = ? WHERE id = ?`, boolInt(active), id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		action := "activate"
		if !active {
			action = "deactivate"
		}
		return a.audit(ctx, tx, id, actor.ID, "user", id, action, reason, nil)
	})
}

// ProfileInput updates the user's own profile.
type ProfileInput struct {
	Name           string `json:"name" minLength:"1" maxLength:"200"`
	Email          string `json:"email" maxLength:"200"`
	Locale         string `json:"locale" enum:"de,en"`
	Theme          string `json:"theme" enum:"system,light,dark"`
	EmployeeNumber string `json:"employeeNumber" maxLength:"50"`
	Address        string `json:"address" maxLength:"500"`
	IBAN           string `json:"iban" maxLength:"40"`
}

// UpdateProfile saves profile fields.
func (a *App) UpdateProfile(ctx context.Context, u *User, in ProfileInput) (*User, error) {
	err := a.tx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE users SET name = ?, email = ?, locale = ?, theme = ?, employee_number = ?, address = ?, iban = ? WHERE id = ?`,
			strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), in.Locale, in.Theme, strings.TrimSpace(in.EmployeeNumber), strings.TrimSpace(in.Address), strings.ReplaceAll(strings.ToUpper(in.IBAN), " ", ""), u.ID)
		if err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "user", u.ID, "profile", "", nil)
	})
	if err != nil {
		return nil, err
	}
	return a.UserByID(ctx, u.ID)
}

// ChangePassword changes the user's own password.
func (a *App) ChangePassword(ctx context.Context, u *User, current, next string) error {
	var hash sql.NullString
	if err := a.DB.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, u.ID).Scan(&hash); err != nil {
		return err
	}
	if hash.Valid {
		ok, err := argon2id.ComparePasswordAndHash(current, hash.String)
		if err != nil || !ok {
			return Invalid("password.wrong", "current password is wrong")
		}
	}
	if err := PasswordPolicy(next); err != nil {
		return err
	}
	h, err := hashPassword(next)
	if err != nil {
		return err
	}
	return a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, h, u.ID); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "user", u.ID, "password", "", nil)
	})
}

const (
	loginWindow      = 15 * time.Minute
	loginMaxFailures = 5
	// SessionTTL is the lifetime of a login session.
	SessionTTL = 30 * 24 * time.Hour
)

// ErrTooManyAttempts limits password guessing (P02).
var ErrTooManyAttempts = &Error{Status: 429, Code: "login.rate_limited", Message: "too many failed attempts; try again later"}

// dummyHash keeps timing similar for unknown usernames.
var dummyHash, _ = argon2id.CreateHash("dummy-password-for-timing", argon2id.DefaultParams)

// PasswordLogin verifies credentials, limited per username and client.
func (a *App) PasswordLogin(ctx context.Context, username, password, client string) (*User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	keys := []string{"u:" + username, "c:" + client}
	since := a.now().Add(-loginWindow).UTC().Format(time.RFC3339)
	for _, k := range keys {
		var n int
		if err := a.DB.QueryRowContext(ctx, `SELECT count(*) FROM login_failures WHERE key = ? AND at > ?`, k, since).Scan(&n); err != nil {
			return nil, err
		}
		if n >= loginMaxFailures {
			return nil, ErrTooManyAttempts
		}
	}
	var id int64
	var hash sql.NullString
	var active bool
	err := a.DB.QueryRowContext(ctx, `SELECT id, password_hash, active FROM users WHERE username = ?`, username).Scan(&id, &hash, &active)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	h := dummyHash
	if hash.Valid {
		h = hash.String
	}
	ok, _ := argon2id.ComparePasswordAndHash(password, h)
	if !ok || !hash.Valid || !active {
		for _, k := range keys {
			if _, err := a.DB.ExecContext(ctx, `INSERT INTO login_failures(key, at) VALUES (?, ?)`, k, a.stamp()); err != nil {
				return nil, err
			}
		}
		return nil, &Error{Status: 401, Code: "login.invalid", Message: "username or password is wrong"}
	}
	if _, err := a.DB.ExecContext(ctx, `DELETE FROM login_failures WHERE key = ? OR at < ?`, keys[0], since); err != nil {
		return nil, err
	}
	return a.UserByID(ctx, id)
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CreateSession starts a session and returns its opaque token.
func (a *App) CreateSession(ctx context.Context, userID int64, method string) (string, error) {
	raw := randomHex(32)
	token := base64.RawURLEncoding.EncodeToString([]byte(raw))
	now := a.now().UTC()
	return token, a.tx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now.Format(time.RFC3339)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, method, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
			tokenHash(token), userID, method, now.Format(time.RFC3339), now.Add(SessionTTL).Format(time.RFC3339)); err != nil {
			return err
		}
		return a.audit(ctx, tx, userID, userID, "session", userID, "login", "", map[string]any{"method": method})
	})
}

// SessionUser resolves a session token to an active user.
func (a *App) SessionUser(ctx context.Context, token string) (*User, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	u, err := scanUser(a.DB.QueryRowContext(ctx, `SELECT `+prefixed("u.", userCols)+` FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = ? AND s.expires_at > ? AND u.active = 1`,
		tokenHash(token), a.stamp()))
	if err != nil {
		return nil, ErrUnauthorized
	}
	return u, nil
}

// EndSession deletes a session (logout).
func (a *App) EndSession(ctx context.Context, token string) error {
	_, err := a.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash(token))
	return err
}

func prefixed(p, cols string) string {
	parts := strings.Split(cols, ", ")
	for i, c := range parts {
		if strings.HasPrefix(c, "coalesce(") {
			parts[i] = strings.Replace(c, "coalesce(", "coalesce("+p, 1)
		} else {
			parts[i] = p + c
		}
	}
	return strings.Join(parts, ", ")
}

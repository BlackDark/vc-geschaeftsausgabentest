package app

import (
	"context"
	"database/sql"
	"slices"
)

// OIDCIdentity is a verified identity from the OIDC provider. The stable
// key is (Issuer, Subject), never the email (ARCHITECTURE: authentication).
type OIDCIdentity struct {
	Issuer  string
	Subject string
	Name    string
	Email   string
	Groups  []string
}

// OIDCPolicy controls provisioning and group-derived roles.
type OIDCPolicy struct {
	AllowedGroups []string // empty: no automatic provisioning
	AdminGroups   []string
}

func intersects(a, b []string) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

// OIDCLogin resolves an identity to a local account. Unknown identities are
// provisioned only when they belong to an allowed group. Admin rights from
// groups are re-evaluated on every login.
func (a *App) OIDCLogin(ctx context.Context, id OIDCIdentity, p OIDCPolicy) (*User, error) {
	var userID int64
	err := a.tx(ctx, func(tx *sql.Tx) error {
		var active bool
		err := tx.QueryRowContext(ctx, `SELECT id, active FROM users WHERE oidc_issuer = ? AND oidc_subject = ?`, id.Issuer, id.Subject).Scan(&userID, &active)
		switch {
		case err == sql.ErrNoRows:
			if len(p.AllowedGroups) == 0 || !intersects(id.Groups, p.AllowedGroups) {
				return &Error{Status: 403, Code: "oidc.not_provisioned", Message: "this identity is not linked to an account"}
			}
			name := id.Name
			if name == "" {
				name = id.Email
			}
			res, err := tx.ExecContext(ctx, `INSERT INTO users(name, email, oidc_issuer, oidc_subject, created_at) VALUES (?, ?, ?, ?, ?)`, name, id.Email, id.Issuer, id.Subject, a.stamp())
			if err != nil {
				return err
			}
			userID, _ = res.LastInsertId()
			if err := a.audit(ctx, tx, userID, userID, "user", userID, "oidc_provision", "", map[string]any{"issuer": id.Issuer}); err != nil {
				return err
			}
		case err != nil:
			return err
		case !active:
			return &Error{Status: 403, Code: "login.deactivated", Message: "account is deactivated"}
		}
		admin := intersects(id.Groups, p.AdminGroups)
		_, err = tx.ExecContext(ctx, `UPDATE users SET oidc_admin = ?, email = CASE WHEN ? <> '' THEN ? ELSE email END WHERE id = ?`, boolInt(admin), id.Email, id.Email, userID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return a.UserByID(ctx, userID)
}

// LinkOIDC links the logged-in user to an identity (explicit linking,
// audited). An identity can be linked to one account only.
func (a *App) LinkOIDC(ctx context.Context, u *User, id OIDCIdentity) error {
	return a.tx(ctx, func(tx *sql.Tx) error {
		var other int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE oidc_issuer = ? AND oidc_subject = ?`, id.Issuer, id.Subject).Scan(&other)
		if err == nil && other != u.ID {
			return Invalid("oidc.already_linked", "this identity is linked to another account")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET oidc_issuer = ?, oidc_subject = ? WHERE id = ?`, id.Issuer, id.Subject, u.ID); err != nil {
			return err
		}
		return a.audit(ctx, tx, u.ID, u.ID, "user", u.ID, "oidc_link", "", map[string]any{"issuer": id.Issuer})
	})
}

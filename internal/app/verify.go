package app

import (
	"context"
	"fmt"
)

// VerifyReport summarizes an integrity check of stored evidence and exports.
type VerifyReport struct {
	Receipts int      `json:"receipts"`
	Exports  int      `json:"exports"`
	Problems []string `json:"problems"`
}

// Verify recomputes the SHA-256 of every stored receipt original and every
// published export and compares it with the database (backup/restore check).
func (a *App) Verify(ctx context.Context) (*VerifyReport, error) {
	r := &VerifyReport{Problems: []string{}}
	type item struct{ label, key, sum string }
	var items []item
	rows, err := a.DB.QueryContext(ctx, `SELECT coalesce(number, 'upload #' || id), storage_key, sha256 FROM receipts WHERE kind = 'file'`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.label, &it.key, &it.sum); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, it)
		r.Receipts++
	}
	rows.Close()
	rows, err = a.DB.QueryContext(ctx, `SELECT coalesce(c.number, ''), v.version, v.pdf_key, v.pdf_sha256, v.zip_key, v.zip_sha256 FROM claim_versions v JOIN claims c ON c.id = v.claim_id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var number string
		var version int
		var pk, ps, zk, zs string
		if err := rows.Scan(&number, &version, &pk, &ps, &zk, &zs); err != nil {
			rows.Close()
			return nil, err
		}
		label := fmt.Sprintf("%s v%d", number, version)
		items = append(items, item{label + " PDF", pk, ps}, item{label + " ZIP", zk, zs})
		r.Exports += 2
	}
	rows.Close()
	for _, it := range items {
		sum, err := a.Files.SHA256(it.key)
		switch {
		case err != nil:
			r.Problems = append(r.Problems, fmt.Sprintf("%s: %v", it.label, err))
		case sum != it.sum:
			r.Problems = append(r.Problems, fmt.Sprintf("%s: checksum mismatch", it.label))
		}
	}
	return r, nil
}

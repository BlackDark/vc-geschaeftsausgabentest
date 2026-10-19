// Package data embeds the inherited rate datasets described in README.md.
package data

import "embed"

// Files holds the foreign-rate CSVs and the country-code mapping.
//
//go:embed *.csv
var Files embed.FS

// Package migrations embeds the SQL migration files for golang-migrate.
package migrations

import "embed"

// Files holds every *.sql migration file in this directory, in lexical order.
//
//go:embed *.sql
var Files embed.FS

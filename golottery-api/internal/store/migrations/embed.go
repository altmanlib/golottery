package migrations

import "embed"

// Files contains immutable SQL migrations applied in lexical order.
//
//go:embed *.sql
var Files embed.FS

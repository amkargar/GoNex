package migrations

import "embed"

// Files contains explicit migrations. Application startup never changes schema.
//
//go:embed *.sql
var Files embed.FS

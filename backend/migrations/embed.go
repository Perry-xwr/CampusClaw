package migrations

import "embed"

// Files contains the versioned SQL schema bundled into the API binary.
//
//go:embed *.sql
var Files embed.FS

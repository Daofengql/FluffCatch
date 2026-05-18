package migrations

import "embed"

// FS contains SQL migrations used by automatic startup migrations.
//
//go:embed *.sql sqlite/*.sql
var FS embed.FS

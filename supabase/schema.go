// Package dbschema embeds the application's immutable SQL migration files.
package dbschema

import "embed"

// Migrations is embedded so deployment does not depend on a working directory.
//
//go:embed migrations/*.sql
var Migrations embed.FS

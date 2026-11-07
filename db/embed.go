// Package db holds the goose migrations, embedded so the collector binary carries its
// own schema and needs no files next to it at run time.
package db

import "embed"

// Migrations is every migration under db/migrations, applied in version order.
//
//go:embed migrations/*.sql
var Migrations embed.FS

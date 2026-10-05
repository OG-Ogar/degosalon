package dbmigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io/fs"
	"strings"

	dbschema "barber-booking/supabase"
)

type migration struct{ version, checksum, sql string }

func files() ([]migration, error) {
	names, err := fs.Glob(dbschema.Migrations, "migrations/*.sql")
	if err != nil {
		return nil, err
	}
	result := make([]migration, 0, len(names))
	for _, name := range names {
		data, err := dbschema.Migrations.ReadFile(name)
		if err != nil {
			return nil, err
		}
		filename := strings.TrimPrefix(name, "migrations/")
		version, _, ok := strings.Cut(filename, "_")
		if !ok {
			return nil, fmt.Errorf("invalid migration filename %s", filename)
		}
		result = append(result, migration{version: version, checksum: fmt.Sprintf("%x", sha256.Sum256(data)), sql: string(data)})
	}
	return result, nil
}

type querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func history(ctx context.Context, conn querier) (map[string]string, error) {
	rows, err := conn.QueryContext(ctx, "select version,checksum from degosalon_internal.schema_migrations order by version")
	if err != nil {
		return nil, fmt.Errorf("read migration history (run go run ./cmd/migrate before starting the application): %w", err)
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, err
		}
		result[version] = checksum
	}
	return result, rows.Err()
}

func validate(all []migration, applied map[string]string) error {
	known := map[string]bool{}
	pending := false
	for _, file := range all {
		known[file.version] = true
		checksum, found := applied[file.version]
		if !found {
			pending = true
			continue
		}
		if pending {
			return fmt.Errorf("migration history is out of order at %s", file.version)
		}
		if checksum != file.checksum {
			return fmt.Errorf("applied migration %s differs from the embedded file", file.version)
		}
	}
	for version := range applied {
		if !known[version] {
			return fmt.Errorf("database has unknown migration %s; use the matching application version", version)
		}
	}
	return nil
}

// Verify checks schema versions without performing any DDL.
func Verify(ctx context.Context, conn *sql.DB) error {
	all, err := files()
	if err != nil {
		return err
	}
	applied, err := history(ctx, conn)
	if err != nil {
		return err
	}
	if err := validate(all, applied); err != nil {
		return err
	}
	if len(applied) != len(all) {
		return fmt.Errorf("database has pending migrations; run go run ./cmd/migrate before starting the application")
	}
	return nil
}

// Apply atomically applies pending migrations, recording their checksums. A
// transaction-scoped lock also serializes two concurrent migration commands.
func Apply(ctx context.Context, conn *sql.DB) error {
	all, err := files()
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "select pg_advisory_xact_lock(174740819,1)"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `set local lock_timeout='10s'; set local statement_timeout='60s';
  create schema if not exists degosalon_internal;
  revoke all on schema degosalon_internal from public;
  create table if not exists degosalon_internal.schema_migrations(version text primary key,checksum text not null,applied_at timestamptz not null default now());
  alter table degosalon_internal.schema_migrations enable row level security`); err != nil {
		return err
	}
	applied, err := history(ctx, tx)
	if err != nil {
		return err
	}
	if err := validate(all, applied); err != nil {
		return err
	}
	for _, file := range all {
		if _, found := applied[file.version]; found {
			continue
		}
		if _, err := tx.ExecContext(ctx, file.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", file.version, err)
		}
		if _, err := tx.ExecContext(ctx, "insert into degosalon_internal.schema_migrations(version,checksum) values($1,$2)", file.version, file.checksum); err != nil {
			return err
		}
	}
	return tx.Commit()
}

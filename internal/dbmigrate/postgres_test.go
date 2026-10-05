package dbmigrate_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"barber-booking/internal/dbmigrate"
	"barber-booking/internal/testdb"
)

func migrationDatabase(t *testing.T) *sql.DB {
	t.Helper()
	address := testdb.New(t, "")
	conn, err := sql.Open("pgx", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestPostgresMigrations(t *testing.T) {
	ctx := context.Background()
	t.Run("fresh concurrent apply and immutable history", func(t *testing.T) {
		conn := migrationDatabase(t)
		if err := dbmigrate.Verify(ctx, conn); err == nil {
			t.Fatal("fresh database reported ready")
		}
		outcomes := make(chan error, 2)
		for i := 0; i < 2; i++ {
			go func() { outcomes <- dbmigrate.Apply(ctx, conn) }()
		}
		for i := 0; i < 2; i++ {
			if err := <-outcomes; err != nil {
				t.Fatal(err)
			}
		}
		if err := dbmigrate.Verify(ctx, conn); err != nil {
			t.Fatal(err)
		}
		var unprotected int
		if err := conn.QueryRow("select count(*) from pg_tables where schemaname in ('public','degosalon_internal') and not rowsecurity").Scan(&unprotected); err != nil || unprotected != 0 {
			t.Fatalf("unprotected tables=%d %v", unprotected, err)
		}
		if _, err := conn.Exec("update degosalon_internal.schema_migrations set checksum='tampered'"); err != nil {
			t.Fatal(err)
		}
		if err := dbmigrate.Verify(ctx, conn); err == nil {
			t.Fatal("modified history accepted")
		}
		if err := dbmigrate.Apply(ctx, conn); err == nil {
			t.Fatal("modified history overwritten")
		}
	})
	t.Run("existing schema and records survive baseline", func(t *testing.T) {
		address := testdb.New(t, "")
		conn, err := sql.Open("pgx", address)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		schema, err := os.ReadFile("../../supabase/schema.sql")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(string(schema)); err != nil {
			t.Fatal(err)
		}
		if _, err := conn.Exec(`insert into public.customers(full_name,phone_number,email,password_hash) values('Existing customer','123','existing@example.com','hash');
   alter table public.barbers drop column state cascade;
   alter table public.barbers drop column local_area cascade`); err != nil {
			t.Fatal(err)
		}
		if err := dbmigrate.Apply(ctx, conn); err != nil {
			t.Fatal(err)
		}
		if err := dbmigrate.Verify(ctx, conn); err != nil {
			t.Fatal(err)
		}
		var name string
		if err := conn.QueryRow("select full_name from public.customers where email='existing@example.com'").Scan(&name); err != nil || name != "Existing customer" {
			t.Fatalf("lost existing customer: %s %v", name, err)
		}
		var count int
		if err := conn.QueryRow("select count(*) from information_schema.columns where table_schema='public' and table_name='barbers' and column_name in ('state','local_area')").Scan(&count); err != nil || count != 2 {
			t.Fatalf("missing upgraded fields=%d %v", count, err)
		}
	})
	t.Run("failed baseline leaves no partial schema or history", func(t *testing.T) {
		conn := migrationDatabase(t)
		if _, err := conn.Exec("create table public.customers(id text primary key)"); err != nil {
			t.Fatal(err)
		}
		if err := dbmigrate.Apply(ctx, conn); err == nil {
			t.Fatal("incompatible schema accepted")
		}
		var partial bool
		if err := conn.QueryRow("select to_regclass('public.barbers') is not null or to_regclass('degosalon_internal.schema_migrations') is not null").Scan(&partial); err != nil || partial {
			t.Fatalf("partial migration remains=%v %v", partial, err)
		}
	})
}

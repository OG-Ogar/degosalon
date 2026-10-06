package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"testing"

	"barber-booking/internal/mediastore"
)

func TestCommandRequiresServerConfiguration(t *testing.T) {
	t.Setenv("SUPABASE_URL", "")
	t.Setenv("SUPABASE_SECRET_KEY", "")
	t.Setenv("SUPABASE_SERVICE_ROLE_KEY", "")
	for _, args := range [][]string{nil, {"-create-bucket"}, {"unexpected"}} {
		if err := run(context.Background(), args, io.Discard); !errors.Is(err, mediastore.ErrConfiguration) {
			t.Fatalf("missing configuration accepted: %v", err)
		}
	}
	if err := run(context.Background(), []string{"-help"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatal("help not available")
	}
}

func TestLegacyCommandValidatesDatabaseBeforeProviderAccess(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SUPABASE_URL", "https://provider.invalid")
	t.Setenv("SUPABASE_SECRET_KEY", "sb_secret_test-only")
	if err := run(context.Background(), []string{"-migrate-legacy", "-create-bucket"}, io.Discard); !errors.Is(err, mediastore.ErrConfiguration) {
		t.Fatalf("database requirement not checked before provider: %v", err)
	}
}

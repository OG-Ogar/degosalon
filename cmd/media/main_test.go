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

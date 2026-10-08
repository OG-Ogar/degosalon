// Command audit exports bounded pages of private administrator audit events.
package main

import (
	"barber-booking/storage"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	before := flag.Int64("before", 0, "return events older than this ID")
	limit := flag.Int("limit", 100, "number of events (1-200)")
	flag.Parse()
	if flag.NArg() != 0 || os.Getenv("DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL and valid paging flags are required")
		os.Exit(1)
	}
	if err := run(context.Background(), *before, *limit); err != nil {
		fmt.Fprintln(os.Stderr, "audit export failed")
		os.Exit(1)
	}
}
func run(ctx context.Context, before int64, limit int) error {
	if err := storage.InitDatabaseContext(ctx); err != nil {
		return err
	}
	defer storage.CloseDatabase()
	events, err := storage.ListAdminAudit(ctx, before, limit)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(events)
}

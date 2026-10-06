// Command media verifies or provisions the application's dedicated media bucket.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"barber-booking/handlers"
	"barber-booking/internal/mediastore"
	"barber-booking/storage"
)

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("media", flag.ContinueOnError)
	flags.SetOutput(out)
	create := flags.Bool("create-bucket", false, "create the public application bucket only if missing")
	legacy := flags.Bool("migrate-legacy", false, "copy referenced local media and replace unchanged database references")
	root := flags.String("uploads-dir", "uploads", "directory containing the original upload subdirectories")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return mediastore.ErrConfiguration
	}
	if *legacy && os.Getenv("DATABASE_URL") == "" {
		return mediastore.ErrConfiguration
	}
	store, err := mediastore.FromEnvironment()
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.EnsureBucket(ctx, *create); err != nil {
		return err
	}
	if !*legacy {
		return nil
	}
	if err := storage.InitDatabase(); err != nil {
		return errors.New("media migration database is unavailable or schema is not current")
	}
	defer storage.CloseDatabase()
	if err := handlers.ImportLegacySiteImagesFromRoot(ctx, *root); err != nil {
		return handlers.ErrLegacyMediaMigration
	}
	report, err := handlers.MigrateLegacyMedia(ctx, *root, store)
	_, _ = fmt.Fprintf(out, "legacy media: copied=%d updated_records=%d unreferenced=%d failed=%d remaining_local=%t\n", report.Copied, report.UpdatedRecords, report.Unreferenced, report.Failed, report.Remaining)
	return err
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
	log.Print("media bucket configuration verified")
}

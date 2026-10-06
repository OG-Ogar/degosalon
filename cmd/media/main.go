// Command media verifies or provisions the application's dedicated media bucket.
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"os"

	"barber-booking/internal/mediastore"
)

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("media", flag.ContinueOnError)
	flags.SetOutput(out)
	create := flags.Bool("create-bucket", false, "create the public application bucket only if missing")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return mediastore.ErrConfiguration
	}
	store, err := mediastore.FromEnvironment()
	if err != nil {
		return err
	}
	defer store.Close()
	return store.EnsureBucket(ctx, *create)
}
func main() {
	if err := run(context.Background(), os.Args[1:], os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
	log.Print("media bucket configuration verified")
}

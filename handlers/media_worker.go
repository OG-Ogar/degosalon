package handlers

import (
	"context"
	"log"
	"time"

	"barber-booking/storage"
)

// RunMediaCleanup periodically reconciles durable intents. Multiple instances
// safely share the queue; ownership validation precedes every provider delete.
func RunMediaCleanup(ctx context.Context) {
	store, err := configuredMediaStore()
	if err != nil {
		log.Print("media cleanup unavailable: configure cloud storage")
		return
	}
	if store == nil {
		return
	}
	defer store.Close()
	for {
		found, err := storage.ProcessMediaCleanup(ctx, store.Delete)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Print("media reconciliation failed; retry remains scheduled")
		}
		if found && err == nil {
			continue
		}
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

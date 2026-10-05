package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"barber-booking/internal/dbmigrate"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required for migrations")
	}
	conn, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal("open database: invalid configuration")
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := dbmigrate.Apply(ctx, conn); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}
	if err := dbmigrate.Verify(ctx, conn); err != nil {
		log.Fatalf("database verification failed: %v", err)
	}
	log.Print("database migrations are current")
}

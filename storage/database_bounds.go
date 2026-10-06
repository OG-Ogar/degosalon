package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgconn/ctxwatch"
	"github.com/jackc/pgx/v5/stdlib"
)

const DatabaseOperationTimeout = 5 * time.Second

func databaseContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, DatabaseOperationTimeout)
}

type databasePoolConfig struct {
	open, idle         int
	lifetime, idleTime time.Duration
}

func poolConfig(getenv func(string) string) (databasePoolConfig, error) {
	config := databasePoolConfig{open: 12, idle: 6, lifetime: 30 * time.Minute, idleTime: 5 * time.Minute}
	number := func(name string, fallback, min, max int) (int, error) {
		value := getenv(name)
		if value == "" {
			return fallback, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < min || n > max {
			return 0, fmt.Errorf("%s is outside its supported range", name)
		}
		return n, nil
	}
	var err error
	if config.open, err = number("DB_MAX_OPEN_CONNS", config.open, 1, 100); err != nil {
		return config, err
	}
	if config.idle > config.open {
		config.idle = config.open
	}
	if config.idle, err = number("DB_MAX_IDLE_CONNS", config.idle, 0, config.open); err != nil {
		return config, err
	}
	duration := func(name string, fallback time.Duration) (time.Duration, error) {
		value := getenv(name)
		if value == "" {
			return fallback, nil
		}
		d, err := time.ParseDuration(value)
		if err != nil || d < time.Minute || d > 24*time.Hour {
			return 0, fmt.Errorf("%s must be between one minute and 24 hours", name)
		}
		return d, nil
	}
	if config.lifetime, err = duration("DB_CONN_MAX_LIFETIME", config.lifetime); err != nil {
		return config, err
	}
	if config.idleTime > config.lifetime {
		config.idleTime = config.lifetime
	}
	if config.idleTime, err = duration("DB_CONN_MAX_IDLE_TIME", config.idleTime); err != nil {
		return config, err
	}
	if config.idleTime > config.lifetime {
		return config, fmt.Errorf("DB_CONN_MAX_IDLE_TIME exceeds DB_CONN_MAX_LIFETIME")
	}
	return config, nil
}
func configurePool(conn *sql.DB) error {
	config, err := poolConfig(os.Getenv)
	if err != nil {
		return err
	}
	conn.SetMaxOpenConns(config.open)
	conn.SetMaxIdleConns(config.idle)
	conn.SetConnMaxLifetime(config.lifetime)
	conn.SetConnMaxIdleTime(config.idleTime)
	return nil
}

// openDatabase requests server cancellation when a context expires, retaining
// a short socket deadline as a fallback. Cancellation cannot prove that a
// commit whose acknowledgment was lost did not happen; callers retain their
// existing idempotency/reconciliation contracts.
func openDatabase(address string) (*sql.DB, error) {
	config, err := pgx.ParseConfig(address)
	if err != nil {
		return nil, errors.New("DATABASE_URL is invalid")
	}
	if config.ConnectTimeout == 0 || config.ConnectTimeout > DatabaseOperationTimeout {
		config.ConnectTimeout = DatabaseOperationTimeout
	}
	config.BuildContextWatcherHandler = func(conn *pgconn.PgConn) ctxwatch.Handler {
		return &pgconn.CancelRequestContextWatcherHandler{Conn: conn, CancelRequestDelay: 0, DeadlineDelay: 500 * time.Millisecond}
	}
	connection := sql.OpenDB(stdlib.GetConnector(*config))
	if err := configurePool(connection); err != nil {
		connection.Close()
		return nil, err
	}
	return connection, nil
}

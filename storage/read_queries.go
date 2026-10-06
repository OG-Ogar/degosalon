package storage

import (
	"context"
	"database/sql"
	"errors"
)

// queryRecords never substitutes memory or partial results for a failed query.
func queryRecords[T any](ctx context.Context, query string, scan func(*sql.Rows) (T, error), args ...any) ([]T, error) {
	ctx, cancel := databaseContext(ctx)
	defer cancel()

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]T, 0)
	for rows.Next() {
		record, err := scan(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func recordError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

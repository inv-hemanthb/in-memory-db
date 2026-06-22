package db

import (
	"database/sql"
	"time"
)

type Item struct {
	ID        int64
	Key       string
	Value     string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type rowScanner interface {
	Scan(dest ...any) error
}

const itemColumns = `id, key, value, created_at, updated_at, deleted_at`

func scanItem(row rowScanner) (Item, error) {
	var item Item
	var deletedAt sql.NullTime

	err := row.Scan(
		&item.ID,
		&item.Key,
		&item.Value,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return Item{}, err
	}

	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}

	return item, nil
}

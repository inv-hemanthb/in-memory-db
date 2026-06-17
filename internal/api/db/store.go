package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, key, value string) (Item, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO items (key, value)
		VALUES ($1, $2)
		RETURNING `+itemColumns,
		key, value,
	)

	item, err := scanItem(row)
	if err != nil {
		return Item{}, mapInsertError(err)
	}

	return item, nil
}

func (s *Store) GetByID(ctx context.Context, id int64) (Item, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+itemColumns+`
		FROM items
		WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)

	return scanItemOrNotFound(row)
}

func (s *Store) GetByKey(ctx context.Context, key string) (Item, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+itemColumns+`
		FROM items
		WHERE key = $1 AND deleted_at IS NULL`,
		key,
	)

	return scanItemOrNotFound(row)
}

func (s *Store) GetByIDAndKey(ctx context.Context, id int64, key string) (Item, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+itemColumns+`
		FROM items
		WHERE id = $1 AND key = $2 AND deleted_at IS NULL`,
		id, key,
	)

	return scanItemOrNotFound(row)
}

func (s *Store) Update(ctx context.Context, id int64, value string) (Item, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE items
		SET value = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+itemColumns,
		id, value,
	)

	return scanItemOrNotFound(row)
}

func (s *Store) SoftDelete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE items
		SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL`,
		id,
	)
	if err != nil {
		return err
	}

	return rowsAffectedOrNotFound(result)
}

func (s *Store) HardDelete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM items
		WHERE id = $1`,
		id,
	)
	if err != nil {
		return err
	}

	return rowsAffectedOrNotFound(result)
}

func scanItemOrNotFound(row *sql.Row) (Item, error) {
	item, err := scanItem(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Item{}, ErrNotFound
		}
		return Item{}, err
	}
	return item, nil
}

func rowsAffectedOrNotFound(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func mapInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicateKey
	}
	return fmt.Errorf("insert item: %w", err)
}

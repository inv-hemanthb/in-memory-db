package db

import "errors"

var (
	ErrNotFound     = errors.New("item not found")
	ErrDuplicateKey = errors.New("duplicate key")
)

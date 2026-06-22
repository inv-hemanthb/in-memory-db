package kvclient

import "errors"

var (
	ErrNotFound   = errors.New("key not found")
	ErrServerBusy = errors.New("server busy")
)

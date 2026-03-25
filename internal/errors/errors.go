package errors

import "errors"

var (
	ErrAccountNotFound  = errors.New("account not found")
	ErrUnsafeConfigPath = errors.New("unsafe config path")
)

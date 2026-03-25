package errors

import "errors"

var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrRegistryNotFound   = errors.New("registry not found")
	ErrInvalidAccountName = errors.New("invalid account name")
	ErrCredentialMissing  = errors.New("credentials missing")
	ErrCredentialExpired  = errors.New("credentials expired")
	ErrUnsafeConfigPath   = errors.New("unsafe config path")
)

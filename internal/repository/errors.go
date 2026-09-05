package repository

import "errors"

// ошибки уровня repository.
var (
	ErrOrderExists     = errors.New("order already exists")
	ErrUserExists      = errors.New("user already exists")
	ErrBalanceNotFound = errors.New("balance not found")
	ErrNotFound        = errors.New("not found")
)

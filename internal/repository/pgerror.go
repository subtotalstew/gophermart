package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Коды ошибок PostgreSQL, которые нас интересуют.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// mapPgError превращает низкоуровневую ошибку pgx/pgconn в наши sentinel-ошибки.
// Возвращает исходную ошибку, если это не распознанный случай.
func mapPgError(err error, uniqueErr error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			if uniqueErr != nil {
				return uniqueErr
			}
			return ErrOrderExists
		}
	}
	return err
}

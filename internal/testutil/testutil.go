package testutil

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateTestUUID создает тестовый UUID из строки
func CreateTestUUID(id string) pgtype.UUID {
	var uuid pgtype.UUID
	uuid.Scan(id)
	return uuid
}

// CreateTestUser создает тестового пользователя
func CreateTestUser(id, login string) *pgtype.UUID {
	var uuid pgtype.UUID
	uuid.Scan(id)
	return &uuid
}

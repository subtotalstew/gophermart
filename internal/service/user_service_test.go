package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/subtotalstew/gophermart/internal/testutil"
)

func TestUserService_Register(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := NewUserService(mockUserRepo)

	tests := []struct {
		name        string
		login       string
		password    string
		shouldError bool
		errorType   error
	}{
		{
			name:        "successful registration",
			login:       "testuser",
			password:    "test123",
			shouldError: false,
		},
		{
			name:        "duplicate user",
			login:       "testuser",
			password:    "test456",
			shouldError: true,
			errorType:   ErrUserExists,
		},
		{
			name:        "empty login",
			login:       "",
			password:    "test123",
			shouldError: false, // Сервис не валидирует пустые логины, это делает хендлер
		},
		{
			name:        "empty password",
			login:       "testuser2",
			password:    "",
			shouldError: false, // Сервис не валидирует пустые пароли
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := userService.Register(context.Background(), tt.login, tt.password)

			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.Equal(t, tt.errorType, err)
				}
				assert.Nil(t, user)
			} else {
				if tt.login != "" && tt.password != "" {
					require.NoError(t, err)
					assert.NotNil(t, user)
					assert.Equal(t, tt.login, user.Login)
					assert.NotEmpty(t, user.PasswordHash)
					assert.NotZero(t, user.CreatedAt)
				}
			}
		})
	}
}

func TestUserService_Login(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := NewUserService(mockUserRepo)

	// Регистрируем пользователя
	login := "testuser"
	password := "test123"
	_, err := userService.Register(context.Background(), login, password)
	require.NoError(t, err)

	tests := []struct {
		name        string
		login       string
		password    string
		shouldError bool
		errorType   error
	}{
		{
			name:        "successful login",
			login:       login,
			password:    password,
			shouldError: false,
		},
		{
			name:        "wrong password",
			login:       login,
			password:    "wrongpass",
			shouldError: true,
			errorType:   ErrInvalidLogin,
		},
		{
			name:        "non-existent user",
			login:       "nonexistent",
			password:    "test123",
			shouldError: true,
			errorType:   ErrInvalidLogin,
		},
		{
			name:        "empty login",
			login:       "",
			password:    "test123",
			shouldError: true,
			errorType:   ErrInvalidLogin,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := userService.Login(context.Background(), tt.login, tt.password)

			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.Equal(t, tt.errorType, err)
				}
				assert.Nil(t, user)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, tt.login, user.Login)
			}
		})
	}
}

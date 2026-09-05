package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_GenerateToken(t *testing.T) {
	secret := "test-secret"
	auth := NewAuthMiddleware(secret)

	token, err := auth.GenerateToken("test-user-id")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Проверяем, что токен валидный
	userID, err := auth.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "test-user-id", userID)
}

func TestAuthMiddleware_ValidateToken_Invalid(t *testing.T) {
	secret := "test-secret"
	auth := NewAuthMiddleware(secret)

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "invalid token",
			token: "invalid.token.here",
		},
		{
			name:  "expired token",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, err := auth.ValidateToken(tt.token)
			assert.Error(t, err)
			assert.Empty(t, userID)
		})
	}
}

func TestAuthMiddleware_ValidateToken_WrongSecret(t *testing.T) {
	auth1 := NewAuthMiddleware("secret1")
	auth2 := NewAuthMiddleware("secret2")

	token, err := auth1.GenerateToken("test-user-id")
	require.NoError(t, err)

	// Пытаемся валидировать токен с другим секретом
	userID, err := auth2.ValidateToken(token)
	assert.Error(t, err)
	assert.Empty(t, userID)
}

func TestAuthMiddleware_RequireAuth_Success(t *testing.T) {
	secret := "test-secret"
	auth := NewAuthMiddleware(secret)

	token, err := auth.GenerateToken("test-user-id")
	require.NoError(t, err)

	// Создаем тестовый хендлер
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			t.Error("UserID not found in context")
			return
		}
		if userID != "test-user-id" {
			t.Errorf("Expected userID = test-user-id, got %s", userID)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := auth.RequireAuth(next)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_RequireAuth_NoToken(t *testing.T) {
	secret := "test-secret"
	auth := NewAuthMiddleware(secret)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := auth.RequireAuth(next)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_RequireAuth_InvalidToken(t *testing.T) {
	secret := "test-secret"
	auth := NewAuthMiddleware(secret)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := auth.RequireAuth(next)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_RequireAuth_WrongFormat(t *testing.T) {
	secret := "test-secret"
	auth := NewAuthMiddleware(secret)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := auth.RequireAuth(next)

	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "no Bearer prefix",
			header: "token123",
		},
		{
			name:   "empty Bearer",
			header: "Bearer ",
		},
		{
			name:   "invalid format",
			header: "Bearer token extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Authorization", tt.header)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
		ok       bool
	}{
		{
			name:     "valid user ID",
			ctx:      context.WithValue(context.Background(), UserIDKey, "user-123"),
			expected: "user-123",
			ok:       true,
		},
		{
			name:     "empty context",
			ctx:      context.Background(),
			expected: "",
			ok:       false,
		},
		{
			name:     "wrong type",
			ctx:      context.WithValue(context.Background(), UserIDKey, 123),
			expected: "",
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, ok := GetUserIDFromContext(tt.ctx)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.expected, userID)
		})
	}
}

func TestGetUserUUIDFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "00000000-0000-0000-0000-000000000001")

	uuid, ok := GetUserUUIDFromContext(ctx)
	assert.True(t, ok)
	assert.NotZero(t, uuid)
	assert.Equal(t, "00000000-0000-0000-0000-000000000001", uuid.String())

	// Пустой контекст
	uuid, ok = GetUserUUIDFromContext(context.Background())
	assert.False(t, ok)
	assert.Zero(t, uuid)
}

// Бенчмарки
func BenchmarkAuthMiddleware_GenerateToken(b *testing.B) {
	auth := NewAuthMiddleware("test-secret")
	for i := 0; i < b.N; i++ {
		auth.GenerateToken("test-user-id")
	}
}

func BenchmarkAuthMiddleware_ValidateToken(b *testing.B) {
	auth := NewAuthMiddleware("test-secret")
	token, _ := auth.GenerateToken("test-user-id")
	for i := 0; i < b.N; i++ {
		auth.ValidateToken(token)
	}
}

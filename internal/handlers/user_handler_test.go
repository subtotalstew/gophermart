package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/subtotalstew/gophermart/internal/middleware"
	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/service"
	"github.com/subtotalstew/gophermart/internal/testutil"
)

// ============================================
// Тесты для регистрации пользователя
// ============================================

func TestUserHandler_Register(t *testing.T) {
	tests := []struct {
		name           string
		request        models.RegisterRequest
		expectedStatus int
		expectedError  string
		setup          func(*testutil.MockUserRepository)
	}{
		{
			name: "successful registration",
			request: models.RegisterRequest{
				Login:    "newuser",
				Password: "test123",
			},
			expectedStatus: http.StatusOK,
			expectedError:  "",
			setup:          nil,
		},
		{
			name: "duplicate user - should conflict",
			request: models.RegisterRequest{
				Login:    "existing_user",
				Password: "test456",
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "Login already taken\n",
			setup: func(m *testutil.MockUserRepository) {
				user := &models.User{Login: "existing_user"}
				m.Create(context.Background(), user)
			},
		},
		{
			name: "empty login - should error",
			request: models.RegisterRequest{
				Login:    "",
				Password: "test123",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Login and password are required\n",
			setup:          nil,
		},
		{
			name: "empty password - should error",
			request: models.RegisterRequest{
				Login:    "user_without_password",
				Password: "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Login and password are required\n",
			setup:          nil,
		},
		{
			name: "empty login and password - should error",
			request: models.RegisterRequest{
				Login:    "",
				Password: "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Login and password are required\n",
			setup:          nil,
		},
		{
			name: "valid registration with special chars in login",
			request: models.RegisterRequest{
				Login:    "user_123",
				Password: "test@123",
			},
			expectedStatus: http.StatusOK,
			expectedError:  "",
			setup:          nil,
		},
		{
			name: "valid registration with long login",
			request: models.RegisterRequest{
				Login:    "verylongusername1234567890",
				Password: "test123",
			},
			expectedStatus: http.StatusOK,
			expectedError:  "",
			setup:          nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем новый мок для каждого теста
			mockUserRepo := testutil.NewMockUserRepository()
			userService := service.NewUserService(mockUserRepo)
			authMiddleware := middleware.NewAuthMiddleware("test-secret")
			userHandler := NewUserHandler(userService, authMiddleware)

			// Настраиваем мок если нужно
			if tt.setup != nil {
				tt.setup(mockUserRepo)
			}

			body, err := json.Marshal(tt.request)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/user/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			userHandler.Register(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Status code mismatch for test: %s", tt.name)

			if tt.expectedError != "" {
				assert.Equal(t, tt.expectedError, w.Body.String())
			}

			if tt.expectedStatus == http.StatusOK {
				token := w.Header().Get("Authorization")
				assert.NotEmpty(t, token, "Authorization header should not be empty")
				assert.Contains(t, token, "Bearer ", "Authorization header should contain 'Bearer '")
			}
		})
	}
}

// ============================================
// Тесты для регистрации с невалидным JSON
// ============================================

func TestUserHandler_Register_InvalidJSON(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := service.NewUserService(mockUserRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	userHandler := NewUserHandler(userService, authMiddleware)

	// Некорректный JSON (незакрытая кавычка)
	req := httptest.NewRequest("POST", "/api/user/register", bytes.NewBufferString(`{"login": "test", "password": "test123"`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	userHandler.Register(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request format")
}

// ============================================
// Тесты для аутентификации пользователя
// ============================================

func TestUserHandler_Login(t *testing.T) {
	tests := []struct {
		name           string
		request        models.LoginRequest
		expectedStatus int
		expectedError  string
		setup          func(*testutil.MockUserRepository)
	}{
		{
			name: "successful login",
			request: models.LoginRequest{
				Login:    "testuser",
				Password: "test123",
			},
			expectedStatus: http.StatusOK,
			expectedError:  "",
			setup: func(m *testutil.MockUserRepository) {
				userService := service.NewUserService(m)
				userService.Register(context.Background(), "testuser", "test123")
			},
		},
		{
			name: "wrong password - should error",
			request: models.LoginRequest{
				Login:    "testuser",
				Password: "wrongpass",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid login or password\n",
			setup: func(m *testutil.MockUserRepository) {
				userService := service.NewUserService(m)
				userService.Register(context.Background(), "testuser", "test123")
			},
		},
		{
			name: "non-existent user - should error",
			request: models.LoginRequest{
				Login:    "nonexistent",
				Password: "test123",
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid login or password\n",
			setup:          nil,
		},
		{
			name: "empty login - should error",
			request: models.LoginRequest{
				Login:    "",
				Password: "test123",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Login and password are required\n",
			setup:          nil,
		},
		{
			name: "empty password - should error",
			request: models.LoginRequest{
				Login:    "testuser",
				Password: "",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Login and password are required\n",
			setup: func(m *testutil.MockUserRepository) {
				userService := service.NewUserService(m)
				userService.Register(context.Background(), "testuser", "test123")
			},
		},
		{
			name: "login with special chars",
			request: models.LoginRequest{
				Login:    "user_123",
				Password: "test@123",
			},
			expectedStatus: http.StatusOK,
			expectedError:  "",
			setup: func(m *testutil.MockUserRepository) {
				userService := service.NewUserService(m)
				userService.Register(context.Background(), "user_123", "test@123")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем новый мок для каждого теста
			mockUserRepo := testutil.NewMockUserRepository()
			userService := service.NewUserService(mockUserRepo)
			authMiddleware := middleware.NewAuthMiddleware("test-secret")
			userHandler := NewUserHandler(userService, authMiddleware)

			// Настраиваем мок если нужно
			if tt.setup != nil {
				tt.setup(mockUserRepo)
			}

			body, err := json.Marshal(tt.request)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/user/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			userHandler.Login(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Status code mismatch for test: %s", tt.name)

			if tt.expectedError != "" {
				assert.Equal(t, tt.expectedError, w.Body.String())
			}

			if tt.expectedStatus == http.StatusOK {
				token := w.Header().Get("Authorization")
				assert.NotEmpty(t, token, "Authorization header should not be empty")
				assert.Contains(t, token, "Bearer ", "Authorization header should contain 'Bearer '")
			}
		})
	}
}

// ============================================
// Тесты для логина с невалидным JSON
// ============================================

func TestUserHandler_Login_InvalidJSON(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := service.NewUserService(mockUserRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	userHandler := NewUserHandler(userService, authMiddleware)

	// Некорректный JSON
	req := httptest.NewRequest("POST", "/api/user/login", bytes.NewBufferString(`{"login": "test", "password": "test123"`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	userHandler.Login(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request format")
}

// ============================================
// Тест: автоматическая аутентификация после регистрации
// ============================================

func TestUserHandler_Register_AutoAuth(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := service.NewUserService(mockUserRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	userHandler := NewUserHandler(userService, authMiddleware)

	// Регистрируем пользователя
	registerReq := models.RegisterRequest{
		Login:    "autouser",
		Password: "test123",
	}
	body, err := json.Marshal(registerReq)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/user/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	userHandler.Register(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем токен
	token := w.Header().Get("Authorization")
	assert.NotEmpty(t, token)
	assert.Contains(t, token, "Bearer ")

	// Проверяем, что токен валидный
	tokenPart := token[7:] // Убираем "Bearer "
	userID, err := authMiddleware.ValidateToken(tokenPart)
	require.NoError(t, err)
	assert.NotEmpty(t, userID)
}

// ============================================
// Тест: регистрация с уже существующим логином (в моке)
// ============================================

func TestUserHandler_Register_ExistingLoginInMock(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := service.NewUserService(mockUserRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	userHandler := NewUserHandler(userService, authMiddleware)

	// Создаем пользователя напрямую в моке (минуя сервис)
	user := &models.User{
		Login: "existing_user",
	}
	err := mockUserRepo.Create(context.Background(), user)
	require.NoError(t, err)

	// Пытаемся зарегистрироваться с тем же логином
	req := models.RegisterRequest{
		Login:    "existing_user",
		Password: "test123",
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/register", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	userHandler.Register(w, httpReq)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "Login already taken\n", w.Body.String())
}

// ============================================
// Тест: несколько успешных регистраций подряд
// ============================================

func TestUserHandler_Register_MultipleUsers(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := service.NewUserService(mockUserRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	userHandler := NewUserHandler(userService, authMiddleware)

	users := []string{"user1", "user2", "user3", "user4", "user5"}

	for _, login := range users {
		req := models.RegisterRequest{
			Login:    login,
			Password: "test123",
		}
		body, err := json.Marshal(req)
		require.NoError(t, err)

		httpReq := httptest.NewRequest("POST", "/api/user/register", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		userHandler.Register(w, httpReq)

		assert.Equal(t, http.StatusOK, w.Code, "Registration should succeed for user: %s", login)

		token := w.Header().Get("Authorization")
		assert.NotEmpty(t, token, "Authorization header should not be empty for user: %s", login)
		assert.Contains(t, token, "Bearer ", "Authorization header should contain 'Bearer ' for user: %s", login)
	}

	// Проверяем, что все пользователи созданы
	for _, login := range users {
		exists, err := mockUserRepo.UserExists(context.Background(), login)
		require.NoError(t, err)
		assert.True(t, exists, "User %s should exist", login)
	}
}

// ============================================
// Тест: регистрация с одинаковыми паролями для разных пользователей
// ============================================

func TestUserHandler_Register_SamePassword(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := service.NewUserService(mockUserRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	userHandler := NewUserHandler(userService, authMiddleware)

	password := "samepassword123"

	users := []string{"user_a", "user_b", "user_c"}

	for _, login := range users {
		req := models.RegisterRequest{
			Login:    login,
			Password: password,
		}
		body, err := json.Marshal(req)
		require.NoError(t, err)

		httpReq := httptest.NewRequest("POST", "/api/user/register", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		userHandler.Register(w, httpReq)

		assert.Equal(t, http.StatusOK, w.Code, "Registration should succeed for user: %s", login)
	}

	// Проверяем, что все пользователи созданы
	for _, login := range users {
		exists, err := mockUserRepo.UserExists(context.Background(), login)
		require.NoError(t, err)
		assert.True(t, exists, "User %s should exist", login)
	}

	// Проверяем, что пароли хешированы (не хранятся в открытом виде)
	user, err := mockUserRepo.FindByLogin(context.Background(), users[0])
	require.NoError(t, err)
	assert.NotEmpty(t, user.PasswordHash)
	assert.NotEqual(t, password, user.PasswordHash)
}

// ============================================
// Тесты для финансовых эндпоинтов (заглушки)
// ============================================

func TestUserHandler_FinancialEndpoints_NotImplemented(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := service.NewUserService(mockUserRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	userHandler := NewUserHandler(userService, authMiddleware)

	tests := []struct {
		name    string
		method  string
		path    string
		handler func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name:    "GetBalance",
			method:  "GET",
			path:    "/api/user/balance",
			handler: userHandler.GetBalance,
		},
		{
			name:    "Withdraw",
			method:  "POST",
			path:    "/api/user/balance/withdraw",
			handler: userHandler.Withdraw,
		},
		{
			name:    "GetWithdrawals",
			method:  "GET",
			path:    "/api/user/withdrawals",
			handler: userHandler.GetWithdrawals,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			tt.handler(w, req)

			assert.Equal(t, http.StatusNotImplemented, w.Code)
			assert.Contains(t, w.Body.String(), "Not implemented yet")
		})
	}
}

// ============================================
// Тест: проверка, что токен содержит правильный user ID
// ============================================

func TestUserHandler_Register_TokenContainsUserID(t *testing.T) {
	mockUserRepo := testutil.NewMockUserRepository()
	userService := service.NewUserService(mockUserRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	userHandler := NewUserHandler(userService, authMiddleware)

	login := "token_test_user"

	req := models.RegisterRequest{
		Login:    login,
		Password: "test123",
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/register", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	userHandler.Register(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)

	// Получаем токен
	token := w.Header().Get("Authorization")
	assert.NotEmpty(t, token)

	// Извлекаем user ID из токена
	tokenPart := token[7:] // Убираем "Bearer "
	userID, err := authMiddleware.ValidateToken(tokenPart)
	require.NoError(t, err)
	assert.NotEmpty(t, userID)

	// Проверяем, что пользователь с таким ID существует
	user, err := mockUserRepo.FindByLogin(context.Background(), login)
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, userID, user.ID.String())
}

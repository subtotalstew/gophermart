package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/subtotalstew/gophermart/internal/middleware"
	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/service"
	"github.com/subtotalstew/gophermart/internal/testutil"
)

func setupOrderHandlerTest(t *testing.T) (*OrderHandler, *testutil.MockOrderRepository, *testutil.MockUserRepository, *middleware.AuthMiddleware, pgtype.UUID) {
	mockOrderRepo := testutil.NewMockOrderRepository()
	mockUserRepo := testutil.NewMockUserRepository()

	orderService := service.NewOrderService(mockOrderRepo, mockUserRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	orderHandler := NewOrderHandler(orderService)

	// Создаем тестового пользователя
	var userID pgtype.UUID
	userID.Scan("00000000-0000-0000-0000-000000000001")

	user := &models.User{
		ID:    userID,
		Login: "testuser",
	}
	mockUserRepo.Create(context.Background(), user)

	return orderHandler, mockOrderRepo, mockUserRepo, authMiddleware, userID
}

func TestOrderHandler_UploadOrder(t *testing.T) {
	orderHandler, mockOrderRepo, _, authMiddleware, userID := setupOrderHandlerTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	tests := []struct {
		name           string
		contentType    string
		body           string
		expectedStatus int
	}{
		{
			name:           "valid order",
			contentType:    "text/plain",
			body:           "12345678903",
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "valid order with spaces - should normalize and accept",
			contentType:    "text/plain",
			body:           "1234 5678 903",
			expectedStatus: http.StatusAccepted, // Должен быть 202, т.к. после нормализации это новый заказ
		},
		{
			name:           "valid order with dashes",
			contentType:    "text/plain",
			body:           "1234-5678-903",
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "invalid order number",
			contentType:    "text/plain",
			body:           "12345",
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "empty body",
			contentType:    "text/plain",
			body:           "",
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "wrong content type",
			contentType:    "application/json",
			body:           "12345678903",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "order with letters",
			contentType:    "text/plain",
			body:           "1234abcd",
			expectedStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Очищаем мок перед каждым тестом
			mockOrderRepo.Clear()

			req := httptest.NewRequest("POST", "/api/user/orders", bytes.NewBufferString(tt.body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", tt.contentType)

			// Добавляем userID в контекст
			ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			orderHandler.UploadOrder(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestOrderHandler_UploadOrder_Duplicate(t *testing.T) {
	orderHandler, mockOrderRepo, _, authMiddleware, userID := setupOrderHandlerTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	// Создаем заказ в моке
	order := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusNew,
	}
	mockOrderRepo.AddOrder(order)

	// Пытаемся загрузить существующий заказ
	req := httptest.NewRequest("POST", "/api/user/orders", bytes.NewBufferString("12345678903"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "text/plain")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	orderHandler.UploadOrder(w, req)

	// Должен вернуть 200 OK (заказ уже загружен этим пользователем)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrderHandler_UploadOrder_DuplicateWithSpaces(t *testing.T) {
	orderHandler, mockOrderRepo, _, authMiddleware, userID := setupOrderHandlerTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	// Создаем заказ в моке (без пробелов)
	order := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusNew,
	}
	mockOrderRepo.AddOrder(order)

	// Пытаемся загрузить существующий заказ с пробелами
	req := httptest.NewRequest("POST", "/api/user/orders", bytes.NewBufferString("1234 5678 903"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "text/plain")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	orderHandler.UploadOrder(w, req)

	// Должен вернуть 200 OK (заказ уже существует, нормализация должна найти его)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOrderHandler_UploadOrder_Unauthorized(t *testing.T) {
	orderHandler, _, _, _, _ := setupOrderHandlerTest(t)

	req := httptest.NewRequest("POST", "/api/user/orders", bytes.NewBufferString("12345678903"))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	orderHandler.UploadOrder(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOrderHandler_GetOrders(t *testing.T) {
	orderHandler, mockOrderRepo, _, authMiddleware, userID := setupOrderHandlerTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	// Добавляем заказы в мок
	numbers := []string{"12345678903", "5555555555554444", "4111111111111111"}
	for _, num := range numbers {
		order := &models.Order{
			Number: num,
			UserID: userID,
			Status: models.OrderStatusNew,
		}
		mockOrderRepo.AddOrder(order)
	}

	// Получаем заказы
	req := httptest.NewRequest("GET", "/api/user/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	orderHandler.GetOrders(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.OrderResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 3)
}

func TestOrderHandler_GetOrders_NoContent(t *testing.T) {
	orderHandler, _, _, authMiddleware, userID := setupOrderHandlerTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/user/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	orderHandler.GetOrders(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestOrderHandler_GetOrders_Unauthorized(t *testing.T) {
	orderHandler, _, _, _, _ := setupOrderHandlerTest(t)

	req := httptest.NewRequest("GET", "/api/user/orders", nil)

	w := httptest.NewRecorder()
	orderHandler.GetOrders(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOrderHandler_GetOrders_WithDifferentStatuses(t *testing.T) {
	orderHandler, mockOrderRepo, _, authMiddleware, userID := setupOrderHandlerTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	// Добавляем заказы с разными статусами
	orders := []struct {
		number string
		status string
	}{
		{"12345678903", models.OrderStatusNew},
		{"5555555555554444", models.OrderStatusProcessing},
		{"4111111111111111", models.OrderStatusProcessed},
		{"49927398716", models.OrderStatusInvalid},
	}

	for _, o := range orders {
		order := &models.Order{
			Number: o.number,
			UserID: userID,
			Status: o.status,
		}
		mockOrderRepo.AddOrder(order)
	}

	req := httptest.NewRequest("GET", "/api/user/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	orderHandler.GetOrders(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.OrderResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Len(t, response, 4)

	// Проверяем статусы
	statusMap := make(map[string]int)
	for _, order := range response {
		statusMap[order.Status]++
	}
	assert.Equal(t, 1, statusMap[models.OrderStatusNew])
	assert.Equal(t, 1, statusMap[models.OrderStatusProcessing])
	assert.Equal(t, 1, statusMap[models.OrderStatusProcessed])
	assert.Equal(t, 1, statusMap[models.OrderStatusInvalid])
}

// Тест на проверку, что разные пользователи не могут загрузить один заказ
func TestOrderHandler_UploadOrder_Conflict(t *testing.T) {
	orderHandler, mockOrderRepo, _, authMiddleware, userID := setupOrderHandlerTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	// Создаем другого пользователя
	var otherUserID pgtype.UUID
	otherUserID.Scan("00000000-0000-0000-0000-000000000002")

	// Добавляем заказ от другого пользователя
	order := &models.Order{
		Number: "12345678903",
		UserID: otherUserID,
		Status: models.OrderStatusNew,
	}
	mockOrderRepo.AddOrder(order)

	// Пытаемся загрузить заказ от другого пользователя
	req := httptest.NewRequest("POST", "/api/user/orders", bytes.NewBufferString("12345678903"))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "text/plain")
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	orderHandler.UploadOrder(w, req)

	// Должен вернуть 409 Conflict
	assert.Equal(t, http.StatusConflict, w.Code)
}

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

// ============================================
// Вспомогательная функция для настройки тестов
// ============================================

func setupBalanceTest(t *testing.T) (*BalanceHandler, *testutil.MockBalanceRepository, *testutil.MockWithdrawalRepository, *testutil.MockOrderRepository, *middleware.AuthMiddleware, pgtype.UUID) {
	mockUserRepo := testutil.NewMockUserRepository()
	mockOrderRepo := testutil.NewMockOrderRepository()
	mockBalanceRepo := testutil.NewMockBalanceRepository()
	mockWithdrawalRepo := testutil.NewMockWithdrawalRepository()

	balanceService := service.NewBalanceService(mockBalanceRepo, mockWithdrawalRepo, mockOrderRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")

	balanceHandler := NewBalanceHandler(balanceService, authMiddleware)

	// Создаем тестового пользователя
	var userID pgtype.UUID
	userID.Scan("00000000-0000-0000-0000-000000000001")

	user := &models.User{
		ID:    userID,
		Login: "testuser",
	}
	mockUserRepo.Create(context.Background(), user)

	// Создаем баланс для пользователя
	mockBalanceRepo.Create(context.Background(), userID)

	return balanceHandler, mockBalanceRepo, mockWithdrawalRepo, mockOrderRepo, authMiddleware, userID
}

// ============================================
// Тесты для GET /api/user/balance
// ============================================

func TestBalanceHandler_GetBalance_Success(t *testing.T) {
	balanceHandler, mockBalanceRepo, _, _, authMiddleware, userID := setupBalanceTest(t)

	// Добавляем баланс
	mockBalanceRepo.AddBalance(userID, 500.5)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/user/balance", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.GetBalance(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.BalanceResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 500.5, response.Current)
	assert.Equal(t, 0.0, response.Withdrawn)
}

func TestBalanceHandler_GetBalance_WithWithdrawals(t *testing.T) {
	balanceHandler, mockBalanceRepo, mockWithdrawalRepo, _, authMiddleware, userID := setupBalanceTest(t)

	// Добавляем баланс
	mockBalanceRepo.AddBalance(userID, 1000.0)

	// Создаем списание
	withdrawal := &models.Withdrawal{
		UserID:      userID,
		OrderNumber: "12345678903",
		Sum:         300.0,
	}
	mockWithdrawalRepo.Create(context.Background(), withdrawal)

	// Обновляем баланс (списание)
	mockBalanceRepo.Withdraw(context.Background(), userID, 300.0)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/user/balance", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.GetBalance(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.BalanceResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 700.0, response.Current)
	assert.Equal(t, 300.0, response.Withdrawn)
}

func TestBalanceHandler_GetBalance_Unauthorized(t *testing.T) {
	balanceHandler, _, _, _, _, _ := setupBalanceTest(t)

	req := httptest.NewRequest("GET", "/api/user/balance", nil)
	w := httptest.NewRecorder()

	balanceHandler.GetBalance(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Упрощенный тест для проверки создания баланса
func TestBalanceHandler_GetBalance_CreatesNewBalance(t *testing.T) {
	// Создаем свежие моки
	mockUserRepo := testutil.NewMockUserRepository()
	mockOrderRepo := testutil.NewMockOrderRepository()
	mockBalanceRepo := testutil.NewMockBalanceRepository()
	mockWithdrawalRepo := testutil.NewMockWithdrawalRepository()

	balanceService := service.NewBalanceService(mockBalanceRepo, mockWithdrawalRepo, mockOrderRepo)
	authMiddleware := middleware.NewAuthMiddleware("test-secret")
	balanceHandler := NewBalanceHandler(balanceService, authMiddleware)

	// Создаем тестового пользователя
	var userID pgtype.UUID
	userID.Scan("00000000-0000-0000-0000-000000000002")

	user := &models.User{
		ID:    userID,
		Login: "testuser2",
	}
	mockUserRepo.Create(context.Background(), user)

	// НЕ создаем баланс заранее - он должен создаться автоматически

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/user/balance", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.GetBalance(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.BalanceResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, 0.0, response.Current)
	assert.Equal(t, 0.0, response.Withdrawn)

	// Проверяем, что баланс был создан
	balance, err := mockBalanceRepo.GetByUserID(context.Background(), userID)
	require.NoError(t, err)
	assert.NotNil(t, balance)
	assert.Equal(t, 0.0, balance.Current)
}

// ============================================
// Тесты для POST /api/user/balance/withdraw
// ============================================

func TestBalanceHandler_Withdraw_Success(t *testing.T) {
	balanceHandler, mockBalanceRepo, mockWithdrawalRepo, _, authMiddleware, userID := setupBalanceTest(t)

	// Добавляем баланс
	mockBalanceRepo.AddBalance(userID, 1000.0)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := models.WithdrawRequest{
		Order: "12345678903",
		Sum:   500.0,
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(httpReq.Context(), middleware.UserIDKey, userID.String())
	httpReq = httpReq.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.Withdraw(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем, что баланс обновился
	balance, err := mockBalanceRepo.GetByUserID(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, 500.0, balance.Current)
	assert.Equal(t, 500.0, balance.Withdrawn)

	// Проверяем, что запись о списании создалась
	withdrawals, err := mockWithdrawalRepo.GetByUserID(context.Background(), userID)
	require.NoError(t, err)
	assert.Len(t, withdrawals, 1)
	assert.Equal(t, "12345678903", withdrawals[0].OrderNumber)
	assert.Equal(t, 500.0, withdrawals[0].Sum)
}

func TestBalanceHandler_Withdraw_InsufficientFunds(t *testing.T) {
	balanceHandler, _, _, _, authMiddleware, userID := setupBalanceTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := models.WithdrawRequest{
		Order: "12345678903",
		Sum:   500.0, // Баланс 0
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(httpReq.Context(), middleware.UserIDKey, userID.String())
	httpReq = httpReq.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.Withdraw(w, httpReq)

	assert.Equal(t, http.StatusPaymentRequired, w.Code)
	assert.Contains(t, w.Body.String(), "Insufficient funds")
}

func TestBalanceHandler_Withdraw_InvalidOrder(t *testing.T) {
	balanceHandler, _, _, _, authMiddleware, userID := setupBalanceTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := models.WithdrawRequest{
		Order: "invalid",
		Sum:   500.0,
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(httpReq.Context(), middleware.UserIDKey, userID.String())
	httpReq = httpReq.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.Withdraw(w, httpReq)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid order number")
}

func TestBalanceHandler_Withdraw_EmptyOrder(t *testing.T) {
	balanceHandler, _, _, _, authMiddleware, userID := setupBalanceTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := models.WithdrawRequest{
		Order: "",
		Sum:   500.0,
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(httpReq.Context(), middleware.UserIDKey, userID.String())
	httpReq = httpReq.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.Withdraw(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Order number is required")
}

func TestBalanceHandler_Withdraw_ZeroSum(t *testing.T) {
	balanceHandler, _, _, _, authMiddleware, userID := setupBalanceTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := models.WithdrawRequest{
		Order: "12345678903",
		Sum:   0,
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(httpReq.Context(), middleware.UserIDKey, userID.String())
	httpReq = httpReq.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.Withdraw(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Withdrawal sum must be positive")
}

func TestBalanceHandler_Withdraw_Unauthorized(t *testing.T) {
	balanceHandler, _, _, _, _, _ := setupBalanceTest(t)

	req := models.WithdrawRequest{
		Order: "12345678903",
		Sum:   500.0,
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	balanceHandler.Withdraw(w, httpReq)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestBalanceHandler_Withdraw_InvalidJSON(t *testing.T) {
	balanceHandler, _, _, _, authMiddleware, userID := setupBalanceTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewBufferString(`{"order": "123", "sum": 500`))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(httpReq.Context(), middleware.UserIDKey, userID.String())
	httpReq = httpReq.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.Withdraw(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request format")
}

func TestBalanceHandler_Withdraw_OrderAlreadyExists(t *testing.T) {
	balanceHandler, mockBalanceRepo, _, mockOrderRepo, authMiddleware, userID := setupBalanceTest(t)

	// Добавляем баланс
	mockBalanceRepo.AddBalance(userID, 1000.0)

	// Создаем существующий заказ
	existingOrder := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusProcessed,
	}
	mockOrderRepo.AddOrder(existingOrder)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := models.WithdrawRequest{
		Order: "12345678903",
		Sum:   500.0,
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewBuffer(body))
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(httpReq.Context(), middleware.UserIDKey, userID.String())
	httpReq = httpReq.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.Withdraw(w, httpReq)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid order number")
}

// ============================================
// Тесты для GET /api/user/withdrawals
// ============================================

func TestBalanceHandler_GetWithdrawals_Success(t *testing.T) {
	balanceHandler, _, mockWithdrawalRepo, _, authMiddleware, userID := setupBalanceTest(t)

	// Создаем списания
	withdrawals := []struct {
		order string
		sum   float64
	}{
		{"12345678903", 500.0},
		{"5555555555554444", 300.0},
		{"4111111111111111", 200.0},
	}

	for _, w := range withdrawals {
		withdrawal := &models.Withdrawal{
			UserID:      userID,
			OrderNumber: w.order,
			Sum:         w.sum,
		}
		mockWithdrawalRepo.Create(context.Background(), withdrawal)
	}

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/user/withdrawals", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.GetWithdrawals(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response []models.WithdrawalResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Len(t, response, 3)
}

func TestBalanceHandler_GetWithdrawals_NoContent(t *testing.T) {
	balanceHandler, _, _, _, authMiddleware, userID := setupBalanceTest(t)

	token, err := authMiddleware.GenerateToken(userID.String())
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/user/withdrawals", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID.String())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	balanceHandler.GetWithdrawals(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestBalanceHandler_GetWithdrawals_Unauthorized(t *testing.T) {
	balanceHandler, _, _, _, _, _ := setupBalanceTest(t)

	req := httptest.NewRequest("GET", "/api/user/withdrawals", nil)
	w := httptest.NewRecorder()

	balanceHandler.GetWithdrawals(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

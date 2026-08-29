package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/subtotalstew/gophermart/internal/middleware"
	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/service"
)

// BalanceHandler - хендлер для работы с балансом
type BalanceHandler struct {
	balanceService *service.BalanceService
	authMW         *middleware.AuthMiddleware
}

// NewBalanceHandler создает новый экземпляр хендлера баланса
func NewBalanceHandler(balanceService *service.BalanceService, authMW *middleware.AuthMiddleware) *BalanceHandler {
	return &BalanceHandler{
		balanceService: balanceService,
		authMW:         authMW,
	}
}

// GetBalance обрабатывает GET /api/user/balance
// Возвращает текущий баланс пользователя и сумму списаний
func (h *BalanceHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, ok := middleware.GetUserUUIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Получаем баланс
	balance, err := h.balanceService.GetBalance(r.Context(), userID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Формируем ответ
	response := models.BalanceResponse{
		Current:   balance.Current,
		Withdrawn: balance.Withdrawn,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Withdraw обрабатывает POST /api/user/balance/withdraw
// Списывает баллы с баланса пользователя
func (h *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, ok := middleware.GetUserUUIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Парсим запрос
	var req models.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Валидация
	if req.Order == "" {
		http.Error(w, "Order number is required", http.StatusBadRequest)
		return
	}
	if req.Sum <= 0 {
		http.Error(w, "Withdrawal sum must be positive", http.StatusBadRequest)
		return
	}

	// Выполняем списание
	err := h.balanceService.Withdraw(r.Context(), userID, req.Order, req.Sum)
	if err != nil {
		switch err {
		case service.ErrInsufficientFunds:
			http.Error(w, "Insufficient funds", http.StatusPaymentRequired)
			return
		case service.ErrInvalidWithdrawOrder:
			http.Error(w, "Invalid order number", http.StatusUnprocessableEntity)
			return
		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// GetWithdrawals обрабатывает GET /api/user/withdrawals
// Возвращает историю списаний пользователя
func (h *BalanceHandler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, ok := middleware.GetUserUUIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Получаем историю списаний
	withdrawals, err := h.balanceService.GetWithdrawals(r.Context(), userID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Проверяем, есть ли данные
	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Формируем ответ
	response := make([]models.WithdrawalResponse, len(withdrawals))
	for i, withdrawal := range withdrawals {
		response[i] = models.WithdrawalResponse{
			Order:       withdrawal.OrderNumber,
			Sum:         withdrawal.Sum,
			ProcessedAt: withdrawal.ProcessedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

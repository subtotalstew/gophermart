package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/subtotalstew/gophermart/internal/middleware"
	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/service"
)

type OrderHandler struct {
	orderService *service.OrderService
}

func NewOrderHandler(orderService *service.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

// UploadOrder обрабатывает POST /api/user/orders
func (h *OrderHandler) UploadOrder(w http.ResponseWriter, r *http.Request) {
	// Проверяем Content-Type
	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	orderNumber := string(body)

	// Получаем userID из контекста
	userID, ok := middleware.GetUserUUIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Загружаем заказ
	order, err := h.orderService.UploadOrder(r.Context(), userID, orderNumber)
	if err != nil {
		switch err {
		case service.ErrEmptyOrderNumber, service.ErrInvalidOrderNumber:
			http.Error(w, "Invalid order number format", http.StatusUnprocessableEntity)
			return
		case service.ErrOrderAlreadyUploadedByUser:
			// Заказ уже загружен этим пользователем
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(order)
			return
		case service.ErrOrderAlreadyUploadedByOther:
			http.Error(w, "Order already uploaded by another user", http.StatusConflict)
			return
		default:
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	// Новый заказ принят в обработку
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(order)
}

// GetOrders обрабатывает GET /api/user/orders
func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из контекста
	userID, ok := middleware.GetUserUUIDFromContext(r.Context())
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Получаем заказы пользователя
	orders, err := h.orderService.GetUserOrders(r.Context(), userID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Преобразуем в формат ответа
	response := make([]models.OrderResponse, len(orders))
	for i, order := range orders {
		response[i] = models.OrderResponse{
			Number:     order.Number,
			Status:     order.Status,
			Accrual:    order.Accrual,
			UploadedAt: order.UploadedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/subtotalstew/gophermart/internal/accrual"
	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/service"
)

// OrderProcessor - воркер для обработки заказов
type OrderProcessor struct {
	orderService   *service.OrderService
	balanceService *service.BalanceService
	accrualClient  accrual.AccrualClientInterface
	ticker         *time.Ticker
	interval       time.Duration
	stopChan       chan struct{}
}

// NewOrderProcessor создает новый экземпляр воркера
func NewOrderProcessor(
	orderService *service.OrderService,
	balanceService *service.BalanceService,
	accrualClient accrual.AccrualClientInterface,
	interval time.Duration,
) *OrderProcessor {
	return &OrderProcessor{
		orderService:   orderService,
		balanceService: balanceService,
		accrualClient:  accrualClient,
		interval:       interval,
		ticker:         time.NewTicker(interval),
		stopChan:       make(chan struct{}),
	}
}

// Run запускает воркер
func (w *OrderProcessor) Run(ctx context.Context) {
	slog.Info("Order processor started", "interval", w.interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Order processor stopped by context")
			return
		case <-w.stopChan:
			slog.Info("Order processor stopped by stop signal")
			return
		case <-w.ticker.C:
			w.processOrders(ctx)
		}
	}
}

// Stop останавливает воркер
func (w *OrderProcessor) Stop() {
	w.ticker.Stop()
	close(w.stopChan)
}

// processOrders обрабатывает заказы, требующие проверки
func (w *OrderProcessor) processOrders(ctx context.Context) {
	// Получаем заказы для обработки
	orders, err := w.orderService.GetOrdersForProcessing(ctx)
	if err != nil {
		slog.Error("Failed to get orders for processing", "error", err)
		return
	}

	if len(orders) == 0 {
		return
	}

	slog.Info("Processing orders", "count", len(orders))

	for _, order := range orders {
		w.processOrder(ctx, order)
	}
}

// processOrder обрабатывает один заказ
func (w *OrderProcessor) processOrder(ctx context.Context, order models.Order) {
	slog.Info("Processing order", "order", order.Number, "status", order.Status)

	// 1. Получаем информацию из Accrual System
	resp, err := w.accrualClient.GetOrderInfo(ctx, order.Number)
	if err != nil {
		w.handleAccrualError(ctx, order, err)
		return
	}

	// 2. Обрабатываем ответ
	w.handleAccrualResponse(ctx, order, resp)
}

// handleAccrualError обрабатывает ошибки от Accrual System
func (w *OrderProcessor) handleAccrualError(ctx context.Context, order models.Order, err error) {
	// Rate limit - пропускаем, повторим позже
	if _, ok := err.(*accrual.RateLimitError); ok {
		slog.Warn("Rate limit exceeded, will retry later", "order", order.Number)
		return
	}

	// Заказ не зарегистрирован - пропускаем
	if err == accrual.ErrOrderNotRegistered {
		slog.Info("Order not registered in accrual system", "order", order.Number)
		return
	}

	// Другие ошибки - логируем, заказ останется в очереди
	slog.Error("Failed to get order info", "order", order.Number, "error", err)
}

// handleAccrualResponse обрабатывает успешный ответ от Accrual System
func (w *OrderProcessor) handleAccrualResponse(ctx context.Context, order models.Order, resp *accrual.OrderResponse) {
	newStatus := accrual.MapAccrualStatusToOrderStatus(resp.Status)

	switch {
	// Случай 1: PROCESSED с начислением
	case newStatus == models.OrderStatusProcessed && resp.Accrual != nil:
		w.handleProcessedWithAccrual(ctx, order, resp)

	// Случай 2: PROCESSED без начисления → INVALID
	case newStatus == models.OrderStatusProcessed && resp.Accrual == nil:
		w.handleProcessedWithoutAccrual(ctx, order)

	// Случай 3: INVALID
	case newStatus == models.OrderStatusInvalid:
		w.handleInvalid(ctx, order)

	// Случай 4: PROCESSING
	case newStatus == models.OrderStatusProcessing:
		w.handleProcessing(ctx, order)

	// Случай 5: неизвестный статус → INVALID
	default:
		w.handleUnknownStatus(ctx, order, resp.Status)
	}
}

// handleProcessedWithAccrual обрабатывает заказ с начислением
func (w *OrderProcessor) handleProcessedWithAccrual(ctx context.Context, order models.Order, resp *accrual.OrderResponse) {
	// Обновляем статус и начисление
	if err := w.orderService.UpdateOrderStatusAndAccrual(ctx, order.Number, models.OrderStatusProcessed, resp.Accrual); err != nil {
		slog.Error("Failed to update order status and accrual", "order", order.Number, "error", err)
		return
	}

	// Начисляем баллы на баланс
	if err := w.balanceService.AddAccrual(ctx, order.UserID, *resp.Accrual); err != nil {
		slog.Error("Failed to add accrual to balance",
			"order", order.Number,
			"user", order.UserID.String(),
			"amount", *resp.Accrual,
			"error", err)
		return
	}

	slog.Info("Order processed successfully",
		"order", order.Number,
		"accrual", *resp.Accrual,
		"user", order.UserID.String())
}

// handleProcessedWithoutAccrual обрабатывает случай PROCESSED без начисления
func (w *OrderProcessor) handleProcessedWithoutAccrual(ctx context.Context, order models.Order) {
	slog.Warn("Order processed but no accrual, marking as INVALID", "order", order.Number)

	if err := w.orderService.UpdateOrderStatus(ctx, order.Number, models.OrderStatusInvalid); err != nil {
		slog.Error("Failed to update order status to invalid", "order", order.Number, "error", err)
	}
}

// handleInvalid обрабатывает невалидный заказ
func (w *OrderProcessor) handleInvalid(ctx context.Context, order models.Order) {
	if err := w.orderService.UpdateOrderStatus(ctx, order.Number, models.OrderStatusInvalid); err != nil {
		slog.Error("Failed to update order status to invalid", "order", order.Number, "error", err)
		return
	}
	slog.Info("Order marked as invalid", "order", order.Number)
}

// handleProcessing обрабатывает заказ в статусе PROCESSING
func (w *OrderProcessor) handleProcessing(ctx context.Context, order models.Order) {
	if order.Status != models.OrderStatusProcessing {
		if err := w.orderService.UpdateOrderStatus(ctx, order.Number, models.OrderStatusProcessing); err != nil {
			slog.Error("Failed to update order status to processing", "order", order.Number, "error", err)
			return
		}
		slog.Info("Order status updated to processing", "order", order.Number)
	}
}

// handleUnknownStatus обрабатывает неизвестный статус
func (w *OrderProcessor) handleUnknownStatus(ctx context.Context, order models.Order, status string) {
	slog.Warn("Unknown status from accrual system, marking as INVALID",
		"order", order.Number,
		"status", status)

	if err := w.orderService.UpdateOrderStatus(ctx, order.Number, models.OrderStatusInvalid); err != nil {
		slog.Error("Failed to update order status to invalid", "order", order.Number, "error", err)
	}
}

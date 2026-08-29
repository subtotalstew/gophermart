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
	accrualClient  accrual.AccrualClientInterface // Используем интерфейс
	ticker         *time.Ticker
	interval       time.Duration
	stopChan       chan struct{}
}

// NewOrderProcessor создает новый экземпляр воркера
func NewOrderProcessor(
	orderService *service.OrderService,
	balanceService *service.BalanceService,
	accrualClient accrual.AccrualClientInterface, // Принимаем интерфейс
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

	// Запрашиваем информацию о заказе из Accrual System
	resp, err := w.accrualClient.GetOrderInfo(ctx, order.Number)
	if err != nil {
		// Проверяем, не превышен ли лимит запросов
		if _, ok := err.(*accrual.RateLimitError); ok {
			slog.Warn("Rate limit exceeded, will retry later",
				"order", order.Number)
			return
		}

		if err == accrual.ErrOrderNotRegistered {
			// Заказ не зарегистрирован в системе расчета - пропускаем
			slog.Info("Order not registered in accrual system", "order", order.Number)
			return
		}

		slog.Error("Failed to get order info", "order", order.Number, "error", err)
		return
	}

	// Обновляем статус заказа
	newStatus := accrual.MapAccrualStatusToOrderStatus(resp.Status)

	// Если заказ обработан и есть начисление
	if newStatus == models.OrderStatusProcessed && resp.Accrual != nil {
		// Обновляем статус и начисление
		if err := w.orderService.UpdateOrderStatusAndAccrual(ctx, order.Number, newStatus, resp.Accrual); err != nil {
			slog.Error("Failed to update order status and accrual", "order", order.Number, "error", err)
			return
		}

		// Начисляем баллы на баланс пользователя
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

	} else if newStatus == models.OrderStatusInvalid {
		// Заказ признан невалидным
		if err := w.orderService.UpdateOrderStatus(ctx, order.Number, newStatus); err != nil {
			slog.Error("Failed to update order status to invalid", "order", order.Number, "error", err)
			return
		}
		slog.Info("Order marked as invalid", "order", order.Number)

	} else if newStatus == models.OrderStatusProcessing {
		// Заказ все еще в обработке - обновляем статус, если изменился
		if order.Status != models.OrderStatusProcessing {
			if err := w.orderService.UpdateOrderStatus(ctx, order.Number, newStatus); err != nil {
				slog.Error("Failed to update order status to processing", "order", order.Number, "error", err)
				return
			}
			slog.Info("Order status updated to processing", "order", order.Number)
		}
	}
}

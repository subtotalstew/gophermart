package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/subtotalstew/gophermart/internal/accrual"
	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/service"
	"github.com/subtotalstew/gophermart/internal/testutil"
)

// ============================================
// Вспомогательные функции
// ============================================

func setupOrderProcessorTest(t *testing.T) (*OrderProcessor, *testutil.MockOrderRepository, *testutil.MockBalanceRepository, *testutil.MockWithdrawalRepository, *testutil.MockUserRepository, pgtype.UUID) {
	mockUserRepo := testutil.NewMockUserRepository()
	mockOrderRepo := testutil.NewMockOrderRepository()
	mockBalanceRepo := testutil.NewMockBalanceRepository()
	mockWithdrawalRepo := testutil.NewMockWithdrawalRepository()

	orderService := service.NewOrderService(mockOrderRepo, mockUserRepo)
	balanceService := service.NewBalanceService(mockBalanceRepo, mockWithdrawalRepo, mockOrderRepo)

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

	// Создаем мок-клиент для Accrual System
	mockAccrualClient := testutil.NewMockAccrualClient()

	processor := NewOrderProcessor(
		orderService,
		balanceService,
		mockAccrualClient,
		100*time.Millisecond,
	)

	return processor, mockOrderRepo, mockBalanceRepo, mockWithdrawalRepo, mockUserRepo, userID
}

// ============================================
// Тесты для OrderProcessor
// ============================================

func TestOrderProcessor_ProcessOrder_Success(t *testing.T) {
	processor, mockOrderRepo, mockBalanceRepo, _, _, userID := setupOrderProcessorTest(t)

	// Создаем заказ со статусом NEW
	order := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusNew,
	}
	mockOrderRepo.AddOrder(order)

	// Настраиваем мок-клиент на успешный ответ
	mockAccrualClient := processor.accrualClient.(*testutil.MockAccrualClient)
	mockAccrualClient.SetResponse(&accrual.OrderResponse{
		Order:   "12345678903",
		Status:  accrual.StatusProcessed,
		Accrual: ptr(500.0),
	}, nil)

	// Запускаем обработку
	ctx := context.Background()
	processor.processOrders(ctx)

	// Проверяем, что заказ обновился
	updatedOrder, err := mockOrderRepo.FindByNumber(ctx, "12345678903")
	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusProcessed, updatedOrder.Status)
	assert.NotNil(t, updatedOrder.Accrual)
	assert.Equal(t, 500.0, *updatedOrder.Accrual)

	// Проверяем, что баланс обновился
	balance, err := mockBalanceRepo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 500.0, balance.Current)
}

func TestOrderProcessor_ProcessOrder_Processing(t *testing.T) {
	processor, mockOrderRepo, mockBalanceRepo, _, _, userID := setupOrderProcessorTest(t)

	// Создаем заказ со статусом NEW
	order := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusNew,
	}
	mockOrderRepo.AddOrder(order)

	// Настраиваем мок-клиент на статус PROCESSING
	mockAccrualClient := processor.accrualClient.(*testutil.MockAccrualClient)
	mockAccrualClient.SetResponse(&accrual.OrderResponse{
		Order:  "12345678903",
		Status: accrual.StatusProcessing,
	}, nil)

	ctx := context.Background()
	processor.processOrders(ctx)

	// Проверяем, что статус обновился на PROCESSING
	updatedOrder, err := mockOrderRepo.FindByNumber(ctx, "12345678903")
	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusProcessing, updatedOrder.Status)
	assert.Nil(t, updatedOrder.Accrual)

	// Проверяем, что баланс не изменился
	balance, err := mockBalanceRepo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, balance.Current)
}

func TestOrderProcessor_ProcessOrder_Invalid(t *testing.T) {
	processor, mockOrderRepo, mockBalanceRepo, _, _, userID := setupOrderProcessorTest(t)

	// Создаем заказ со статусом NEW
	order := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusNew,
	}
	mockOrderRepo.AddOrder(order)

	// Настраиваем мок-клиент на статус INVALID
	mockAccrualClient := processor.accrualClient.(*testutil.MockAccrualClient)
	mockAccrualClient.SetResponse(&accrual.OrderResponse{
		Order:  "12345678903",
		Status: accrual.StatusInvalid,
	}, nil)

	ctx := context.Background()
	processor.processOrders(ctx)

	// Проверяем, что статус обновился на INVALID
	updatedOrder, err := mockOrderRepo.FindByNumber(ctx, "12345678903")
	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusInvalid, updatedOrder.Status)
	assert.Nil(t, updatedOrder.Accrual)

	// Проверяем, что баланс не изменился
	balance, err := mockBalanceRepo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, balance.Current)
}

func TestOrderProcessor_ProcessOrder_Error(t *testing.T) {
	processor, mockOrderRepo, mockBalanceRepo, _, _, userID := setupOrderProcessorTest(t)

	// Создаем заказ со статусом NEW
	order := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusNew,
	}
	mockOrderRepo.AddOrder(order)

	// Настраиваем мок-клиент на ошибку
	mockAccrualClient := processor.accrualClient.(*testutil.MockAccrualClient)
	mockAccrualClient.SetResponse(nil, errors.New("network error"))

	ctx := context.Background()
	processor.processOrders(ctx)

	// Проверяем, что статус не изменился
	updatedOrder, err := mockOrderRepo.FindByNumber(ctx, "12345678903")
	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusNew, updatedOrder.Status)

	// Проверяем, что баланс не изменился
	balance, err := mockBalanceRepo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, balance.Current)
}

func TestOrderProcessor_ProcessOrder_OrderNotRegistered(t *testing.T) {
	processor, mockOrderRepo, mockBalanceRepo, _, _, userID := setupOrderProcessorTest(t)

	// Создаем заказ со статусом NEW
	order := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusNew,
	}
	mockOrderRepo.AddOrder(order)

	// Настраиваем мок-клиент на ошибку "order not registered"
	mockAccrualClient := processor.accrualClient.(*testutil.MockAccrualClient)
	mockAccrualClient.SetResponse(nil, accrual.ErrOrderNotRegistered)

	ctx := context.Background()
	processor.processOrders(ctx)

	// Проверяем, что статус не изменился
	updatedOrder, err := mockOrderRepo.FindByNumber(ctx, "12345678903")
	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusNew, updatedOrder.Status)

	// Проверяем, что баланс не изменился
	balance, err := mockBalanceRepo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, balance.Current)
}

func TestOrderProcessor_ProcessOrder_RateLimit(t *testing.T) {
	processor, mockOrderRepo, mockBalanceRepo, _, _, userID := setupOrderProcessorTest(t)

	// Создаем заказ со статусом NEW
	order := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusNew,
	}
	mockOrderRepo.AddOrder(order)

	// Настраиваем мок-клиент на ошибку RateLimit
	mockAccrualClient := processor.accrualClient.(*testutil.MockAccrualClient)
	mockAccrualClient.SetResponse(nil, &accrual.RateLimitError{RetryAfter: 60 * time.Second})

	ctx := context.Background()
	processor.processOrders(ctx)

	// Проверяем, что статус не изменился
	updatedOrder, err := mockOrderRepo.FindByNumber(ctx, "12345678903")
	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusNew, updatedOrder.Status)

	// Проверяем, что баланс не изменился
	balance, err := mockBalanceRepo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, balance.Current)
}

func TestOrderProcessor_ProcessMultipleOrders(t *testing.T) {
	processor, mockOrderRepo, mockBalanceRepo, _, _, userID := setupOrderProcessorTest(t)

	// Создаем несколько заказов
	orders := []struct {
		number  string
		status  string
		accrual float64
	}{
		{"12345678901", accrual.StatusProcessed, 100.0},
		{"12345678902", accrual.StatusProcessed, 200.0},
		{"12345678903", accrual.StatusProcessed, 300.0},
		{"12345678904", accrual.StatusInvalid, 0},
	}

	for _, o := range orders {
		order := &models.Order{
			Number: o.number,
			UserID: userID,
			Status: models.OrderStatusNew,
		}
		mockOrderRepo.AddOrder(order)
	}

	// Настраиваем мок-клиент на ответы для каждого заказа
	mockAccrualClient := processor.accrualClient.(*testutil.MockAccrualClient)
	mockAccrualClient.SetResponseForOrder("12345678901", &accrual.OrderResponse{
		Order:   "12345678901",
		Status:  accrual.StatusProcessed,
		Accrual: ptr(100.0),
	}, nil)
	mockAccrualClient.SetResponseForOrder("12345678902", &accrual.OrderResponse{
		Order:   "12345678902",
		Status:  accrual.StatusProcessed,
		Accrual: ptr(200.0),
	}, nil)
	mockAccrualClient.SetResponseForOrder("12345678903", &accrual.OrderResponse{
		Order:   "12345678903",
		Status:  accrual.StatusProcessed,
		Accrual: ptr(300.0),
	}, nil)
	mockAccrualClient.SetResponseForOrder("12345678904", &accrual.OrderResponse{
		Order:  "12345678904",
		Status: accrual.StatusInvalid,
	}, nil)

	ctx := context.Background()
	processor.processOrders(ctx)

	// Проверяем все заказы
	for _, o := range orders {
		updatedOrder, err := mockOrderRepo.FindByNumber(ctx, o.number)
		require.NoError(t, err)

		if o.accrual > 0 {
			assert.Equal(t, models.OrderStatusProcessed, updatedOrder.Status)
			assert.NotNil(t, updatedOrder.Accrual)
			assert.Equal(t, o.accrual, *updatedOrder.Accrual)
		} else {
			assert.Equal(t, models.OrderStatusInvalid, updatedOrder.Status)
			assert.Nil(t, updatedOrder.Accrual)
		}
	}

	// Проверяем общий баланс (сумма всех начислений)
	balance, err := mockBalanceRepo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 600.0, balance.Current) // 100 + 200 + 300
}

func TestOrderProcessor_RunAndStop(t *testing.T) {
	processor, _, _, _, _, _ := setupOrderProcessorTest(t)

	ctx, cancel := context.WithCancel(context.Background())

	// Запускаем воркер
	go processor.Run(ctx)

	// Даем ему поработать немного
	time.Sleep(150 * time.Millisecond)

	// Останавливаем через контекст
	cancel()

	// Даем время на остановку
	time.Sleep(50 * time.Millisecond)

	// Проверяем, что воркер остановился (нет паники)
}

func TestOrderProcessor_Stop(t *testing.T) {
	processor, _, _, _, _, _ := setupOrderProcessorTest(t)

	ctx := context.Background()

	// Запускаем воркер
	go processor.Run(ctx)

	// Даем ему поработать немного
	time.Sleep(150 * time.Millisecond)

	// Останавливаем через Stop
	processor.Stop()

	// Даем время на остановку
	time.Sleep(50 * time.Millisecond)

	// Проверяем, что воркер остановился (нет паники)
}

func TestOrderProcessor_NoOrders(t *testing.T) {
	processor, mockOrderRepo, _, _, _, _ := setupOrderProcessorTest(t)

	// Нет заказов в репозитории
	ctx := context.Background()

	// Запускаем обработку - ничего не должно произойти
	processor.processOrders(ctx)

	// Проверяем, что заказов нет
	orders, err := mockOrderRepo.GetOrdersByStatus(ctx, []string{models.OrderStatusNew, models.OrderStatusProcessing})
	require.NoError(t, err)
	assert.Len(t, orders, 0)
}

func TestOrderProcessor_ProcessOrder_AlreadyProcessing(t *testing.T) {
	processor, mockOrderRepo, mockBalanceRepo, _, _, userID := setupOrderProcessorTest(t)

	// Создаем заказ со статусом PROCESSING
	order := &models.Order{
		Number: "12345678903",
		UserID: userID,
		Status: models.OrderStatusProcessing,
	}
	mockOrderRepo.AddOrder(order)

	// Настраиваем мок-клиент на успешный ответ
	mockAccrualClient := processor.accrualClient.(*testutil.MockAccrualClient)
	mockAccrualClient.SetResponse(&accrual.OrderResponse{
		Order:   "12345678903",
		Status:  accrual.StatusProcessed,
		Accrual: ptr(500.0),
	}, nil)

	ctx := context.Background()
	processor.processOrders(ctx)

	// Проверяем, что заказ обновился
	updatedOrder, err := mockOrderRepo.FindByNumber(ctx, "12345678903")
	require.NoError(t, err)
	assert.Equal(t, models.OrderStatusProcessed, updatedOrder.Status)
	assert.NotNil(t, updatedOrder.Accrual)
	assert.Equal(t, 500.0, *updatedOrder.Accrual)

	// Проверяем, что баланс обновился
	balance, err := mockBalanceRepo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, 500.0, balance.Current)
}

// ============================================
// Вспомогательные функции
// ============================================

func ptr(v float64) *float64 {
	return &v
}

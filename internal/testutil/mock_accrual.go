package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/subtotalstew/gophermart/internal/accrual"
)

// MockAccrualClient - мок для Accrual Client (реализует AccrualClientInterface)
type MockAccrualClient struct {
	mu          sync.RWMutex
	responses   map[string]mockResponse // orderNumber -> response
	defaultResp mockResponse
	retryAfter  time.Duration
}

type mockResponse struct {
	resp *accrual.OrderResponse
	err  error
}

// NewMockAccrualClient создает новый мок-клиент
func NewMockAccrualClient() *MockAccrualClient {
	return &MockAccrualClient{
		responses:  make(map[string]mockResponse),
		retryAfter: 60 * time.Second,
		defaultResp: mockResponse{
			resp: nil,
			err:  nil,
		},
	}
}

// SetResponse устанавливает ответ по умолчанию
func (m *MockAccrualClient) SetResponse(resp *accrual.OrderResponse, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.defaultResp = mockResponse{resp: resp, err: err}
}

// SetResponseForOrder устанавливает ответ для конкретного заказа
func (m *MockAccrualClient) SetResponseForOrder(orderNumber string, resp *accrual.OrderResponse, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[orderNumber] = mockResponse{resp: resp, err: err}
}

// GetOrderInfo реализует метод интерфейса
func (m *MockAccrualClient) GetOrderInfo(ctx context.Context, orderNumber string) (*accrual.OrderResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Проверяем, есть ли ответ для конкретного заказа
	if resp, ok := m.responses[orderNumber]; ok {
		return resp.resp, resp.err
	}

	// Возвращаем ответ по умолчанию
	return m.defaultResp.resp, m.defaultResp.err
}

// GetRetryAfter возвращает время ожидания
func (m *MockAccrualClient) GetRetryAfter() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.retryAfter
}

// SetRetryAfter устанавливает время ожидания
func (m *MockAccrualClient) SetRetryAfter(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retryAfter = duration
}

// SetTimeout устанавливает таймаут
func (m *MockAccrualClient) SetTimeout(timeout time.Duration) {
	// Ничего не делаем в моке
}

// Clear очищает все ответы
func (m *MockAccrualClient) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = make(map[string]mockResponse)
	m.defaultResp = mockResponse{resp: nil, err: nil}
	m.retryAfter = 60 * time.Second
}

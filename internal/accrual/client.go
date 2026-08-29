package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// AccrualClientInterface - интерфейс для клиента Accrual System
type AccrualClientInterface interface {
	GetOrderInfo(ctx context.Context, orderNumber string) (*OrderResponse, error)
	GetRetryAfter() time.Duration
	SetRetryAfter(duration time.Duration)
	SetTimeout(timeout time.Duration)
}

// Client - клиент для взаимодействия с системой расчета баллов
type Client struct {
	baseURL    string
	httpClient *http.Client
	retryAfter time.Duration
}

// NewClient создает новый экземпляр клиента
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryAfter: 60 * time.Second,
	}
}

// GetOrderInfo получает информацию о заказе из системы расчета баллов
func (c *Client) GetOrderInfo(ctx context.Context, orderNumber string) (*OrderResponse, error) {
	// Формируем URL
	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber)

	// Создаем запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Выполняем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Обрабатываем ответ
	switch resp.StatusCode {
	case http.StatusOK:
		// Успешный ответ
		var orderResp OrderResponse
		if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &orderResp, nil

	case http.StatusNoContent:
		// Заказ не зарегистрирован в системе расчета
		return nil, ErrOrderNotRegistered

	case http.StatusTooManyRequests:
		// Превышен лимит запросов
		retryAfter := c.parseRetryAfter(resp)
		return nil, &RateLimitError{RetryAfter: retryAfter}

	case http.StatusInternalServerError:
		return nil, fmt.Errorf("accrual system internal error: %w", ErrInternalError)

	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// parseRetryAfter извлекает Retry-After из заголовка ответа
func (c *Client) parseRetryAfter(resp *http.Response) time.Duration {
	if retryAfterHeader := resp.Header.Get("Retry-After"); retryAfterHeader != "" {
		// Пробуем распарсить как число (секунды)
		if seconds, err := strconv.Atoi(retryAfterHeader); err == nil {
			return time.Duration(seconds) * time.Second
		}
		// Пробуем распарсить как HTTP-дату
		if t, err := http.ParseTime(retryAfterHeader); err == nil {
			return time.Until(t)
		}
	}
	// Значение по умолчанию
	return c.retryAfter
}

// GetRetryAfter возвращает время ожидания при ограничении запросов
func (c *Client) GetRetryAfter() time.Duration {
	return c.retryAfter
}

// SetRetryAfter устанавливает время ожидания при ограничении запросов
func (c *Client) SetRetryAfter(duration time.Duration) {
	c.retryAfter = duration
}

// SetTimeout устанавливает таймаут для HTTP-клиента
func (c *Client) SetTimeout(timeout time.Duration) {
	c.httpClient.Timeout = timeout
}

// ============================================
// Ошибки
// ============================================

var (
	// ErrOrderNotRegistered - заказ не зарегистрирован в системе расчета
	ErrOrderNotRegistered = fmt.Errorf("order not registered in accrual system")

	// ErrInternalError - внутренняя ошибка системы расчета
	ErrInternalError = fmt.Errorf("accrual system internal error")
)

// RateLimitError - ошибка превышения лимита запросов
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded, retry after %v", e.RetryAfter)
}

func (e *RateLimitError) Is(target error) bool {
	_, ok := target.(*RateLimitError)
	return ok
}

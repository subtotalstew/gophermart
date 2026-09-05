package accrual

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://test.com")

	assert.NotNil(t, client)
	assert.Equal(t, "http://test.com", client.baseURL)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 60*time.Second, client.retryAfter)
}

func TestClient_GetOrderInfo_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(OrderResponse{
			Order:   "12345678903",
			Status:  StatusProcessed,
			Accrual: ptr(500.5),
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.GetOrderInfo(context.Background(), "12345678903")

	require.NoError(t, err)
	assert.Equal(t, "12345678903", resp.Order)
	assert.Equal(t, StatusProcessed, resp.Status)
	assert.Equal(t, 500.5, *resp.Accrual)
}

func TestClient_GetOrderInfo_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.GetOrderInfo(context.Background(), "12345678903")

	assert.Error(t, err)
	assert.Equal(t, ErrOrderNotRegistered, err)
	assert.Nil(t, resp)
}

func TestClient_GetOrderInfo_RateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.GetOrderInfo(context.Background(), "12345678903")

	assert.Error(t, err)
	assert.Nil(t, resp)

	rateLimitErr, ok := err.(*RateLimitError)
	assert.True(t, ok)
	assert.Equal(t, 30*time.Second, rateLimitErr.RetryAfter)
}

func TestClient_GetOrderInfo_RateLimit_DefaultRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.GetOrderInfo(context.Background(), "12345678903")

	assert.Error(t, err)
	assert.Nil(t, resp)

	rateLimitErr, ok := err.(*RateLimitError)
	assert.True(t, ok)
	assert.Equal(t, 60*time.Second, rateLimitErr.RetryAfter)
}

func TestClient_GetOrderInfo_InternalServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.GetOrderInfo(context.Background(), "12345678903")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "accrual system internal error")
}

func TestClient_GetOrderInfo_Processing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(OrderResponse{
			Order:  "12345678903",
			Status: StatusProcessing,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.GetOrderInfo(context.Background(), "12345678903")

	require.NoError(t, err)
	assert.Equal(t, "12345678903", resp.Order)
	assert.Equal(t, StatusProcessing, resp.Status)
	assert.Nil(t, resp.Accrual)
}

func TestClient_GetOrderInfo_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"order": "123", "status": "PROCESSED", "accrual": "invalid"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.GetOrderInfo(context.Background(), "12345678903")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to decode response")
}

func TestClient_parseRetryAfter(t *testing.T) {
	client := NewClient("http://test.com")

	tests := []struct {
		name       string
		retryAfter string
		expected   time.Duration
	}{
		{
			name:       "seconds",
			retryAfter: "30",
			expected:   30 * time.Second,
		},
		{
			name:       "empty",
			retryAfter: "",
			expected:   60 * time.Second,
		},
		{
			name:       "invalid",
			retryAfter: "invalid",
			expected:   60 * time.Second,
		},
		{
			name:       "http date - should return default",
			retryAfter: time.Now().Add(30 * time.Second).Format(time.RFC1123),
			expected:   60 * time.Second, // parseRetryAfter возвращает default для дат
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: make(http.Header)}
			if tt.retryAfter != "" {
				resp.Header.Set("Retry-After", tt.retryAfter)
			}

			result := client.parseRetryAfter(resp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClient_SetRetryAfter(t *testing.T) {
	client := NewClient("http://test.com")
	assert.Equal(t, 60*time.Second, client.retryAfter)

	client.SetRetryAfter(10 * time.Second)
	assert.Equal(t, 10*time.Second, client.retryAfter)
}

func TestClient_SetTimeout(t *testing.T) {
	client := NewClient("http://test.com")
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)

	client.SetTimeout(60 * time.Second)
	assert.Equal(t, 60*time.Second, client.httpClient.Timeout)
}

func TestClient_GetRetryAfter(t *testing.T) {
	client := NewClient("http://test.com")
	assert.Equal(t, 60*time.Second, client.GetRetryAfter())

	client.SetRetryAfter(15 * time.Second)
	assert.Equal(t, 15*time.Second, client.GetRetryAfter())
}

func TestMapAccrualStatusToOrderStatus(t *testing.T) {
	tests := []struct {
		accrualStatus  string
		expectedStatus string
	}{
		{StatusRegistered, "NEW"},
		{StatusInvalid, "INVALID"},
		{StatusProcessing, "PROCESSING"},
		{StatusProcessed, "PROCESSED"},
		{"UNKNOWN", "NEW"},
		{"", "NEW"},
	}

	for _, tt := range tests {
		result := MapAccrualStatusToOrderStatus(tt.accrualStatus)
		assert.Equal(t, tt.expectedStatus, result)
	}
}

// ============================================
// Вспомогательные функции
// ============================================

func ptr(v float64) *float64 {
	return &v
}

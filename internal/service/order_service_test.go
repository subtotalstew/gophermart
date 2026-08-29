package service

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/testutil"
)

func TestOrderService_UploadOrder(t *testing.T) {
	mockOrderRepo := testutil.NewMockOrderRepository()
	mockUserRepo := testutil.NewMockUserRepository()

	// Теперь это работает, т.к. моки реализуют интерфейсы
	orderService := NewOrderService(mockOrderRepo, mockUserRepo)

	var userID pgtype.UUID
	userID.Scan("00000000-0000-0000-0000-000000000001")

	user := &models.User{
		ID:    userID,
		Login: "testuser",
	}
	mockUserRepo.Create(context.Background(), user)

	tests := []struct {
		name           string
		number         string
		userID         pgtype.UUID
		expectedError  error
		expectedStatus string
	}{
		{
			name:           "valid order",
			number:         "12345678903",
			userID:         userID,
			expectedError:  nil,
			expectedStatus: models.OrderStatusNew,
		},
		{
			name:          "invalid order number",
			number:        "12345",
			userID:        userID,
			expectedError: ErrInvalidOrderNumber,
		},
		{
			name:          "empty order number",
			number:        "",
			userID:        userID,
			expectedError: ErrEmptyOrderNumber,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrderRepo.Clear()

			order, err := orderService.UploadOrder(context.Background(), tt.userID, tt.number)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
				assert.Nil(t, order)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, order)
				assert.Equal(t, tt.expectedStatus, order.Status)
			}
		})
	}
}

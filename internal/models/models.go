package models

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Константы статусов заказов
const (
	OrderStatusNew        = "NEW"
	OrderStatusProcessing = "PROCESSING"
	OrderStatusInvalid    = "INVALID"
	OrderStatusProcessed  = "PROCESSED"
)

type User struct {
	ID           pgtype.UUID `json:"id"`
	Login        string      `json:"login"`
	PasswordHash string      `json:"-"`
	CreatedAt    time.Time   `json:"created_at"`
}

type Order struct {
	Number     string      `json:"number"`
	UserID     pgtype.UUID `json:"user_id"`
	Status     string      `json:"status"`
	Accrual    *float64    `json:"accrual,omitempty"`
	UploadedAt time.Time   `json:"uploaded_at"`
	UpdatedAt  time.Time   `json:"-"`
}

type Balance struct {
	UserID    pgtype.UUID `json:"user_id"`
	Current   float64     `json:"current"`
	Withdrawn float64     `json:"withdrawn"`
}

type Withdrawal struct {
	ID          pgtype.UUID `json:"id"`
	UserID      pgtype.UUID `json:"user_id"`
	OrderNumber string      `json:"order_number"`
	Sum         float64     `json:"sum"`
	ProcessedAt time.Time   `json:"processed_at"`
}

// DTO для запросов/ответов
type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type OrderResponse struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    *float64  `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type BalanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

type WithdrawalResponse struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

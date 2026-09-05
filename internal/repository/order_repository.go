package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/subtotalstew/gophermart/internal/models"
)

type OrderRepository struct {
	db *pgxpool.Pool
}

func NewOrderRepository(db *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Save(ctx context.Context, order *models.Order) error {
	query := `
        INSERT INTO orders (number, user_id, status, accrual, uploaded_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING uploaded_at, updated_at
    `

	now := time.Now()
	err := r.db.QueryRow(ctx, query,
		order.Number,
		order.UserID,
		order.Status,
		order.Accrual,
		now,
		now,
	).Scan(&order.UploadedAt, &order.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to save order: %w", err)
	}

	return nil
}

// SaveWithLock создает новый заказ с блокировкой для избежания гонки
func (r *OrderRepository) SaveWithLock(ctx context.Context, order *models.Order) error {
	query := `
		INSERT INTO orders (number, user_id, status, uploaded_at, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING uploaded_at, updated_at
	`
	err := r.db.QueryRow(ctx, query,
		order.Number,
		order.UserID,
		order.Status,
	).Scan(&order.UploadedAt, &order.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to save order: %w", mapPgError(err, ErrOrderExists))
	}
	return nil
}

func (r *OrderRepository) FindByNumber(ctx context.Context, number string) (*models.Order, error) {
	query := `
        SELECT number, user_id, status, accrual, uploaded_at, updated_at
        FROM orders
        WHERE number = $1
    `

	var order models.Order
	err := r.db.QueryRow(ctx, query, number).Scan(
		&order.Number,
		&order.UserID,
		&order.Status,
		&order.Accrual,
		&order.UploadedAt,
		&order.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find order: %w", err)
	}

	return &order, nil
}

func (r *OrderRepository) FindByUserID(ctx context.Context, userID pgtype.UUID) ([]models.Order, error) {
	query := `
        SELECT number, user_id, status, accrual, uploaded_at, updated_at
        FROM orders
        WHERE user_id = $1
        ORDER BY uploaded_at DESC
    `

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find orders: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		err := rows.Scan(
			&order.Number,
			&order.UserID,
			&order.Status,
			&order.Accrual,
			&order.UploadedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return orders, nil
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, number string, status string) error {
	query := `
        UPDATE orders
        SET status = $1, updated_at = CURRENT_TIMESTAMP
        WHERE number = $2
    `

	result, err := r.db.Exec(ctx, query, status, number)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("order not found: %s", number)
	}

	return nil
}

func (r *OrderRepository) UpdateStatusAndAccrual(ctx context.Context, number string, status string, accrual *float64) error {
	query := `
        UPDATE orders
        SET status = $1, accrual = $2, updated_at = CURRENT_TIMESTAMP
        WHERE number = $3
    `

	result, err := r.db.Exec(ctx, query, status, accrual, number)
	if err != nil {
		return fmt.Errorf("failed to update order status and accrual: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("order not found: %s", number)
	}

	return nil
}

func (r *OrderRepository) GetOrdersByStatus(ctx context.Context, statuses []string) ([]models.Order, error) {
	query := `
        SELECT number, user_id, status, accrual, uploaded_at, updated_at
        FROM orders
        WHERE status = ANY($1)
        ORDER BY uploaded_at ASC
    `

	rows, err := r.db.Query(ctx, query, statuses)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders by status: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		err := rows.Scan(
			&order.Number,
			&order.UserID,
			&order.Status,
			&order.Accrual,
			&order.UploadedAt,
			&order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return orders, nil
}

func (r *OrderRepository) OrderExists(ctx context.Context, number string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM orders WHERE number = $1)`
	var exists bool
	err := r.db.QueryRow(ctx, query, number).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check order existence: %w", err)
	}
	return exists, nil
}

func (r *OrderRepository) GetOrderOwner(ctx context.Context, number string) (*pgtype.UUID, error) {
	query := `SELECT user_id FROM orders WHERE number = $1`

	var userID pgtype.UUID
	err := r.db.QueryRow(ctx, query, number).Scan(&userID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order owner: %w", err)
	}

	return &userID, nil
}

// GetOrderOwnerWithLock получает владельца заказа с блокировкой
func (r *OrderRepository) GetOrderOwnerWithLock(ctx context.Context, number string) (*pgtype.UUID, error) {
	query := `
		SELECT user_id FROM orders WHERE number = $1 FOR UPDATE
	`

	var userID pgtype.UUID
	err := r.db.QueryRow(ctx, query, number).Scan(&userID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order owner: %w", err)
	}

	return &userID, nil
}

package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/subtotalstew/gophermart/internal/models"
)

// WithdrawalRepository - репозиторий для работы со списаниями
type WithdrawalRepository struct {
	db *pgxpool.Pool
}

// NewWithdrawalRepository создает новый экземпляр репозитория списаний
func NewWithdrawalRepository(db *pgxpool.Pool) *WithdrawalRepository {
	return &WithdrawalRepository{db: db}
}

// Create создает новую запись о списании
func (r *WithdrawalRepository) Create(ctx context.Context, withdrawal *models.Withdrawal) error {
	query := `
        INSERT INTO withdrawals (user_id, order_number, sum, processed_at)
        VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
        RETURNING id, processed_at
    `

	err := r.db.QueryRow(ctx, query,
		withdrawal.UserID,
		withdrawal.OrderNumber,
		withdrawal.Sum,
	).Scan(&withdrawal.ID, &withdrawal.ProcessedAt)

	if err != nil {
		return fmt.Errorf("failed to create withdrawal: %w", err)
	}

	return nil
}

// GetByUserID возвращает все списания пользователя, отсортированные по времени (новые сверху)
func (r *WithdrawalRepository) GetByUserID(ctx context.Context, userID pgtype.UUID) ([]models.Withdrawal, error) {
	query := `
        SELECT id, user_id, order_number, sum, processed_at
        FROM withdrawals
        WHERE user_id = $1
        ORDER BY processed_at DESC
    `

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []models.Withdrawal
	for rows.Next() {
		var withdrawal models.Withdrawal
		err := rows.Scan(
			&withdrawal.ID,
			&withdrawal.UserID,
			&withdrawal.OrderNumber,
			&withdrawal.Sum,
			&withdrawal.ProcessedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return withdrawals, nil
}

// GetTotalWithdrawn возвращает общую сумму списаний пользователя
func (r *WithdrawalRepository) GetTotalWithdrawn(ctx context.Context, userID pgtype.UUID) (float64, error) {
	query := `
        SELECT COALESCE(SUM(sum), 0)
        FROM withdrawals
        WHERE user_id = $1
    `

	var total float64
	err := r.db.QueryRow(ctx, query, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total withdrawn: %w", err)
	}

	return total, nil
}

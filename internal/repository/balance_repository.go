package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/subtotalstew/gophermart/internal/models"
)

// BalanceRepository - репозиторий для работы с балансом
type BalanceRepository struct {
	db *pgxpool.Pool
}

// NewBalanceRepository создает новый экземпляр репозитория баланса
func NewBalanceRepository(db *pgxpool.Pool) *BalanceRepository {
	return &BalanceRepository{db: db}
}

// Транзакционное списание
func (r *BalanceRepository) WithdrawInTransaction(ctx context.Context, withdrawal *models.Withdrawal) error {
	// Начинаем транзакцию
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Проверяем и обновляем баланс
	updateQuery := `
        UPDATE balances
        SET current = current - $1,
            withdrawn = withdrawn + $1
        WHERE user_id = $2 AND current >= $1
        RETURNING current, withdrawn
    `

	var current, withdrawn float64
	err = tx.QueryRow(ctx, updateQuery, withdrawal.Sum, withdrawal.UserID).Scan(&current, &withdrawn)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("insufficient funds or user not found")
	}
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	// 2. Создаем запись о списании
	insertQuery := `
        INSERT INTO withdrawals (user_id, order_number, sum, processed_at)
        VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
        RETURNING id, processed_at
    `

	err = tx.QueryRow(ctx, insertQuery,
		withdrawal.UserID,
		withdrawal.OrderNumber,
		withdrawal.Sum,
	).Scan(&withdrawal.ID, &withdrawal.ProcessedAt)
	if err != nil {
		return fmt.Errorf("failed to create withdrawal record: %w", err)
	}

	// Фиксируем транзакцию
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetByUserID возвращает баланс пользователя
func (r *BalanceRepository) GetByUserID(ctx context.Context, userID pgtype.UUID) (*models.Balance, error) {
	query := `
        SELECT user_id, current, withdrawn
        FROM balances
        WHERE user_id = $1
    `

	var balance models.Balance
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&balance.UserID,
		&balance.Current,
		&balance.Withdrawn,
	)

	if err == pgx.ErrNoRows {
		// Если баланса нет, создаем новый
		return r.Create(ctx, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return &balance, nil
}

// Create создает новый баланс для пользователя
func (r *BalanceRepository) Create(ctx context.Context, userID pgtype.UUID) (*models.Balance, error) {
	query := `
        INSERT INTO balances (user_id, current, withdrawn)
        VALUES ($1, 0, 0)
        RETURNING user_id, current, withdrawn
    `

	var balance models.Balance
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&balance.UserID,
		&balance.Current,
		&balance.Withdrawn,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create balance: %w", err)
	}

	return &balance, nil
}

// AddAccrual добавляет начисление на баланс пользователя
func (r *BalanceRepository) AddAccrual(ctx context.Context, userID pgtype.UUID, amount float64) error {
	query := `
        UPDATE balances
        SET current = current + $1
        WHERE user_id = $2
    `

	result, err := r.db.Exec(ctx, query, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to add accrual: %w", err)
	}

	if result.RowsAffected() == 0 {
		// Если баланса нет, создаем и пробуем еще раз
		if _, err := r.Create(ctx, userID); err != nil {
			return fmt.Errorf("failed to create balance: %w", err)
		}
		// Повторяем обновление
		result, err = r.db.Exec(ctx, query, amount, userID)
		if err != nil {
			return fmt.Errorf("failed to add accrual after creation: %w", err)
		}
		if result.RowsAffected() == 0 {
			return fmt.Errorf("failed to add accrual: no rows affected")
		}
	}

	return nil
}

// Withdraw - старый метод, оставляем для совместимости, но используем новый
func (r *BalanceRepository) Withdraw(ctx context.Context, userID pgtype.UUID, amount float64) error {
	query := `
        UPDATE balances
        SET current = current - $1,
            withdrawn = withdrawn + $1
        WHERE user_id = $2 AND current >= $1
    `

	result, err := r.db.Exec(ctx, query, amount, userID)
	if err != nil {
		return fmt.Errorf("failed to withdraw: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("insufficient funds or user not found")
	}

	return nil
}

// Update обновляет баланс
func (r *BalanceRepository) Update(ctx context.Context, balance *models.Balance) error {
	query := `
        UPDATE balances
        SET current = $1, withdrawn = $2
        WHERE user_id = $3
    `

	result, err := r.db.Exec(ctx, query, balance.Current, balance.Withdrawn, balance.UserID)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("balance not found for user")
	}

	return nil
}

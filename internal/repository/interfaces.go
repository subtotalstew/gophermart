package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/subtotalstew/gophermart/internal/models"
)

// UserRepositoryInterface определяет методы для работы с пользователями
type UserRepositoryInterface interface {
	Create(ctx context.Context, user *models.User) error
	FindByLogin(ctx context.Context, login string) (*models.User, error)
	UserExists(ctx context.Context, login string) (bool, error)
}

// OrderRepositoryInterface определяет методы для работы с заказами
type OrderRepositoryInterface interface {
	Save(ctx context.Context, order *models.Order) error
	FindByNumber(ctx context.Context, number string) (*models.Order, error)
	FindByUserID(ctx context.Context, userID pgtype.UUID) ([]models.Order, error)
	UpdateStatus(ctx context.Context, number string, status string) error
	UpdateStatusAndAccrual(ctx context.Context, number string, status string, accrual *float64) error
	GetOrdersByStatus(ctx context.Context, statuses []string) ([]models.Order, error)
	OrderExists(ctx context.Context, number string) (bool, error)
	GetOrderOwner(ctx context.Context, number string) (*pgtype.UUID, error)
	SaveWithLock(ctx context.Context, order *models.Order) error
}

// BalanceRepositoryInterface определяет методы для работы с балансом
type BalanceRepositoryInterface interface {
	GetByUserID(ctx context.Context, userID pgtype.UUID) (*models.Balance, error)
	Create(ctx context.Context, userID pgtype.UUID) (*models.Balance, error)
	AddAccrual(ctx context.Context, userID pgtype.UUID, amount float64) error
	Withdraw(ctx context.Context, userID pgtype.UUID, amount float64) error
	WithdrawInTransaction(ctx context.Context, withdrawal *models.Withdrawal) error // НОВЫЙ МЕТОД
	Update(ctx context.Context, balance *models.Balance) error
}

// WithdrawalRepositoryInterface определяет методы для работы со списаниями
type WithdrawalRepositoryInterface interface {
	Create(ctx context.Context, withdrawal *models.Withdrawal) error
	GetByUserID(ctx context.Context, userID pgtype.UUID) ([]models.Withdrawal, error)
	GetTotalWithdrawn(ctx context.Context, userID pgtype.UUID) (float64, error)
}

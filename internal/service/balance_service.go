package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/repository"
	"github.com/subtotalstew/gophermart/internal/utils"
)

var (
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidWithdrawOrder = errors.New("invalid order number for withdrawal")
)

// BalanceService - сервис для работы с балансом
type BalanceService struct {
	balanceRepo    repository.BalanceRepositoryInterface
	withdrawalRepo repository.WithdrawalRepositoryInterface
	orderRepo      repository.OrderRepositoryInterface
}

// NewBalanceService создает новый экземпляр сервиса баланса
func NewBalanceService(
	balanceRepo repository.BalanceRepositoryInterface,
	withdrawalRepo repository.WithdrawalRepositoryInterface,
	orderRepo repository.OrderRepositoryInterface,
) *BalanceService {
	return &BalanceService{
		balanceRepo:    balanceRepo,
		withdrawalRepo: withdrawalRepo,
		orderRepo:      orderRepo,
	}
}

// GetBalance возвращает баланс пользователя
func (s *BalanceService) GetBalance(ctx context.Context, userID pgtype.UUID) (*models.Balance, error) {
	balance, err := s.balanceRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}
	return balance, nil
}

// AddAccrual добавляет начисление на баланс пользователя
func (s *BalanceService) AddAccrual(ctx context.Context, userID pgtype.UUID, amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("accrual amount must be positive")
	}

	if err := s.balanceRepo.AddAccrual(ctx, userID, amount); err != nil {
		return fmt.Errorf("failed to add accrual: %w", err)
	}

	return nil
}

// Withdraw списывает средства с баланса пользователя
func (s *BalanceService) Withdraw(ctx context.Context, userID pgtype.UUID, orderNumber string, amount float64) error {
	// 1. Валидация номера заказа
	normalizedNumber, err := utils.ValidateAndNormalizeOrderNumber(orderNumber)
	if err != nil {
		return ErrInvalidWithdrawOrder
	}

	// 2. Проверяем, что заказ не существует
	exists, err := s.orderRepo.OrderExists(ctx, normalizedNumber)
	if err != nil {
		return fmt.Errorf("failed to check order existence: %w", err)
	}
	if exists {
		// Заказ уже существует, проверяем владельца
		owner, err := s.orderRepo.GetOrderOwner(ctx, normalizedNumber)
		if err != nil {
			return fmt.Errorf("failed to get order owner: %w", err)
		}
		if owner != nil && owner.Bytes == userID.Bytes {
			// Это заказ пользователя, его нельзя использовать для списания
			return ErrInvalidWithdrawOrder
		}
		// Заказ принадлежит другому пользователю - тоже нельзя использовать
		return ErrInvalidWithdrawOrder
	}

	// 3. Проверяем достаточность средств (до транзакции)
	balance, err := s.balanceRepo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}
	if balance.Current < amount {
		return ErrInsufficientFunds
	}

	// Используем транзакцию
	withdrawal := &models.Withdrawal{
		UserID:      userID,
		OrderNumber: normalizedNumber,
		Sum:         amount,
	}

	if err := s.balanceRepo.WithdrawInTransaction(ctx, withdrawal); err != nil {
		return fmt.Errorf("failed to withdraw: %w", err)
	}

	return nil
}

// GetWithdrawals возвращает историю списаний пользователя
func (s *BalanceService) GetWithdrawals(ctx context.Context, userID pgtype.UUID) ([]models.Withdrawal, error) {
	withdrawals, err := s.withdrawalRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get withdrawals: %w", err)
	}
	return withdrawals, nil
}

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
	ErrOrderAlreadyUploadedByUser  = errors.New("order already uploaded by this user")
	ErrOrderAlreadyUploadedByOther = errors.New("order already uploaded by another user")
	ErrOrderNotFound               = errors.New("order not found")
	ErrInvalidOrderNumber          = errors.New("invalid order number")
	ErrEmptyOrderNumber            = errors.New("empty order number")
)

type OrderService struct {
	orderRepo repository.OrderRepositoryInterface
	userRepo  repository.UserRepositoryInterface
}

func NewOrderService(orderRepo repository.OrderRepositoryInterface, userRepo repository.UserRepositoryInterface) *OrderService {
	return &OrderService{
		orderRepo: orderRepo,
		userRepo:  userRepo,
	}
}

// UploadOrder загружает новый заказ
func (s *OrderService) UploadOrder(ctx context.Context, userID pgtype.UUID, number string) (*models.Order, error) {
	// 1. Валидация номера заказа
	normalizedNumber, err := utils.ValidateAndNormalizeOrderNumber(number)
	if err != nil {
		if errors.Is(err, utils.ErrEmptyOrderNumber) {
			return nil, ErrEmptyOrderNumber
		}
		return nil, ErrInvalidOrderNumber
	}

	// 2. Проверяем, существует ли заказ в системе
	exists, err := s.orderRepo.OrderExists(ctx, normalizedNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to check order existence: %w", err)
	}

	if exists {
		// Заказ существует, проверяем владельца
		ownerID, err := s.orderRepo.GetOrderOwner(ctx, normalizedNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to get order owner: %w", err)
		}

		if ownerID == nil {
			return nil, fmt.Errorf("order exists but owner not found")
		}

		// Сравниваем владельца с текущим пользователем
		if ownerID.Bytes != userID.Bytes {
			return nil, ErrOrderAlreadyUploadedByOther
		}

		// Заказ принадлежит этому пользователю
		order, err := s.orderRepo.FindByNumber(ctx, normalizedNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to find order: %w", err)
		}

		return order, ErrOrderAlreadyUploadedByUser
	}

	// 3. Создаем новый заказ
	order := &models.Order{
		Number:  normalizedNumber,
		UserID:  userID,
		Status:  models.OrderStatusNew,
		Accrual: nil,
	}

	if err := s.orderRepo.Save(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to save order: %w", err)
	}

	return order, nil
}

// GetUserOrders возвращает все заказы пользователя
func (s *OrderService) GetUserOrders(ctx context.Context, userID pgtype.UUID) ([]models.Order, error) {
	orders, err := s.orderRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user orders: %w", err)
	}
	return orders, nil
}

// GetOrderByNumber возвращает заказ по номеру
func (s *OrderService) GetOrderByNumber(ctx context.Context, number string) (*models.Order, error) {
	order, err := s.orderRepo.FindByNumber(ctx, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	return order, nil
}

// UpdateOrderStatus обновляет статус заказа
func (s *OrderService) UpdateOrderStatus(ctx context.Context, number string, status string) error {
	if err := s.orderRepo.UpdateStatus(ctx, number, status); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	return nil
}

// UpdateOrderStatusAndAccrual обновляет статус и начисление
func (s *OrderService) UpdateOrderStatusAndAccrual(ctx context.Context, number string, status string, accrual *float64) error {
	if err := s.orderRepo.UpdateStatusAndAccrual(ctx, number, status, accrual); err != nil {
		return fmt.Errorf("failed to update order status and accrual: %w", err)
	}
	return nil
}

// GetOrdersForProcessing возвращает заказы, требующие обработки
func (s *OrderService) GetOrdersForProcessing(ctx context.Context) ([]models.Order, error) {
	statuses := []string{
		models.OrderStatusNew,
		models.OrderStatusProcessing,
	}
	return s.orderRepo.GetOrdersByStatus(ctx, statuses)
}

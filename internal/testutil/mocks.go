package testutil

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/subtotalstew/gophermart/internal/models"
)

// MockUserRepository - мок для пользователей (реализует UserRepositoryInterface)
type MockUserRepository struct {
	mu      sync.RWMutex
	users   map[string]*models.User // login -> user
	idIndex map[string]*models.User // id -> user
	err     error
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users:   make(map[string]*models.User),
		idIndex: make(map[string]*models.User),
	}
}

func (m *MockUserRepository) SetError(err error) {
	m.err = err
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	if m.err != nil {
		return m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[user.Login]; exists {
		return errors.New("user already exists")
	}

	// Генерируем ID если его нет
	var emptyUUID pgtype.UUID
	if user.ID == emptyUUID {
		var uuid pgtype.UUID
		uuid.Scan("00000000-0000-0000-0000-000000000001")
		user.ID = uuid
	}

	user.CreatedAt = time.Now()
	m.users[user.Login] = user
	m.idIndex[user.ID.String()] = user
	return nil
}

func (m *MockUserRepository) FindByLogin(ctx context.Context, login string) (*models.User, error) {
	if m.err != nil {
		return nil, m.err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[login]
	if !exists {
		return nil, nil
	}
	return user, nil
}

func (m *MockUserRepository) UserExists(ctx context.Context, login string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.users[login]
	return exists, nil
}

func (m *MockUserRepository) GetUserByID(id string) *models.User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.idIndex[id]
}

// MockOrderRepository - мок для заказов (реализует OrderRepositoryInterface)
type MockOrderRepository struct {
	mu         sync.RWMutex
	orders     map[string]*models.Order // number -> order
	userOrders map[string][]string      // userID -> []orderNumbers
	lastError  error
}

func NewMockOrderRepository() *MockOrderRepository {
	return &MockOrderRepository{
		orders:     make(map[string]*models.Order),
		userOrders: make(map[string][]string),
	}
}

func (m *MockOrderRepository) SetError(err error) {
	m.lastError = err
}

func (m *MockOrderRepository) Save(ctx context.Context, order *models.Order) error {
	if m.lastError != nil {
		return m.lastError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Проверяем, существует ли уже заказ
	if _, exists := m.orders[order.Number]; exists {
		return errors.New("order already exists")
	}

	order.UploadedAt = time.Now()
	order.UpdatedAt = time.Now()
	m.orders[order.Number] = order

	userID := order.UserID.String()
	m.userOrders[userID] = append(m.userOrders[userID], order.Number)

	return nil
}

func (m *MockOrderRepository) FindByNumber(ctx context.Context, number string) (*models.Order, error) {
	if m.lastError != nil {
		return nil, m.lastError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	order, exists := m.orders[number]
	if !exists {
		return nil, nil
	}
	return order, nil
}

func (m *MockOrderRepository) FindByUserID(ctx context.Context, userID pgtype.UUID) ([]models.Order, error) {
	if m.lastError != nil {
		return nil, m.lastError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	id := userID.String()
	numbers, exists := m.userOrders[id]
	if !exists || len(numbers) == 0 {
		return []models.Order{}, nil
	}

	orders := make([]models.Order, 0, len(numbers))
	for _, num := range numbers {
		if order, ok := m.orders[num]; ok {
			orders = append(orders, *order)
		}
	}
	return orders, nil
}

func (m *MockOrderRepository) UpdateStatus(ctx context.Context, number string, status string) error {
	if m.lastError != nil {
		return m.lastError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	order, exists := m.orders[number]
	if !exists {
		return errors.New("order not found")
	}

	order.Status = status
	order.UpdatedAt = time.Now()
	return nil
}

func (m *MockOrderRepository) UpdateStatusAndAccrual(ctx context.Context, number string, status string, accrual *float64) error {
	if m.lastError != nil {
		return m.lastError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	order, exists := m.orders[number]
	if !exists {
		return errors.New("order not found")
	}

	order.Status = status
	order.Accrual = accrual
	order.UpdatedAt = time.Now()
	return nil
}

func (m *MockOrderRepository) GetOrdersByStatus(ctx context.Context, statuses []string) ([]models.Order, error) {
	if m.lastError != nil {
		return nil, m.lastError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	statusMap := make(map[string]bool)
	for _, s := range statuses {
		statusMap[s] = true
	}

	var result []models.Order
	for _, order := range m.orders {
		if statusMap[order.Status] {
			result = append(result, *order)
		}
	}
	return result, nil
}

func (m *MockOrderRepository) OrderExists(ctx context.Context, number string) (bool, error) {
	if m.lastError != nil {
		return false, m.lastError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.orders[number]
	return exists, nil
}

func (m *MockOrderRepository) GetOrderOwner(ctx context.Context, number string) (*pgtype.UUID, error) {
	if m.lastError != nil {
		return nil, m.lastError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	order, exists := m.orders[number]
	if !exists {
		return nil, nil
	}

	return &order.UserID, nil
}

// Вспомогательные методы для тестов
func (m *MockOrderRepository) AddOrder(order *models.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order.UploadedAt = time.Now()
	order.UpdatedAt = time.Now()
	m.orders[order.Number] = order

	userID := order.UserID.String()
	m.userOrders[userID] = append(m.userOrders[userID], order.Number)
}

func (m *MockOrderRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.orders = make(map[string]*models.Order)
	m.userOrders = make(map[string][]string)
	m.lastError = nil
}

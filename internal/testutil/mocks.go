package testutil

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/repository"
)

// ============================================
// MockUserRepository - мок для пользователей
// ============================================

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

func (m *MockUserRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users = make(map[string]*models.User)
	m.idIndex = make(map[string]*models.User)
	m.err = nil
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	if m.err != nil {
		return m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[user.Login]; exists {
		return repository.ErrUserExists // было: errors.New("user already exists")
	}

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

// ============================================
// MockOrderRepository - мок для заказов
// ============================================

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

func (m *MockOrderRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders = make(map[string]*models.Order)
	m.userOrders = make(map[string][]string)
	m.lastError = nil
}

func (m *MockOrderRepository) Save(ctx context.Context, order *models.Order) error {
	if m.lastError != nil {
		return m.lastError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

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

func (m *MockOrderRepository) AddOrder(order *models.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()

	order.UploadedAt = time.Now()
	order.UpdatedAt = time.Now()
	m.orders[order.Number] = order

	userID := order.UserID.String()
	m.userOrders[userID] = append(m.userOrders[userID], order.Number)
}

func (m *MockOrderRepository) SaveWithLock(ctx context.Context, order *models.Order) error {
	if m.lastError != nil {
		return m.lastError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Проверяем, существует ли уже заказ
	if _, exists := m.orders[order.Number]; exists {
		return repository.ErrOrderExists // было: errors.New("order already exists")
	}

	order.UploadedAt = time.Now()
	order.UpdatedAt = time.Now()
	m.orders[order.Number] = order

	userID := order.UserID.String()
	m.userOrders[userID] = append(m.userOrders[userID], order.Number)

	return nil
}

// ============================================
// MockBalanceRepository - мок для баланса
// ============================================

type MockBalanceRepository struct {
	mu       sync.RWMutex
	balances map[string]*models.Balance // userID -> balance
	err      error
}

func NewMockBalanceRepository() *MockBalanceRepository {
	return &MockBalanceRepository{
		balances: make(map[string]*models.Balance),
	}
}

func (m *MockBalanceRepository) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func (m *MockBalanceRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.balances = make(map[string]*models.Balance)
	m.err = nil
}

func (m *MockBalanceRepository) GetByUserID(ctx context.Context, userID pgtype.UUID) (*models.Balance, error) {
	if m.err != nil {
		return nil, m.err
	}

	m.mu.RLock()
	balance, exists := m.balances[userID.String()]
	m.mu.RUnlock()

	if !exists {
		// Если баланса нет, создаем новый (без блокировки)
		return m.Create(ctx, userID)
	}
	return balance, nil
}

func (m *MockBalanceRepository) Create(ctx context.Context, userID pgtype.UUID) (*models.Balance, error) {
	if m.err != nil {
		return nil, m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Проверяем еще раз после получения блокировки
	if balance, exists := m.balances[userID.String()]; exists {
		return balance, nil
	}

	balance := &models.Balance{
		UserID:    userID,
		Current:   0,
		Withdrawn: 0,
	}
	m.balances[userID.String()] = balance
	return balance, nil
}

func (m *MockBalanceRepository) AddAccrual(ctx context.Context, userID pgtype.UUID, amount float64) error {
	if m.err != nil {
		return m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	balance, exists := m.balances[userID.String()]
	if !exists {
		balance = &models.Balance{
			UserID:    userID,
			Current:   0,
			Withdrawn: 0,
		}
		m.balances[userID.String()] = balance
	}
	balance.Current += amount
	return nil
}
func (m *MockBalanceRepository) WithdrawInTransaction(ctx context.Context, withdrawal *models.Withdrawal) error {
	if m.err != nil {
		return m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Проверяем существование баланса
	balance, exists := m.balances[withdrawal.UserID.String()]
	if !exists {
		return fmt.Errorf("balance not found")
	}

	// Проверяем достаточность средств
	if balance.Current < withdrawal.Sum {
		return fmt.Errorf("insufficient funds")
	}

	// Списываем средства
	balance.Current -= withdrawal.Sum
	balance.Withdrawn += withdrawal.Sum

	// Создаем запись о списании
	// В моке просто сохраняем в отдельную структуру
	// Для простоты используем существующее поле Withdrawal.ID как индикатор
	withdrawal.ID = pgtype.UUID{}
	withdrawal.ID.Scan(fmt.Sprintf("%d", len(m.balances)))
	withdrawal.ProcessedAt = time.Now()

	return nil
}

func (m *MockBalanceRepository) Withdraw(ctx context.Context, userID pgtype.UUID, amount float64) error {
	if m.err != nil {
		return m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	balance, exists := m.balances[userID.String()]
	if !exists {
		return fmt.Errorf("balance not found")
	}
	if balance.Current < amount {
		return fmt.Errorf("insufficient funds")
	}
	balance.Current -= amount
	balance.Withdrawn += amount
	return nil
}

func (m *MockBalanceRepository) Update(ctx context.Context, balance *models.Balance) error {
	if m.err != nil {
		return m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.balances[balance.UserID.String()] = balance
	return nil
}

func (m *MockBalanceRepository) AddBalance(userID pgtype.UUID, amount float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	balance, exists := m.balances[userID.String()]
	if !exists {
		balance = &models.Balance{
			UserID:    userID,
			Current:   0,
			Withdrawn: 0,
		}
		m.balances[userID.String()] = balance
	}
	balance.Current += amount
}

// ============================================
// MockWithdrawalRepository - мок для списаний
// ============================================

type MockWithdrawalRepository struct {
	mu          sync.RWMutex
	withdrawals map[string][]*models.Withdrawal // userID -> []*withdrawals
	err         error
}

func NewMockWithdrawalRepository() *MockWithdrawalRepository {
	return &MockWithdrawalRepository{
		withdrawals: make(map[string][]*models.Withdrawal),
	}
}

func (m *MockWithdrawalRepository) SetError(err error) {
	m.err = err
}

func (m *MockWithdrawalRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.withdrawals = make(map[string][]*models.Withdrawal)
	m.err = nil
}

func (m *MockWithdrawalRepository) Create(ctx context.Context, withdrawal *models.Withdrawal) error {
	if m.err != nil {
		return m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if withdrawal.ProcessedAt.IsZero() {
		withdrawal.ProcessedAt = time.Now()
	}
	userID := withdrawal.UserID.String()
	m.withdrawals[userID] = append(m.withdrawals[userID], withdrawal)
	return nil
}

func (m *MockWithdrawalRepository) CreateWithTime(ctx context.Context, withdrawal *models.Withdrawal) error {
	if m.err != nil {
		return m.err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	userID := withdrawal.UserID.String()
	m.withdrawals[userID] = append(m.withdrawals[userID], withdrawal)
	return nil
}

func (m *MockWithdrawalRepository) GetByUserID(ctx context.Context, userID pgtype.UUID) ([]models.Withdrawal, error) {
	if m.err != nil {
		return nil, m.err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	withdrawals, exists := m.withdrawals[userID.String()]
	if !exists || len(withdrawals) == 0 {
		return []models.Withdrawal{}, nil
	}

	// Конвертируем []*models.Withdrawal в []models.Withdrawal
	result := make([]models.Withdrawal, len(withdrawals))
	for i, w := range withdrawals {
		result[i] = *w
	}
	return result, nil
}

func (m *MockWithdrawalRepository) GetTotalWithdrawn(ctx context.Context, userID pgtype.UUID) (float64, error) {
	if m.err != nil {
		return 0, m.err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	withdrawals, exists := m.withdrawals[userID.String()]
	if !exists || len(withdrawals) == 0 {
		return 0, nil
	}

	var total float64
	for _, w := range withdrawals {
		total += w.Sum
	}
	return total, nil
}

var (
	_ repository.UserRepositoryInterface       = (*MockUserRepository)(nil)
	_ repository.OrderRepositoryInterface      = (*MockOrderRepository)(nil)
	_ repository.BalanceRepositoryInterface    = (*MockBalanceRepository)(nil)
	_ repository.WithdrawalRepositoryInterface = (*MockWithdrawalRepository)(nil)
)

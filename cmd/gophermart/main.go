package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/subtotalstew/gophermart/internal/accrual"
	"github.com/subtotalstew/gophermart/internal/config"
	"github.com/subtotalstew/gophermart/internal/database"
	"github.com/subtotalstew/gophermart/internal/handlers"
	"github.com/subtotalstew/gophermart/internal/middleware"
	"github.com/subtotalstew/gophermart/internal/repository"
	"github.com/subtotalstew/gophermart/internal/service"
	"github.com/subtotalstew/gophermart/internal/worker"
)

func main() {
	// 1. Загрузка конфигурации
	cfg := config.Load()

	// 2. Настройка логирования
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 3. Подключение к БД
	db, err := database.Connect(cfg.DatabaseURI)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 4. Выполнение миграций
	if err := database.RunMigrations(cfg.DatabaseURI); err != nil {
		slog.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("Migrations completed successfully")

	// 5. Инициализация репозиториев
	userRepo := repository.NewUserRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	balanceRepo := repository.NewBalanceRepository(db)
	withdrawalRepo := repository.NewWithdrawalRepository(db)

	// 6. Инициализация сервисов
	userService := service.NewUserService(userRepo)
	orderService := service.NewOrderService(orderRepo, userRepo)
	balanceService := service.NewBalanceService(balanceRepo, withdrawalRepo, orderRepo)

	// 7. Инициализация клиента Accrual System
	var accrualClient *accrual.Client
	if cfg.AccrualSystemAddress != "" {
		accrualClient = accrual.NewClient(cfg.AccrualSystemAddress)
		slog.Info("Accrual system client initialized", "address", cfg.AccrualSystemAddress)
	} else {
		slog.Warn("Accrual system address not configured, orders will not be processed automatically")
	}

	// 8. Инициализация воркера
	var orderProcessor *worker.OrderProcessor
	if accrualClient != nil {
		orderProcessor = worker.NewOrderProcessor(
			orderService,
			balanceService,
			accrualClient,
			10*time.Second,
		)
	}

	// 9. Инициализация middleware
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret)

	// 10. Инициализация хендлеров
	userHandler := handlers.NewUserHandler(userService, authMiddleware)
	orderHandler := handlers.NewOrderHandler(orderService)
	balanceHandler := handlers.NewBalanceHandler(balanceService, authMiddleware) // <-- ДОБАВЛЯЕМ

	// 11. Настройка роутера
	mux := http.NewServeMux()

	// Публичные маршруты
	mux.HandleFunc("POST /api/user/register", userHandler.Register)
	mux.HandleFunc("POST /api/user/login", userHandler.Login)

	// Защищенные маршруты
	mux.Handle("POST /api/user/orders", authMiddleware.RequireAuth(http.HandlerFunc(orderHandler.UploadOrder)))
	mux.Handle("GET /api/user/orders", authMiddleware.RequireAuth(http.HandlerFunc(orderHandler.GetOrders)))
	mux.Handle("GET /api/user/balance", authMiddleware.RequireAuth(http.HandlerFunc(balanceHandler.GetBalance)))
	mux.Handle("POST /api/user/balance/withdraw", authMiddleware.RequireAuth(http.HandlerFunc(balanceHandler.Withdraw)))
	mux.Handle("GET /api/user/withdrawals", authMiddleware.RequireAuth(http.HandlerFunc(balanceHandler.GetWithdrawals)))

	// 12. Сборка middleware
	handler := middleware.Logging(mux)
	handler = middleware.Gzip(handler)

	// 13. Запуск воркера
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if orderProcessor != nil {
		go orderProcessor.Run(ctx)
		slog.Info("Order processor started")
	}

	// 14. Запуск сервера
	server := &http.Server{
		Addr:         cfg.RunAddress,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Server starting", "address", cfg.RunAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	// 15. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	// Останавливаем воркер
	if orderProcessor != nil {
		orderProcessor.Stop()
		slog.Info("Order processor stopped")
	}

	// Останавливаем сервер
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server exited properly")
}

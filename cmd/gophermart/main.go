package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/subtotalstew/gophermart/internal/config"
	"github.com/subtotalstew/gophermart/internal/database"
	"github.com/subtotalstew/gophermart/internal/handlers"
	"github.com/subtotalstew/gophermart/internal/middleware"
	"github.com/subtotalstew/gophermart/internal/repository"
	"github.com/subtotalstew/gophermart/internal/service"
)

func main() {
	cfg := config.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	db, err := database.Connect(cfg.DatabaseURI)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.RunMigrations(cfg.DatabaseURI); err != nil {
		slog.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(db)

	userService := service.NewUserService(userRepo)
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret)

	userHandler := handlers.NewUserHandler(userService, authMiddleware)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/user/register", userHandler.Register)
	mux.HandleFunc("POST /api/user/login", userHandler.Login)

	mux.Handle("GET /api/user/orders", authMiddleware.RequireAuth(http.HandlerFunc(userHandler.GetOrders)))
	mux.Handle("POST /api/user/orders", authMiddleware.RequireAuth(http.HandlerFunc(userHandler.UploadOrder)))
	mux.Handle("GET /api/user/balance", authMiddleware.RequireAuth(http.HandlerFunc(userHandler.GetBalance)))
	mux.Handle("POST /api/user/balance/withdraw", authMiddleware.RequireAuth(http.HandlerFunc(userHandler.Withdraw)))
	mux.Handle("GET /api/user/withdrawals", authMiddleware.RequireAuth(http.HandlerFunc(userHandler.GetWithdrawals)))

	handler := middleware.Logging(mux)
	handler = middleware.Gzip(handler)

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server exited properly")
}

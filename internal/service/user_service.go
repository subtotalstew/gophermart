package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/subtotalstew/gophermart/internal/models"
	"github.com/subtotalstew/gophermart/internal/repository"
	"github.com/subtotalstew/gophermart/internal/utils"
)

var (
	ErrUserExists   = errors.New("user already exists")
	ErrInvalidLogin = errors.New("invalid login or password")
)

type UserService struct {
	repo repository.UserRepositoryInterface
}

func NewUserService(repo repository.UserRepositoryInterface) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Register(ctx context.Context, login, password string) (*models.User, error) {
	exists, err := s.repo.UserExists(ctx, login)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	if exists {
		return nil, ErrUserExists
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		Login:        login,
		PasswordHash: string(hash),
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func (s *UserService) Login(ctx context.Context, login, password string) (*models.User, error) {
	user, err := s.repo.FindByLogin(ctx, login)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidLogin
	}

	if !utils.CheckPasswordHash(password, user.PasswordHash) {
		return nil, ErrInvalidLogin
	}

	return user, nil
}

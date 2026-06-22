package repositories

import (
	"context"

	"github.com/stretchr/testify/mock"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

type MockUsersQuerier struct {
	mock.Mock
}

func (m *MockUsersQuerier) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersQuerier) FindUserByID(ctx context.Context, id string) (db.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersQuerier) FindUserByGoogleID(ctx context.Context, googleID string) (db.User, error) {
	args := m.Called(ctx, googleID)
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersQuerier) UpdateUserLastLogin(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUsersQuerier) SoftDeleteUser(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUsersQuerier) CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.RefreshToken), args.Error(1)
}

func (m *MockUsersQuerier) ExtendRefreshToken(ctx context.Context, arg db.ExtendRefreshTokenParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockUsersQuerier) FindActiveRefreshTokenByUserID(ctx context.Context, userID string) (db.RefreshToken, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(db.RefreshToken), args.Error(1)
}

func (m *MockUsersQuerier) FindRefreshTokenByID(ctx context.Context, id string) (db.RefreshToken, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(db.RefreshToken), args.Error(1)
}

func (m *MockUsersQuerier) RevokeAllRefreshTokensByUserID(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockUsersQuerier) RevokeRefreshToken(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

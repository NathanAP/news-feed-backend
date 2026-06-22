package repositories

import (
	"context"

	"github.com/stretchr/testify/mock"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

type MockRefreshTokensQuerier struct {
	mock.Mock
}

func (m *MockRefreshTokensQuerier) CreateRefreshToken(ctx context.Context, arg db.CreateRefreshTokenParams) (db.RefreshToken, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokensQuerier) FindRefreshTokenByID(ctx context.Context, id string) (db.RefreshToken, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(db.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokensQuerier) FindActiveRefreshTokenByUserID(ctx context.Context, userID string) (db.RefreshToken, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(db.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokensQuerier) ExtendRefreshToken(ctx context.Context, arg db.ExtendRefreshTokenParams) error {
	args := m.Called(ctx, arg)
	return args.Error(0)
}

func (m *MockRefreshTokensQuerier) RevokeRefreshToken(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRefreshTokensQuerier) RevokeAllRefreshTokensByUserID(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRefreshTokensQuerier) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockRefreshTokensQuerier) FindUserByID(ctx context.Context, id string) (db.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockRefreshTokensQuerier) FindUserByGoogleID(ctx context.Context, googleID string) (db.User, error) {
	args := m.Called(ctx, googleID)
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockRefreshTokensQuerier) UpdateUserLastLogin(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRefreshTokensQuerier) SoftDeleteUser(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

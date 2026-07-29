package services

import (
	"context"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	"github.com/nathanap/news-feed-backend/tests/fixtures"
)

// MockUserController implements controllers.UserControllerInterface for unit tests.
//
// It lives here, shared, instead of being copied into each unit package like the other controller
// mocks: the admin middleware is mounted in four different route groups, so four hand-written copies
// of the same six methods would be four places to forget when the interface changes. The other mocks
// stay local because each test package configures them differently; this one is only ever asked one
// question — "is this user an administrator?".
type MockUserController struct {
	// Admin is the flag the default FindUserByID reports. It is what the admin middleware reads, so
	// it is effectively the switch between "this request is authorized" and "403".
	Admin bool
	// FindUserByIDFn overrides the lookup entirely, for the cases the flag cannot express — mainly
	// returning an infrastructure error to check that it surfaces as 500 rather than as a denial.
	FindUserByIDFn func(ctx context.Context, q db.Querier, id string) (db.User, error)
}

func (m *MockUserController) CreateUser(_ context.Context, _ db.Querier, googleID, email, name, _ string) (db.User, error) {
	user := fixtures.NewTestUser()
	user.GoogleID, user.Email, user.Name = googleID, email, name
	return user, nil
}

func (m *MockUserController) FindUserByID(ctx context.Context, q db.Querier, id string) (db.User, error) {
	if m.FindUserByIDFn != nil {
		return m.FindUserByIDFn(ctx, q, id)
	}
	user := fixtures.NewTestUser()
	user.ID = id
	user.Admin = m.Admin
	return user, nil
}

func (m *MockUserController) FindUserByGoogleID(_ context.Context, _ db.Querier, googleID string) (db.User, error) {
	user := fixtures.NewTestUser()
	user.GoogleID = googleID
	user.Admin = m.Admin
	return user, nil
}

func (m *MockUserController) UpdateUserLastLogin(_ context.Context, _ db.Querier, _ string) error {
	return nil
}

func (m *MockUserController) SetAdmin(_ context.Context, _ db.Querier, id string, admin bool) (db.User, error) {
	user := fixtures.NewTestUser()
	user.ID = id
	user.Admin = admin
	return user, nil
}

func (m *MockUserController) SoftDeleteUser(_ context.Context, _ db.Querier, _ string) error {
	return nil
}

var _ controllers.UserControllerInterface = (*MockUserController)(nil)

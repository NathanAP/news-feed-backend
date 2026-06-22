package external

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

type GoogleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type MockGoogleOAuth struct {
	UserInfo      GoogleUserInfo
	ExchangeError error
	ClientError   error
}

func (m *MockGoogleOAuth) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	if m.ExchangeError != nil {
		return nil, m.ExchangeError
	}
	return &oauth2.Token{AccessToken: "mock-google-access-token"}, nil
}

func (m *MockGoogleOAuth) Client(ctx context.Context, t *oauth2.Token) *http.Client {
	return &http.Client{Transport: &mockRoundTripper{
		userInfo:    m.UserInfo,
		returnError: m.ClientError,
	}}
}

type mockRoundTripper struct {
	userInfo    GoogleUserInfo
	returnError error
}

func (t *mockRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	if t.returnError != nil {
		return nil, t.returnError
	}
	body, _ := json.Marshal(t.userInfo)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

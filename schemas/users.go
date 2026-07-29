package schemas

import "time"

type UserResponse struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture,omitempty"`
	// Admin mirrors the token claim, so the client can render the admin UI without decoding the JWT
	// itself. Like the claim, it is a hint: authorization is decided against the database on every
	// admin request. No omitempty — the client must be able to tell "regular user" from "field absent".
	Admin     bool      `json:"admin"`
	CreatedAt time.Time `json:"created_at"`
}

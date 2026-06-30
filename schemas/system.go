package schemas

import "time"

// UpdateAppStatusRequest carries the maintenance switch value. AppStatus is a pointer so the
// handler can tell "field omitted" (nil → 400) apart from an explicit false.
type UpdateAppStatusRequest struct {
	AppStatus *bool `json:"app_status"`
}

type SystemResponse struct {
	ID                     string     `json:"id"`
	AppStatus              bool       `json:"app_status"`
	LastArticleDiscoveryAt *time.Time `json:"last_article_discovery_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	ModifiedAt             *time.Time `json:"modified_at,omitempty"`
}

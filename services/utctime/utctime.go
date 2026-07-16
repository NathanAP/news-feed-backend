// Package utctime carries the project's UTC rule (conventions.md: "todas as datas devem ser tratadas
// como UTC nesta aplicação") in the type system instead of in a runtime global.
//
// The problem it solves: the database column is TIMESTAMPTZ, which stores an instant, but pgx hands a
// scanned value back as a time.Time in time.Local. On a machine set to -03:00 the API would therefore
// serialize `2026-07-16T15:01:13-03:00` instead of `...Z` — the right instant in a representation that
// contradicts the convention and, worse, that changes with the host's clock configuration. A server in
// UTC would hide the bug entirely, which is what makes it dangerous: it would surface only when
// someone deployed to a host with another timezone.
//
// Pinning time.Local to UTC in main() fixed it, but that is a global mutation whose guarantee is
// invisible at the point of use, and every new entrypoint has to remember it (cmd/seed already had to).
// Here the guarantee is attached to the value: whatever the process's local zone is, a Time scanned
// from the database is in UTC, and it serializes with Z.
package utctime

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Time is a TIMESTAMPTZ that is always in UTC. It is wired to the timestamptz columns through the
// `overrides` in sqlc.yaml, so every generated model and query row uses it.
type Time struct {
	time.Time
}

// New converts a time.Time into a Time, normalizing to UTC.
func New(t time.Time) Time {
	return Time{Time: t.UTC()}
}

// Scan implements sql.Scanner. It is the whole point of the type: the driver may hand back a value in
// any location, and this is where it becomes UTC — once, at the boundary, rather than at each caller.
func (t *Time) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		t.Time = v.UTC()
		return nil
	case nil:
		// A NULL scanned into a non-nullable column means the schema and the model disagree; a zero
		// value would hide that. Nullable columns use NullTime instead.
		return fmt.Errorf("utctime: cannot scan NULL into a non-nullable Time")
	default:
		return fmt.Errorf("utctime: cannot scan %T into Time", src)
	}
}

// Value implements driver.Valuer. Normalizing on the way out is not needed for correctness — a
// TIMESTAMPTZ stores an instant, so writing a -03:00 value stores the same point in time — but it
// keeps what is sent on the wire consistent with what comes back.
func (t Time) Value() (driver.Value, error) {
	return t.Time.UTC(), nil
}

// MarshalJSON emits RFC 3339 in UTC (…Z). time.Time's own marshaller would use the value's location,
// which is exactly the behaviour this type exists to remove.
func (t Time) MarshalJSON() ([]byte, error) {
	return t.Time.UTC().MarshalJSON()
}

// UnmarshalJSON parses RFC 3339 and converts to UTC, so an offset supplied by a caller
// (`...-03:00`) is accepted and immediately normalized.
func (t *Time) UnmarshalJSON(data []byte) error {
	var parsed time.Time
	if err := parsed.UnmarshalJSON(data); err != nil {
		return err
	}
	t.Time = parsed.UTC()
	return nil
}

// NullTime is a nullable TIMESTAMPTZ that is always in UTC when valid. It replaces sql.NullTime on the
// nullable columns (modified_at, removed_at, last_login_at, last_article_discovery_at).
type NullTime struct {
	Time  Time
	Valid bool
}

// NewNull builds a valid NullTime from a time.Time, normalizing to UTC.
func NewNull(t time.Time) NullTime {
	return NullTime{Time: New(t), Valid: true}
}

// Scan implements sql.Scanner.
func (n *NullTime) Scan(src any) error {
	if src == nil {
		n.Time, n.Valid = Time{}, false
		return nil
	}
	if err := n.Time.Scan(src); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// Value implements driver.Valuer.
func (n NullTime) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time.Value()
}

// MarshalJSON emits null when invalid, otherwise RFC 3339 in UTC.
func (n NullTime) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return n.Time.MarshalJSON()
}

// UnmarshalJSON accepts null (invalid) or an RFC 3339 timestamp (valid, normalized to UTC).
func (n *NullTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Time, n.Valid = Time{}, false
		return nil
	}
	if err := n.Time.UnmarshalJSON(data); err != nil {
		return err
	}
	n.Valid = true
	return nil
}

// Ptr returns the instant as a *time.Time in UTC, or nil when invalid. Response schemas carry
// nullable dates as *time.Time, and this is the one-liner that maps a column onto them.
func (n NullTime) Ptr() *time.Time {
	if !n.Valid {
		return nil
	}
	t := n.Time.UTC()
	return &t
}

package utctime_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/utctime"
)

// saoPaulo is a fixed non-UTC zone standing in for "the host is not in UTC" — the condition that
// produced the bug this package exists to prevent. Built by hand rather than via LoadLocation so the
// test does not depend on a tzdata database being present.
var saoPaulo = time.FixedZone("-03", -3*60*60)

// instant is one moment expressed in that zone. In UTC it is 2026-07-16T18:01:13Z.
var instant = time.Date(2026, 7, 16, 15, 1, 13, 0, saoPaulo)

func TestTime_Scan_ConvertsNonUTCToUTC(t *testing.T) {
	var got utctime.Time
	require.NoError(t, got.Scan(instant))

	assert.Equal(t, time.UTC, got.Location(), "a scanned value must land in UTC")
	assert.True(t, got.Equal(instant), "the instant itself must not move")
	assert.Equal(t, "2026-07-16T18:01:13Z", got.Format(time.RFC3339))
}

func TestTime_Scan_RejectsNullAndWrongType(t *testing.T) {
	var got utctime.Time
	assert.Error(t, got.Scan(nil), "NULL into a non-nullable column must fail loudly, not zero out")
	assert.Error(t, got.Scan("2026-07-16"), "a non-time value must fail")
}

func TestTime_Value_NormalizesToUTC(t *testing.T) {
	v, err := utctime.Time{Time: instant}.Value()
	require.NoError(t, err)

	written, ok := v.(time.Time)
	require.True(t, ok)
	assert.Equal(t, time.UTC, written.Location())
	assert.True(t, written.Equal(instant))
}

func TestTime_MarshalJSON_AlwaysZ(t *testing.T) {
	// Built with a non-UTC location on purpose: time.Time's own marshaller would emit -03:00 here.
	out, err := json.Marshal(utctime.Time{Time: instant})
	require.NoError(t, err)
	assert.JSONEq(t, `"2026-07-16T18:01:13Z"`, string(out))
}

func TestTime_UnmarshalJSON_NormalizesOffset(t *testing.T) {
	var got utctime.Time
	require.NoError(t, json.Unmarshal([]byte(`"2026-07-16T15:01:13-03:00"`), &got))

	assert.Equal(t, time.UTC, got.Location())
	assert.Equal(t, "2026-07-16T18:01:13Z", got.Format(time.RFC3339))
}

func TestNew_NormalizesToUTC(t *testing.T) {
	got := utctime.New(instant)
	assert.Equal(t, time.UTC, got.Location())
	assert.True(t, got.Equal(instant))
}

func TestNullTime_Scan_NullIsInvalid(t *testing.T) {
	var got utctime.NullTime
	require.NoError(t, got.Scan(nil))
	assert.False(t, got.Valid)
	assert.Nil(t, got.Ptr())
}

func TestNullTime_Scan_ValueIsUTC(t *testing.T) {
	var got utctime.NullTime
	require.NoError(t, got.Scan(instant))

	require.True(t, got.Valid)
	assert.Equal(t, time.UTC, got.Time.Location())
	assert.Equal(t, "2026-07-16T18:01:13Z", got.Time.Format(time.RFC3339))
}

func TestNullTime_Value(t *testing.T) {
	invalid, err := utctime.NullTime{}.Value()
	require.NoError(t, err)
	assert.Nil(t, invalid, "an invalid NullTime must write NULL")

	valid, err := utctime.NewNull(instant).Value()
	require.NoError(t, err)
	written, ok := valid.(time.Time)
	require.True(t, ok)
	assert.Equal(t, time.UTC, written.Location())
}

func TestNullTime_MarshalJSON(t *testing.T) {
	out, err := json.Marshal(utctime.NullTime{})
	require.NoError(t, err)
	assert.Equal(t, "null", string(out))

	out, err = json.Marshal(utctime.NewNull(instant))
	require.NoError(t, err)
	assert.JSONEq(t, `"2026-07-16T18:01:13Z"`, string(out))
}

func TestNullTime_UnmarshalJSON(t *testing.T) {
	var fromNull utctime.NullTime
	require.NoError(t, json.Unmarshal([]byte("null"), &fromNull))
	assert.False(t, fromNull.Valid)

	var fromValue utctime.NullTime
	require.NoError(t, json.Unmarshal([]byte(`"2026-07-16T15:01:13-03:00"`), &fromValue))
	require.True(t, fromValue.Valid)
	assert.Equal(t, "2026-07-16T18:01:13Z", fromValue.Time.Format(time.RFC3339))
}

func TestNullTime_Ptr_ReturnsUTC(t *testing.T) {
	got := utctime.NullTime{Time: utctime.Time{Time: instant}, Valid: true}.Ptr()
	require.NotNil(t, got)
	assert.Equal(t, time.UTC, got.Location())
	assert.True(t, got.Equal(instant))
}

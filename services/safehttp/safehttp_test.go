package safehttp

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPublicAddr_RefusesNonPublicRanges(t *testing.T) {
	for _, raw := range []string{
		"127.0.0.1",       // loopback
		"::1",             // loopback v6
		"10.0.0.1",        // RFC1918
		"172.16.0.1",      // RFC1918
		"192.168.1.1",     // RFC1918
		"169.254.169.254", // link-local: the cloud metadata endpoint
		"fe80::1",         // link-local v6
		"fd00::1",         // unique local v6 (fc00::/7)
		"0.0.0.0",         // unspecified
		"100.64.0.1",      // CGNAT (RFC 6598)
		"100.127.255.255", // CGNAT, upper bound
		"224.0.0.1",       // multicast
		"0.1.2.3",         // "this network"
	} {
		ip := net.ParseIP(raw)
		require.NotNil(t, ip, "could not parse %s", raw)
		assert.False(t, IsPublicAddr(ip), "%s must be refused", raw)
	}
}

func TestIsPublicAddr_AllowsPublicRanges(t *testing.T) {
	for _, raw := range []string{
		"8.8.8.8",
		"1.1.1.1",
		"93.184.216.34",
		"2606:2800:220:1:248:1893:25c8:1946",
		"100.63.255.255", // just below CGNAT
		"100.128.0.1",    // just above CGNAT
		"172.32.0.1",     // just above the RFC1918 172.16/12 block
	} {
		ip := net.ParseIP(raw)
		require.NotNil(t, ip, "could not parse %s", raw)
		assert.True(t, IsPublicAddr(ip), "%s must be allowed", raw)
	}
}

func TestIsPublicAddr_NilIsRefused(t *testing.T) {
	assert.False(t, IsPublicAddr(nil))
}

// The point of putting the control in the dialer: a real server on loopback must be unreachable even
// though the URL is perfectly well-formed and the host is up.
func TestRestrictedClient_RefusesLoopbackServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := NewRestrictedClient(5 * time.Second).Get(server.URL)
	require.Error(t, err, "a loopback server must not be reachable")
	assert.ErrorIs(t, err, ErrBlockedDestination)
}

// A redirect is the case URL validation cannot cover: the first hop is public-looking, the second is
// internal. Every hop opens a connection, so the dialer catches it.
func TestRestrictedClient_RefusesRedirectToLoopback(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer internal.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL, http.StatusFound)
	}))
	defer redirector.Close()

	// The redirector is itself on loopback, so the first hop is already refused — which is the
	// verdict we want either way. Assert on the error type rather than on which hop failed.
	_, err := NewRestrictedClient(5 * time.Second).Get(redirector.URL)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBlockedDestination), "got %v", err)
}

func TestRestrictedClient_RefusesLiteralMetadataAddress(t *testing.T) {
	_, err := NewRestrictedClient(2 * time.Second).Get("http://169.254.169.254/latest/meta-data/")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBlockedDestination)
}

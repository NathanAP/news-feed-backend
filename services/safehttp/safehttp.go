// Package safehttp builds HTTP clients that refuse to reach private network destinations.
//
// It exists for the one shape of request the application cannot vet in advance: fetching a URL that
// came from outside (today the RSS discovery endpoint; tomorrow the source-suggestion route on the
// roadmap). A server that fetches a caller-chosen URL is a server-side request forgery primitive —
// it can be aimed at loopback, at the cloud metadata endpoint, or at private ranges the process can
// reach but the caller never should.
//
// The control lives in the DIALER, not in URL validation, and that placement is the whole point:
//
//   - A hostname is not an address. `evil.example.com` can resolve to 127.0.0.1, so inspecting the
//     URL string proves nothing about where the connection lands.
//   - Redirects escape URL checks entirely. A public URL answering 302 to
//     http://169.254.169.254/... would sail past any check done before the first request. Every hop
//     opens a new connection, so every hop passes through the dialer.
//
// After resolving, the vetted IP is dialed DIRECTLY rather than handing the hostname back to the
// dialer. Re-resolving would open a window where the second lookup returns a different address than
// the one just approved (DNS rebinding).
package safehttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ErrBlockedDestination is returned when a request resolves to a non-public address. It is typed so
// a caller can tell a refused destination apart from an ordinary network failure.
var ErrBlockedDestination = errors.New("destination is not a public address")

// NewRestrictedClient returns a client that behaves like a normal one except that it will not connect
// to a private, loopback, link-local, multicast or otherwise non-public address. timeout bounds the
// whole request, redirects included.
func NewRestrictedClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

	transport := &http.Transport{
		// Proxy is deliberately nil, NOT http.ProxyFromEnvironment (0.48.1).
		//
		// With a proxy configured, the transport dials the PROXY and the real target travels inside
		// the request line — so the dialer below would vet the proxy's address (public, allowed) and
		// never see the destination. Measured: with HTTP_PROXY set, a request to http://10.0.0.1/
		// stopped returning ErrBlockedDestination and went to the proxy instead. That is the whole
		// control silently disabled by an environment variable.
		//
		// This client only fetches public feed URLs, so losing proxy support costs nothing. Any client
		// that DOES need a proxy cannot use a dialer-based destination allow-list; it would have to
		// vet the target URL before the request and re-vet on every redirect.
		Proxy:                 nil,
		DialContext:           restrictedDialContext(dialer),
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{Timeout: timeout, Transport: transport}
}

func restrictedDialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		// A literal address needs no lookup — just the same verdict.
		if ip := net.ParseIP(host); ip != nil {
			if !IsPublicAddr(ip) {
				return nil, fmt.Errorf("%w: %s", ErrBlockedDestination, ip)
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}

		// Every answer must be public, not merely one of them: a name that resolves to both a public
		// and a private address is exactly what a rebinding attempt looks like, so refuse the lot.
		for _, candidate := range ips {
			if !IsPublicAddr(candidate.IP) {
				return nil, fmt.Errorf("%w: %s resolves to %s", ErrBlockedDestination, host, candidate.IP)
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("%w: %s resolves to nothing", ErrBlockedDestination, host)
		}

		// Dial the address that was just approved, never the hostname (see the package comment).
		var lastErr error
		for _, candidate := range ips {
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(candidate.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

// IsPublicAddr reports whether ip is a routable public address. Everything else — loopback, RFC1918
// and fc00::/7 (net.IP.IsPrivate), link-local (which includes the 169.254.169.254 cloud metadata
// endpoint), multicast, unspecified, and the CGNAT range — is refused.
//
// Exported so a test can assert the verdict per range without dialing anything.
func IsPublicAddr(ip net.IP) bool {
	// Shaped as an allow-list, not a block-list (0.48.1). The first version enumerated the bad ranges
	// and let everything else through, which meant anything it had not thought of was allowed —
	// 255.255.255.255 and 240.0.0.0/4 slipped past exactly that way. Requiring global unicast first
	// inverts the default: unknown is refused. IsGlobalUnicast already excludes loopback, link-local
	// (including the 169.254.169.254 metadata address), multicast, unspecified and broadcast.
	if ip == nil || !ip.IsGlobalUnicast() {
		return false
	}
	// Global unicast but still not public: RFC1918 and fc00::/7.
	if ip.IsPrivate() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 0: // 0.0.0.0/8, "this network"
			return false
		case v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127: // RFC 6598 CGNAT, not covered by IsPrivate
			return false
		case v4[0] >= 240: // 240.0.0.0/4 reserved (Class E)
			return false
		}
	}
	return true
}

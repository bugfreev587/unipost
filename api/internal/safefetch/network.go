package safefetch

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"
)

type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type dialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type pinnedTarget struct {
	Hostname string
	Port     string
	IP       netip.Addr
}

// Config owns time budgets and private temporary-file placement for a
// production Fetcher. Individual fetch policies own content limits.
type Config struct {
	ConnectTimeout        time.Duration
	ResponseHeaderTimeout time.Duration
	IdleReadTimeout       time.Duration
	TotalTimeout          time.Duration
	TempDir               string
}

func DefaultConfig() Config {
	return Config{
		ConnectTimeout:        5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleReadTimeout:       10 * time.Second,
		TotalTimeout:          30 * time.Second,
	}
}

func newPinnedTransport(
	ctx context.Context,
	targetURL *url.URL,
	resolver resolver,
	dialer dialer,
	config Config,
) (*http.Transport, error) {
	if targetURL == nil || resolver == nil || dialer == nil {
		return nil, newFetchError(ErrorInvalidURL)
	}
	hostname, err := normalizeHostname(targetURL.Hostname())
	if err != nil {
		return nil, newFetchError(ErrorInvalidURL)
	}
	port := targetURL.Port()
	if port == "" {
		switch strings.ToLower(targetURL.Scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return nil, newFetchError(ErrorInvalidURL)
		}
	}

	addresses, err := resolveAddresses(ctx, resolver, hostname)
	if err != nil {
		return nil, err
	}
	if err := validateResolvedAddresses(hostname, addresses); err != nil {
		return nil, err
	}
	sort.Slice(addresses, func(i, j int) bool {
		return addresses[i].Unmap().Compare(addresses[j].Unmap()) < 0
	})
	target := pinnedTarget{Hostname: hostname, Port: port, IP: addresses[0].Unmap()}
	pinnedAddresses := append([]netip.Addr(nil), addresses...)

	transport := &http.Transport{
		Proxy:                  nil,
		DisableKeepAlives:      true,
		ResponseHeaderTimeout:  config.ResponseHeaderTimeout,
		TLSHandshakeTimeout:    config.ConnectTimeout,
		MaxResponseHeaderBytes: 64 << 10,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: hostname,
		},
	}
	transport.DialContext = func(dialCtx context.Context, network, address string) (net.Conn, error) {
		if !isExpectedDial(network, address, target) {
			return nil, newFetchError(ErrorPeerMismatch)
		}
		return dialPinnedTarget(
			dialCtx,
			dialer,
			target,
			pinnedAddresses,
			config.ConnectTimeout,
			config.IdleReadTimeout,
		)
	}
	return transport, nil
}

func resolveAddresses(ctx context.Context, resolver resolver, hostname string) ([]netip.Addr, error) {
	if literal, err := netip.ParseAddr(hostname); err == nil {
		return []netip.Addr{literal.Unmap()}, nil
	}
	addresses, err := resolver.LookupNetIP(ctx, "ip", hostname)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			return nil, newFetchStatusError(ErrorTimeout, 0, true)
		}
		return nil, newFetchStatusError(ErrorSourceTemporary, 0, true)
	}
	return append([]netip.Addr(nil), addresses...), nil
}

func dialPinnedTarget(
	ctx context.Context,
	dialer dialer,
	target pinnedTarget,
	allowed []netip.Addr,
	connectTimeout time.Duration,
	idleReadTimeout time.Duration,
) (net.Conn, error) {
	dialCtx := ctx
	cancel := func() {}
	if connectTimeout > 0 {
		dialCtx, cancel = context.WithTimeout(ctx, connectTimeout)
	}
	defer cancel()

	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(target.IP.String(), target.Port))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
			return nil, newFetchStatusError(ErrorTimeout, 0, true)
		}
		return nil, newFetchStatusError(ErrorSourceTemporary, 0, true)
	}
	if conn == nil || !peerAllowed(conn.RemoteAddr(), allowed) {
		if conn != nil {
			_ = conn.Close()
		}
		return nil, newFetchError(ErrorPeerMismatch)
	}
	return &idleDeadlineConn{Conn: conn, idleReadTimeout: idleReadTimeout}, nil
}

func peerAllowed(remote net.Addr, allowed []netip.Addr) bool {
	if remote == nil {
		return false
	}
	host, _, err := net.SplitHostPort(remote.String())
	if err != nil {
		return false
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	peer = peer.Unmap()
	if isProhibitedAddress(peer) {
		return false
	}
	for _, address := range allowed {
		if peer == address.Unmap() {
			return true
		}
	}
	return false
}

func isExpectedDial(network, address string, target pinnedTarget) bool {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return false
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != target.Port {
		return false
	}
	normalized, err := normalizeHostname(host)
	if err != nil {
		return false
	}
	return normalized == target.Hostname || normalized == target.IP.String()
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

type idleDeadlineConn struct {
	net.Conn
	idleReadTimeout time.Duration
}

func (c *idleDeadlineConn) Read(buffer []byte) (int, error) {
	if c.idleReadTimeout > 0 {
		if err := c.Conn.SetReadDeadline(time.Now().Add(c.idleReadTimeout)); err != nil {
			return 0, newFetchStatusError(ErrorSourceTemporary, 0, true)
		}
	}
	read, err := c.Conn.Read(buffer)
	if err != nil && isTimeoutError(err) {
		return read, newFetchStatusError(ErrorTimeout, 0, true)
	}
	return read, err
}

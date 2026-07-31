package safefetch

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestConnectTimeoutIsSanitized(t *testing.T) {
	t.Parallel()

	target := pinnedTarget{Hostname: "cdn.example.com", Port: "443", IP: mustAddr("8.8.8.8")}
	_, err := dialPinnedTarget(
		context.Background(),
		blockingDialer{},
		target,
		[]netipAddr{target.IP},
		20*time.Millisecond,
		time.Second,
	)
	assertTimeoutError(t, err, "cdn.example.com")
}

func TestResponseHeaderTimeoutIsSanitized(t *testing.T) {
	t.Parallel()

	fetcher := &fetcher{
		config: DefaultConfig(),
		transportFactory: func(context.Context, *url.URL) (http.RoundTripper, error) {
			return roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, timeoutNetworkError{}
			}), nil
		},
	}
	_, err := fetcher.fetchResponse(context.Background(), "https://secret.example/media?token=hidden", Policy{MaxRedirects: 5})
	assertTimeoutError(t, err, "secret.example")
}

func TestTotalTimeoutIsSanitized(t *testing.T) {
	t.Parallel()

	fetcher := &fetcher{
		config: DefaultConfig(),
		transportFactory: func(context.Context, *url.URL) (http.RoundTripper, error) {
			return roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				<-request.Context().Done()
				return nil, request.Context().Err()
			}), nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := fetcher.fetchResponse(ctx, "https://secret.example/media?token=hidden", Policy{MaxRedirects: 5})
	assertTimeoutError(t, err, "token=hidden")
}

func TestIdleReadTimeoutIsSanitized(t *testing.T) {
	t.Parallel()

	conn := &idleDeadlineConn{Conn: timeoutReadConn{}, idleReadTimeout: time.Second}
	_, err := conn.Read(make([]byte, 1))
	assertTimeoutError(t, err, "timeoutReadConn")
}

func assertTimeoutError(t *testing.T, err error, secret string) {
	t.Helper()
	assertFetchErrorKind(t, err, ErrorTimeout)
	var fetchErr *FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("error %T is not *FetchError", err)
	}
	if !fetchErr.Temporary {
		t.Fatal("timeout error is not temporary")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("sanitized timeout error contains %q: %q", secret, err.Error())
	}
}

type blockingDialer struct{}

func (blockingDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type timeoutNetworkError struct{}

func (timeoutNetworkError) Error() string   { return "simulated network timeout with secret" }
func (timeoutNetworkError) Timeout() bool   { return true }
func (timeoutNetworkError) Temporary() bool { return true }

type timeoutReadConn struct{}

func (timeoutReadConn) Read([]byte) (int, error)         { return 0, timeoutNetworkError{} }
func (timeoutReadConn) Write(p []byte) (int, error)      { return len(p), nil }
func (timeoutReadConn) Close() error                     { return nil }
func (timeoutReadConn) LocalAddr() net.Addr              { return stringAddr("192.0.2.1:50000") }
func (timeoutReadConn) RemoteAddr() net.Addr             { return stringAddr("8.8.8.8:443") }
func (timeoutReadConn) SetDeadline(time.Time) error      { return nil }
func (timeoutReadConn) SetReadDeadline(time.Time) error  { return nil }
func (timeoutReadConn) SetWriteDeadline(time.Time) error { return nil }

type netipAddr = netip.Addr

func mustAddr(raw string) netipAddr { return netip.MustParseAddr(raw) }

package safefetch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestPinnedTargetValidatesEveryDNSAnswerBeforeDial(t *testing.T) {
	t.Parallel()

	resolved := &sequenceResolver{answers: [][]netip.Addr{{
		netip.MustParseAddr("8.8.8.8"),
		netip.MustParseAddr("10.0.0.8"),
	}}}
	dialed := &recordingDialer{conn: newFakeConn("8.8.8.8:443", nil)}
	targetURL := mustURL(t, "https://cdn.example.com/image.jpg")

	_, err := newPinnedTransport(context.Background(), targetURL, resolved, dialed, DefaultConfig())
	assertFetchErrorKind(t, err, ErrorForbiddenDestination)
	if dialed.callCount() != 0 {
		t.Fatalf("dialer called %d times before all DNS answers passed policy", dialed.callCount())
	}
}

func TestPinnedDialUsesLiteralIPAndOriginalPort(t *testing.T) {
	t.Parallel()

	dialed := &recordingDialer{conn: newFakeConn("8.8.8.8:8443", nil)}
	target := pinnedTarget{
		Hostname: "cdn.example.com",
		Port:     "8443",
		IP:       netip.MustParseAddr("8.8.8.8"),
	}
	allowed := []netip.Addr{target.IP, netip.MustParseAddr("1.1.1.1")}

	conn, err := dialPinnedTarget(context.Background(), dialed, target, allowed, time.Second, time.Second)
	if err != nil {
		t.Fatalf("dialPinnedTarget returned error: %v", err)
	}
	_ = conn.Close()

	if got := dialed.lastAddress(); got != "8.8.8.8:8443" {
		t.Fatalf("dial address = %q, want literal pinned IP", got)
	}
}

func TestPinnedTransportPreservesTLSServerName(t *testing.T) {
	t.Parallel()

	resolved := &sequenceResolver{answers: [][]netip.Addr{{netip.MustParseAddr("8.8.8.8")}}}
	dialed := &recordingDialer{conn: newFakeConn("8.8.8.8:443", nil)}
	targetURL := mustURL(t, "https://CDN.Example.COM/image.jpg")

	transport, err := newPinnedTransport(context.Background(), targetURL, resolved, dialed, DefaultConfig())
	if err != nil {
		t.Fatalf("newPinnedTransport returned error: %v", err)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLSClientConfig is nil")
	}
	if got := transport.TLSClientConfig.ServerName; got != "cdn.example.com" {
		t.Fatalf("TLS ServerName = %q, want cdn.example.com", got)
	}
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS certificate verification is disabled")
	}
}

func TestRebindingDoesNotTriggerSecondHostnameLookup(t *testing.T) {
	t.Parallel()

	resolved := &sequenceResolver{answers: [][]netip.Addr{
		{netip.MustParseAddr("8.8.8.8")},
		{netip.MustParseAddr("127.0.0.1")},
	}}
	dialed := &recordingDialer{connFactory: func() net.Conn {
		return newFakeConn("8.8.8.8:443", nil)
	}}
	targetURL := mustURL(t, "https://cdn.example.com/image.jpg")

	transport, err := newPinnedTransport(context.Background(), targetURL, resolved, dialed, DefaultConfig())
	if err != nil {
		t.Fatalf("newPinnedTransport returned error: %v", err)
	}
	for range 2 {
		conn, dialErr := transport.DialContext(context.Background(), "tcp", "cdn.example.com:443")
		if dialErr != nil {
			t.Fatalf("DialContext returned error: %v", dialErr)
		}
		_ = conn.Close()
	}

	if got := resolved.callCount(); got != 1 {
		t.Fatalf("resolver called %d times, want exactly once", got)
	}
	for _, address := range dialed.addressesSnapshot() {
		if address != "8.8.8.8:443" {
			t.Fatalf("dialed %q after validation, want pinned public address", address)
		}
	}
}

func TestPeerMismatchRejectsLoopbackAndUnvalidatedPublicIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		remote string
	}{
		{name: "loopback", remote: "127.0.0.1:443"},
		{name: "unvalidated public", remote: "1.1.1.1:443"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := pinnedTarget{
				Hostname: "cdn.example.com",
				Port:     "443",
				IP:       netip.MustParseAddr("8.8.8.8"),
			}
			dialed := &recordingDialer{conn: newFakeConn(tt.remote, nil)}
			_, err := dialPinnedTarget(
				context.Background(),
				dialed,
				target,
				[]netip.Addr{target.IP},
				time.Second,
				time.Second,
			)
			assertFetchErrorKind(t, err, ErrorPeerMismatch)
			if !dialed.wasClosed() {
				t.Fatal("mismatched peer connection was not closed")
			}
		})
	}
}

func TestPinnedConnectionRefreshesIdleReadDeadline(t *testing.T) {
	t.Parallel()

	underlying := newFakeConn("8.8.8.8:443", []byte("ok"))
	dialed := &recordingDialer{conn: underlying}
	target := pinnedTarget{
		Hostname: "cdn.example.com",
		Port:     "443",
		IP:       netip.MustParseAddr("8.8.8.8"),
	}
	conn, err := dialPinnedTarget(
		context.Background(),
		dialed,
		target,
		[]netip.Addr{target.IP},
		time.Second,
		250*time.Millisecond,
	)
	if err != nil {
		t.Fatalf("dialPinnedTarget returned error: %v", err)
	}
	defer conn.Close()

	before := time.Now()
	buffer := make([]byte, 2)
	if _, err := io.ReadFull(conn, buffer); err != nil {
		t.Fatalf("read failed: %v", err)
	}
	deadline := underlying.lastReadDeadline()
	if deadline.Before(before.Add(200*time.Millisecond)) || deadline.After(time.Now().Add(350*time.Millisecond)) {
		t.Fatalf("read deadline %s was not refreshed from the read time", deadline)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := parseWebURL(raw)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	return parsed
}

type sequenceResolver struct {
	mu      sync.Mutex
	answers [][]netip.Addr
	calls   int
}

func (r *sequenceResolver) LookupNetIP(_ context.Context, network, _ string) ([]netip.Addr, error) {
	if network != "ip" {
		return nil, errors.New("unexpected network")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	index := r.calls
	r.calls++
	if index >= len(r.answers) {
		index = len(r.answers) - 1
	}
	return append([]netip.Addr(nil), r.answers[index]...), nil
}

func (r *sequenceResolver) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type recordingDialer struct {
	mu          sync.Mutex
	conn        *fakeConn
	connFactory func() net.Conn
	addresses   []string
	lastConn    net.Conn
}

func (d *recordingDialer) DialContext(_ context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" {
		return nil, errors.New("unexpected network")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.addresses = append(d.addresses, address)
	if d.connFactory != nil {
		d.lastConn = d.connFactory()
		return d.lastConn, nil
	}
	d.lastConn = d.conn
	return d.conn, nil
}

func (d *recordingDialer) callCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.addresses)
}

func (d *recordingDialer) lastAddress() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.addresses) == 0 {
		return ""
	}
	return d.addresses[len(d.addresses)-1]
}

func (d *recordingDialer) addressesSnapshot() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.addresses...)
}

func (d *recordingDialer) wasClosed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if conn, ok := d.lastConn.(*fakeConn); ok {
		return conn.isClosed()
	}
	return false
}

type fakeConn struct {
	mu           sync.Mutex
	reader       *bytes.Reader
	remote       net.Addr
	closed       bool
	readDeadline time.Time
}

func newFakeConn(remote string, body []byte) *fakeConn {
	return &fakeConn{reader: bytes.NewReader(body), remote: stringAddr(remote)}
}

func (c *fakeConn) Read(buffer []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reader.Read(buffer)
}

func (c *fakeConn) Write(buffer []byte) (int, error) { return len(buffer), nil }

func (c *fakeConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *fakeConn) LocalAddr() net.Addr              { return stringAddr("192.0.2.10:50000") }
func (c *fakeConn) RemoteAddr() net.Addr             { return c.remote }
func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

func (c *fakeConn) SetReadDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadline = deadline
	return nil
}

func (c *fakeConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *fakeConn) lastReadDeadline() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readDeadline
}

type stringAddr string

func (a stringAddr) Network() string { return "tcp" }
func (a stringAddr) String() string  { return string(a) }

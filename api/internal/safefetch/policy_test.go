package safefetch

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func TestURLPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "https", raw: "https://cdn.example.com/image.jpg"},
		{name: "http", raw: "http://cdn.example.com/image.jpg"},
		{name: "malformed", raw: "://bad", wantErr: true},
		{name: "missing host", raw: "https:///image.jpg", wantErr: true},
		{name: "relative", raw: "/image.jpg", wantErr: true},
		{name: "file", raw: "file:///etc/passwd", wantErr: true},
		{name: "ftp", raw: "ftp://cdn.example.com/image.jpg", wantErr: true},
		{name: "data", raw: "data:image/png;base64,AAAA", wantErr: true},
		{name: "userinfo", raw: "https://user:secret@cdn.example.com/image.jpg", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseWebURL(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseWebURL(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if tt.wantErr {
				assertFetchErrorKind(t, err, ErrorInvalidURL)
				if got := err.Error(); got == "" || containsSensitiveInput(got, tt.raw) {
					t.Fatalf("error %q is empty or contains the raw URL", got)
				}
			}
		})
	}
}

func TestNormalizeHostname(t *testing.T) {
	t.Parallel()

	got, err := normalizeHostname("CDN.Example.COM.")
	if err != nil {
		t.Fatalf("normalizeHostname returned error: %v", err)
	}
	if got != "cdn.example.com" {
		t.Fatalf("normalizeHostname = %q, want cdn.example.com", got)
	}

	for _, host := range []string{"", ".", "bad..example.com", "bad host.example"} {
		if _, err := normalizeHostname(host); err == nil {
			t.Fatalf("normalizeHostname(%q) unexpectedly succeeded", host)
		}
	}
}

func TestAddressPolicyRejectsProhibitedRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
	}{
		{name: "ipv4 unspecified", addr: "0.0.0.0"},
		{name: "ipv4 loopback", addr: "127.0.0.1"},
		{name: "rfc1918 ten", addr: "10.1.2.3"},
		{name: "rfc1918 one seventy two", addr: "172.16.0.1"},
		{name: "rfc1918 one ninety two", addr: "192.168.1.1"},
		{name: "ipv4 link local", addr: "169.254.1.1"},
		{name: "ipv4 multicast", addr: "224.0.0.1"},
		{name: "ipv4 limited broadcast", addr: "255.255.255.255"},
		{name: "ipv4 documentation one", addr: "192.0.2.1"},
		{name: "ipv4 documentation two", addr: "198.51.100.1"},
		{name: "ipv4 documentation three", addr: "203.0.113.1"},
		{name: "ipv4 benchmark", addr: "198.18.0.1"},
		{name: "carrier grade nat", addr: "100.64.0.1"},
		{name: "ipv4 reserved", addr: "240.0.0.1"},
		{name: "aws metadata", addr: "169.254.169.254"},
		{name: "ecs metadata", addr: "169.254.170.2"},
		{name: "ipv6 unspecified", addr: "::"},
		{name: "ipv6 loopback", addr: "::1"},
		{name: "ipv6 unique local", addr: "fc00::1"},
		{name: "ipv6 link local", addr: "fe80::1"},
		{name: "ipv6 multicast", addr: "ff00::1"},
		{name: "ipv6 documentation", addr: "2001:db8::1"},
		{name: "ipv6 orchid", addr: "2001:10::1"},
		{name: "ipv4 mapped loopback", addr: "::ffff:127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			addr := netip.MustParseAddr(tt.addr)
			if !isProhibitedAddress(addr) {
				t.Fatalf("isProhibitedAddress(%s) = false, want true", addr)
			}
			err := validateResolvedAddresses("cdn.example.com", []netip.Addr{addr})
			assertFetchErrorKind(t, err, ErrorForbiddenDestination)
		})
	}
}

func TestAddressPolicyAllowsPublicAnswers(t *testing.T) {
	t.Parallel()

	addresses := []netip.Addr{
		netip.MustParseAddr("8.8.8.8"),
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2606:4700:4700::1111"),
	}
	if err := validateResolvedAddresses("cdn.example.com", addresses); err != nil {
		t.Fatalf("public answers rejected: %v", err)
	}
}

func TestAddressPolicyRejectsMixedDNSAnswers(t *testing.T) {
	t.Parallel()

	err := validateResolvedAddresses("cdn.example.com", []netip.Addr{
		netip.MustParseAddr("8.8.8.8"),
		netip.MustParseAddr("10.0.0.8"),
	})
	assertFetchErrorKind(t, err, ErrorForbiddenDestination)
}

func TestAddressPolicyRejectsMissingDNSAnswers(t *testing.T) {
	t.Parallel()

	err := validateResolvedAddresses("cdn.example.com", nil)
	assertFetchErrorKind(t, err, ErrorForbiddenDestination)
}

func TestMetadataHostnames(t *testing.T) {
	t.Parallel()

	for _, host := range []string{
		"metadata.google.internal",
		"METADATA.GOOGLE.INTERNAL.",
		"metadata.azure.internal",
		"metadata.aws.internal",
		"instance-data",
	} {
		if !isMetadataHostname(host) {
			t.Fatalf("isMetadataHostname(%q) = false, want true", host)
		}
		err := validateResolvedAddresses(host, []netip.Addr{netip.MustParseAddr("8.8.8.8")})
		assertFetchErrorKind(t, err, ErrorForbiddenDestination)
	}

	if isMetadataHostname("metadata.example.com") {
		t.Fatal("ordinary public hostname classified as metadata")
	}
}

func assertFetchErrorKind(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error, got nil", want)
	}
	var fetchErr *FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("error %T is not *FetchError: %v", err, err)
	}
	if fetchErr.Kind != want {
		t.Fatalf("FetchError.Kind = %q, want %q", fetchErr.Kind, want)
	}
}

func containsSensitiveInput(message, raw string) bool {
	return raw != "" && strings.Contains(message, raw)
}

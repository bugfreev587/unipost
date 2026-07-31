package safefetch

import (
	"net/netip"
	"net/url"
	"strings"
)

var prohibitedPrefixes = mustPrefixes(
	// IPv4 special-use, private, local, documentation, benchmark,
	// multicast, and reserved ranges.
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.88.99.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",

	// IPv6 translation, discard-only, IETF special-use, documentation,
	// deprecated site-local, unique-local, link-local, and multicast ranges.
	"::/128",
	"::1/128",
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"100::/64",
	"2001::/23",
	"2001:db8::/32",
	"2002::/16",
	"fc00::/7",
	"fe80::/10",
	"fec0::/10",
	"ff00::/8",
)

var metadataHostnames = map[string]struct{}{
	"instance-data":                {},
	"metadata":                     {},
	"metadata.aws.internal":        {},
	"metadata.azure.internal":      {},
	"metadata.google.internal":     {},
	"metadata.internal":            {},
	"metadata.oraclecloud.com":     {},
	"metadata.packet.net":          {},
	"metadata.platformequinix.com": {},
}

func parseWebURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return nil, newFetchError(ErrorInvalidURL)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, newFetchError(ErrorInvalidURL)
	}
	if parsed.Host == "" || parsed.User != nil {
		return nil, newFetchError(ErrorInvalidURL)
	}
	host, err := normalizeHostname(parsed.Hostname())
	if err != nil {
		return nil, newFetchError(ErrorInvalidURL)
	}
	if isMetadataHostname(host) {
		return nil, newFetchError(ErrorForbiddenDestination)
	}
	if parsed.Port() != "" {
		// url.Parse validates the port syntax. Rebuild Host only to preserve
		// the canonical hostname used by later DNS and TLS policy.
		parsed.Host = netipHostPort(host, parsed.Port())
	} else if strings.Contains(host, ":") {
		parsed.Host = "[" + host + "]"
	} else {
		parsed.Host = host
	}
	return parsed, nil
}

func normalizeHostname(host string) (string, error) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" || len(host) > 253 {
		return "", newFetchError(ErrorInvalidURL)
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String(), nil
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", newFetchError(ErrorInvalidURL)
		}
		for _, char := range label {
			if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
				continue
			}
			return "", newFetchError(ErrorInvalidURL)
		}
	}
	return host, nil
}

func validateResolvedAddresses(host string, addresses []netip.Addr) error {
	normalized, err := normalizeHostname(host)
	if err != nil || isMetadataHostname(normalized) || len(addresses) == 0 {
		return newFetchError(ErrorForbiddenDestination)
	}
	for _, address := range addresses {
		if isProhibitedAddress(address) {
			return newFetchError(ErrorForbiddenDestination)
		}
	}
	return nil
}

func isProhibitedAddress(address netip.Addr) bool {
	if !address.IsValid() {
		return true
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsMulticast() || address.IsUnspecified() {
		return true
	}
	for _, prefix := range prohibitedPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func isMetadataHostname(host string) bool {
	normalized := strings.ToLower(strings.TrimSuffix(host, "."))
	_, denied := metadataHostnames[normalized]
	return denied
}

func mustPrefixes(raw ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(raw))
	for _, value := range raw {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}

func netipHostPort(host, port string) string {
	if strings.Contains(host, ":") {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}

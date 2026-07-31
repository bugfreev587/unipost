package safefetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestRedirectSameHostRevalidatesAndSucceeds(t *testing.T) {
	t.Parallel()

	script := newRedirectScript(map[string]scriptedHTTPResponse{
		"https://origin.example/start": {status: http.StatusFound, location: "/final"},
		"https://origin.example/final": {status: http.StatusOK, body: "image"},
	})
	script.answers["origin.example"] = [][]netip.Addr{
		{netip.MustParseAddr("8.8.8.8")},
		{netip.MustParseAddr("8.8.4.4")},
	}

	response, err := script.fetcher().fetchResponse(context.Background(), "https://origin.example/start", Policy{MaxRedirects: 5})
	if err != nil {
		t.Fatalf("fetchResponse returned error: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if got := script.resolutionCount("origin.example"); got != 2 {
		t.Fatalf("same-host resolver calls = %d, want 2", got)
	}
}

func TestRedirectCrossHostRevalidatesAndSucceeds(t *testing.T) {
	t.Parallel()

	script := newRedirectScript(map[string]scriptedHTTPResponse{
		"https://origin.example/start": {status: http.StatusFound, location: "https://cdn.example/final"},
		"https://cdn.example/final":    {status: http.StatusOK, body: "image"},
	})
	script.answers["origin.example"] = [][]netip.Addr{{netip.MustParseAddr("8.8.8.8")}}
	script.answers["cdn.example"] = [][]netip.Addr{{netip.MustParseAddr("1.1.1.1")}}

	response, err := script.fetcher().fetchResponse(context.Background(), "https://origin.example/start", Policy{MaxRedirects: 5})
	if err != nil {
		t.Fatalf("fetchResponse returned error: %v", err)
	}
	defer response.Body.Close()
	if script.resolutionCount("origin.example") != 1 || script.resolutionCount("cdn.example") != 1 {
		t.Fatalf("resolution counts = origin:%d cdn:%d, want 1 each", script.resolutionCount("origin.example"), script.resolutionCount("cdn.example"))
	}
}

func TestRedirectRejectsPrivateAndMetadataDestinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		location string
		answers  map[string][][]netip.Addr
	}{
		{name: "private ipv4", location: "http://10.0.0.8/media", answers: map[string][][]netip.Addr{}},
		{name: "private ipv6", location: "https://[fc00::8]/media", answers: map[string][][]netip.Addr{}},
		{name: "metadata ip", location: "http://169.254.169.254/latest/meta-data", answers: map[string][][]netip.Addr{}},
		{name: "metadata hostname", location: "https://metadata.google.internal/computeMetadata/v1", answers: map[string][][]netip.Addr{
			"metadata.google.internal": {{netip.MustParseAddr("8.8.8.8")}},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			script := newRedirectScript(map[string]scriptedHTTPResponse{
				"https://origin.example/start": {status: http.StatusFound, location: tt.location},
			})
			script.answers["origin.example"] = [][]netip.Addr{{netip.MustParseAddr("8.8.8.8")}}
			for host, answers := range tt.answers {
				script.answers[host] = answers
			}

			_, err := script.fetcher().fetchResponse(context.Background(), "https://origin.example/start", Policy{MaxRedirects: 5})
			var fetchErr *FetchError
			if !errors.As(err, &fetchErr) {
				t.Fatalf("error %T is not *FetchError: %v", err, err)
			}
			if fetchErr.Kind != ErrorForbiddenDestination && fetchErr.Kind != ErrorRedirectRejected {
				t.Fatalf("error kind = %q, want forbidden destination or redirect rejected", fetchErr.Kind)
			}
			if script.requestCount() != 1 {
				t.Fatalf("request count = %d, private redirect should fail before a second request", script.requestCount())
			}
		})
	}
}

func TestRedirectRejectsDowngradeUnsupportedSchemeAndUserinfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		location string
	}{
		{name: "https downgrade", location: "http://cdn.example/media"},
		{name: "file", location: "file:///etc/passwd"},
		{name: "ftp", location: "ftp://cdn.example/media"},
		{name: "data", location: "data:image/png;base64,AAAA"},
		{name: "relative userinfo", location: "//user:secret@cdn.example/media"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			script := newRedirectScript(map[string]scriptedHTTPResponse{
				"https://origin.example/start": {status: http.StatusFound, location: tt.location},
			})
			script.answers["origin.example"] = [][]netip.Addr{{netip.MustParseAddr("8.8.8.8")}}
			_, err := script.fetcher().fetchResponse(context.Background(), "https://origin.example/start", Policy{MaxRedirects: 5})
			assertFetchErrorKind(t, err, ErrorRedirectRejected)
			if script.requestCount() != 1 {
				t.Fatalf("request count = %d, invalid redirect should fail before a second request", script.requestCount())
			}
		})
	}
}

func TestRedirectLoopFailsBeforeHopLimit(t *testing.T) {
	t.Parallel()

	script := newRedirectScript(map[string]scriptedHTTPResponse{
		"https://origin.example/a": {status: http.StatusFound, location: "/b"},
		"https://origin.example/b": {status: http.StatusFound, location: "/a"},
	})
	script.answers["origin.example"] = repeatAnswers(8, "8.8.8.8")

	_, err := script.fetcher().fetchResponse(context.Background(), "https://origin.example/a", Policy{MaxRedirects: 5})
	assertFetchErrorKind(t, err, ErrorRedirectRejected)
	if got := script.requestCount(); got != 2 {
		t.Fatalf("loop request count = %d, want 2 before repeated URL", got)
	}
}

func TestRedirectAllowsFiveAndRejectsSixth(t *testing.T) {
	t.Parallel()

	responses := make(map[string]scriptedHTTPResponse)
	for index := 0; index < 6; index++ {
		responses[fmt.Sprintf("https://origin.example/%d", index)] = scriptedHTTPResponse{
			status:   http.StatusFound,
			location: fmt.Sprintf("/%d", index+1),
		}
	}
	responses["https://origin.example/6"] = scriptedHTTPResponse{status: http.StatusOK, body: "image"}
	script := newRedirectScript(responses)
	script.answers["origin.example"] = repeatAnswers(8, "8.8.8.8")

	_, err := script.fetcher().fetchResponse(context.Background(), "https://origin.example/0", Policy{MaxRedirects: 5})
	assertFetchErrorKind(t, err, ErrorRedirectRejected)
	if got := script.requestCount(); got != 6 {
		t.Fatalf("request count = %d, want initial plus five permitted redirects", got)
	}
}

func TestRedirectMissingLocationIsRejected(t *testing.T) {
	t.Parallel()

	script := newRedirectScript(map[string]scriptedHTTPResponse{
		"https://origin.example/start": {status: http.StatusFound},
	})
	script.answers["origin.example"] = [][]netip.Addr{{netip.MustParseAddr("8.8.8.8")}}

	_, err := script.fetcher().fetchResponse(context.Background(), "https://origin.example/start", Policy{MaxRedirects: 5})
	assertFetchErrorKind(t, err, ErrorRedirectRejected)
}

type scriptedHTTPResponse struct {
	status   int
	location string
	body     string
}

type redirectScript struct {
	mu          sync.Mutex
	responses   map[string]scriptedHTTPResponse
	answers     map[string][][]netip.Addr
	resolutions map[string]int
	requests    []string
}

func newRedirectScript(responses map[string]scriptedHTTPResponse) *redirectScript {
	return &redirectScript{
		responses:   responses,
		answers:     make(map[string][][]netip.Addr),
		resolutions: make(map[string]int),
	}
}

func (s *redirectScript) fetcher() *fetcher {
	return &fetcher{config: DefaultConfig(), transportFactory: s.transport}
}

func (s *redirectScript) transport(_ context.Context, target *url.URL) (http.RoundTripper, error) {
	s.mu.Lock()
	host := target.Hostname()
	index := s.resolutions[host]
	s.resolutions[host] = index + 1
	answers := s.answers[host]
	if len(answers) == 0 {
		if literal, err := netip.ParseAddr(host); err == nil {
			answers = [][]netip.Addr{{literal}}
		}
	}
	if len(answers) == 0 {
		s.mu.Unlock()
		return nil, newFetchStatusError(ErrorSourceTemporary, 0, true)
	}
	if index >= len(answers) {
		index = len(answers) - 1
	}
	resolved := append([]netip.Addr(nil), answers[index]...)
	s.mu.Unlock()

	if err := validateResolvedAddresses(host, resolved); err != nil {
		return nil, err
	}
	return roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		raw := request.URL.String()
		s.requests = append(s.requests, raw)
		response, ok := s.responses[raw]
		if !ok {
			return nil, fmt.Errorf("missing scripted response")
		}
		header := make(http.Header)
		if response.location != "" {
			header.Set("Location", response.location)
		}
		return &http.Response{
			StatusCode: response.status,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(response.body)),
			Request:    request,
		}, nil
	}), nil
}

func (s *redirectScript) resolutionCount(host string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resolutions[host]
}

func (s *redirectScript) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func repeatAnswers(count int, address string) [][]netip.Addr {
	answers := make([][]netip.Addr, count)
	for index := range answers {
		answers[index] = []netip.Addr{netip.MustParseAddr(address)}
	}
	return answers
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

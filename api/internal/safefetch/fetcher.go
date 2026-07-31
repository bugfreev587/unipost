package safefetch

import (
	"context"
	"crypto/sha256"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

type Policy struct {
	MaxBytes          int64
	AllowedMediaTypes []string
	MaxRedirects      int
}

type Result struct {
	Path      string
	MediaType string
	SizeBytes int64
	SHA256Hex string

	closeOnce sync.Once
	closeErr  error
}

func (r *Result) Open() (*os.File, error) {
	if r == nil || r.Path == "" {
		return nil, os.ErrNotExist
	}
	return os.Open(r.Path)
}

func (r *Result) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.Path != "" {
			r.closeErr = os.Remove(r.Path)
			if errors.Is(r.closeErr, os.ErrNotExist) {
				r.closeErr = nil
			}
		}
	})
	return r.closeErr
}

type Fetcher interface {
	Fetch(context.Context, string, Policy) (*Result, error)
}

type transportFactory func(context.Context, *url.URL) (http.RoundTripper, error)

type fetcher struct {
	config           Config
	transportFactory transportFactory
}

func New(config Config) Fetcher {
	config = normalizeConfig(config)
	networkDialer := &net.Dialer{}
	instance := &fetcher{config: config}
	instance.transportFactory = func(ctx context.Context, target *url.URL) (http.RoundTripper, error) {
		return newPinnedTransport(ctx, target, net.DefaultResolver, networkDialer, config)
	}
	return instance
}

func normalizeConfig(config Config) Config {
	defaults := DefaultConfig()
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = defaults.ConnectTimeout
	}
	if config.ResponseHeaderTimeout <= 0 {
		config.ResponseHeaderTimeout = defaults.ResponseHeaderTimeout
	}
	if config.IdleReadTimeout <= 0 {
		config.IdleReadTimeout = defaults.IdleReadTimeout
	}
	if config.TotalTimeout <= 0 {
		config.TotalTimeout = defaults.TotalTimeout
	}
	return config
}

func (f *fetcher) Fetch(ctx context.Context, rawURL string, policy Policy) (*Result, error) {
	if f == nil {
		return nil, newFetchStatusError(ErrorSourceTemporary, 0, true)
	}
	operationCtx, cancel := context.WithTimeout(ctx, f.config.TotalTimeout)
	defer cancel()
	response, err := f.fetchResponse(operationCtx, rawURL, policy)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return storeVerifiedMedia(operationCtx, response, policy, f.config.TempDir)
}

func (f *fetcher) fetchResponse(ctx context.Context, rawURL string, policy Policy) (*http.Response, error) {
	if f == nil || f.transportFactory == nil {
		return nil, newFetchStatusError(ErrorSourceTemporary, 0, true)
	}
	current, err := parseWebURL(rawURL)
	if err != nil {
		return nil, err
	}
	maxRedirects := policy.MaxRedirects
	if maxRedirects == 0 {
		maxRedirects = 5
	}
	if maxRedirects < 0 {
		return nil, newFetchError(ErrorRedirectRejected)
	}

	visited := map[[sha256.Size]byte]struct{}{canonicalURLHash(current): {}}
	redirects := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, mapRequestError(err)
		}
		transport, err := f.transportFactory(ctx, current)
		if err != nil {
			return nil, sanitizeFetchError(err)
		}
		client := &http.Client{
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, current.String(), nil)
		if err != nil {
			return nil, newFetchError(ErrorInvalidURL)
		}
		request.Header.Set("Accept", "image/*,video/*;q=0.9")
		response, err := client.Do(request)
		if err != nil {
			return nil, mapRequestError(err)
		}
		if !isRedirectStatus(response.StatusCode) {
			return response, nil
		}
		_ = response.Body.Close()
		if redirects >= maxRedirects {
			return nil, newFetchError(ErrorRedirectRejected)
		}
		location := strings.TrimSpace(response.Header.Get("Location"))
		if location == "" {
			return nil, newFetchError(ErrorRedirectRejected)
		}
		next, err := resolveRedirect(current, location)
		if err != nil {
			return nil, err
		}
		if current.Scheme == "https" && next.Scheme != "https" {
			return nil, newFetchError(ErrorRedirectRejected)
		}
		hash := canonicalURLHash(next)
		if _, repeated := visited[hash]; repeated {
			return nil, newFetchError(ErrorRedirectRejected)
		}
		visited[hash] = struct{}{}
		current = next
		redirects++
	}
}

func resolveRedirect(current *url.URL, location string) (*url.URL, error) {
	reference, err := url.Parse(location)
	if err != nil {
		return nil, newFetchError(ErrorRedirectRejected)
	}
	resolved := current.ResolveReference(reference)
	parsed, err := parseWebURL(resolved.String())
	if err == nil {
		return parsed, nil
	}
	var fetchErr *FetchError
	if errors.As(err, &fetchErr) && fetchErr.Kind == ErrorForbiddenDestination {
		return nil, err
	}
	return nil, newFetchError(ErrorRedirectRejected)
}

func canonicalURLHash(target *url.URL) [sha256.Size]byte {
	canonical := strings.ToLower(target.Scheme) + "\x00" +
		strings.ToLower(target.Host) + "\x00" + target.EscapedPath() + "\x00" + target.RawQuery
	return sha256.Sum256([]byte(canonical))
}

func isRedirectStatus(status int) bool {
	switch status {
	case http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func mapRequestError(err error) error {
	if err == nil {
		return nil
	}
	var fetchErr *FetchError
	if errors.As(err, &fetchErr) {
		return fetchErr
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
		return newFetchStatusError(ErrorTimeout, 0, true)
	}
	return newFetchStatusError(ErrorSourceTemporary, 0, true)
}

func sanitizeFetchError(err error) error {
	var fetchErr *FetchError
	if errors.As(err, &fetchErr) {
		return fetchErr
	}
	return mapRequestError(err)
}

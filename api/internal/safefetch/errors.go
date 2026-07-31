package safefetch

// ErrorKind is a stable, provider-neutral classification for remote-media
// retrieval failures. It is safe to persist and map into product errors.
type ErrorKind string

const (
	ErrorInvalidURL           ErrorKind = "invalid_url"
	ErrorForbiddenDestination ErrorKind = "forbidden_destination"
	ErrorRedirectRejected     ErrorKind = "redirect_rejected"
	ErrorSourceNotFound       ErrorKind = "source_not_found"
	ErrorSourceRejected       ErrorKind = "source_rejected"
	ErrorSourceTemporary      ErrorKind = "source_temporary"
	ErrorTimeout              ErrorKind = "timeout"
	ErrorTooLarge             ErrorKind = "too_large"
	ErrorUnsupportedMedia     ErrorKind = "unsupported_media"
	ErrorPeerMismatch         ErrorKind = "peer_mismatch"
)

// FetchError intentionally omits URLs, DNS answers, peer addresses, and
// provider response bodies. Its Error method returns fixed text by kind.
type FetchError struct {
	Kind       ErrorKind
	HTTPStatus int
	Temporary  bool
}

func (e *FetchError) Error() string {
	if e == nil {
		return "remote media request failed"
	}
	if message, ok := fetchErrorMessages[e.Kind]; ok {
		return message
	}
	return "remote media request failed"
}

var fetchErrorMessages = map[ErrorKind]string{
	ErrorInvalidURL:           "remote media URL is invalid",
	ErrorForbiddenDestination: "remote media destination is not allowed",
	ErrorRedirectRejected:     "remote media redirect was rejected",
	ErrorSourceNotFound:       "remote media source was not found",
	ErrorSourceRejected:       "remote media source was rejected",
	ErrorSourceTemporary:      "remote media source is temporarily unavailable",
	ErrorTimeout:              "remote media request timed out",
	ErrorTooLarge:             "remote media exceeds the size limit",
	ErrorUnsupportedMedia:     "remote media type is unsupported",
	ErrorPeerMismatch:         "remote media peer validation failed",
}

func newFetchError(kind ErrorKind) error {
	return &FetchError{Kind: kind}
}

func newFetchStatusError(kind ErrorKind, status int, temporary bool) error {
	return &FetchError{Kind: kind, HTTPStatus: status, Temporary: temporary}
}

var _ error = (*FetchError)(nil)

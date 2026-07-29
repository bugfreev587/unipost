package requestevents

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appmw "github.com/xiaoboyu/unipost-api/internal/middleware"
)

const detailPayloadBudget = 68 * 1024

type Event struct {
	ID              string
	WorkspaceID     string
	APIKeyID        string
	OccurredAt      time.Time
	RequestID       string
	TraceID         string
	Method          string
	RoutePattern    string
	StatusCode      int
	DurationMS      int
	Outcome         Outcome
	ErrorCode       string
	ProfileID       string
	SocialAccountID string
	PostID          string
	HasErrorDetail  bool
}

type Detail struct {
	EventID                string
	OccurredAt             time.Time
	WorkspaceID            string
	RequestHeaders         map[string]string
	ResponseHeaders        map[string]string
	QueryKeys              []string
	RequestPayload         string
	ResponsePayload        string
	RequestOriginalBytes   int64
	RequestStoredBytes     int
	ResponseOriginalBytes  int64
	ResponseStoredBytes    int
	RequestContentType     string
	ResponseContentType    string
	RequestSHA256          string
	ResponseSHA256         string
	RequestTruncated       bool
	ResponseTruncated      bool
	RequestOmissionReason  string
	ResponseOmissionReason string
	RedactionPolicyVersion int
}

type Enqueuer interface {
	Enqueue(Event, *Detail)
}

func Middleware(enqueuer Enqueuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if enqueuer == nil || shouldSkipPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			workspaceID := auth.GetWorkspaceID(r.Context())
			apiKeyID := auth.GetAPIKeyID(r.Context())
			if workspaceID == "" || apiKeyID == "" {
				next.ServeHTTP(w, r)
				return
			}

			requestCapture := newObservedBody(r.Body, r.Header.Get("Content-Type"))
			if r.Body != nil {
				r.Body = requestCapture
			}
			responseCapture := &observedResponseWriter{ResponseWriter: w, status: http.StatusOK}
			startedAt := time.Now()
			next.ServeHTTP(responseCapture, r)
			occurredAt := time.Now().UTC()

			errorCode := extractErrorCode(responseCapture.capturedBytes())
			event := Event{
				ID:           newEventID(occurredAt),
				WorkspaceID:  workspaceID,
				APIKeyID:     apiKeyID,
				OccurredAt:   occurredAt,
				RequestID:    appmw.GetRequestID(r.Context()),
				TraceID:      traceIDFromRequest(r),
				Method:       normalizeMethod(r.Method),
				RoutePattern: NormalizeRoute(chi.RouteContext(r.Context()).RoutePattern(), r.URL.Path),
				StatusCode:   responseCapture.status,
				DurationMS:   int(time.Since(startedAt).Milliseconds()),
				Outcome:      NormalizeOutcome(responseCapture.status, errorCode),
				ErrorCode:    normalizeToken(errorCode),
			}

			detail := buildDetail(event, r, requestCapture, responseCapture)
			event.HasErrorDetail = detail != nil
			enqueuer.Enqueue(event, detail)
		})
	}
}

func newEventID(at time.Time) string {
	return fmt.Sprintf("%020d_%s", at.UnixMicro(), uuid.NewString())
}

func shouldSkipPath(path string) bool {
	for _, skipped := range []string{"/v1/api-metrics", "/v1/admin", "/v1/me", "/v1/public", "/health"} {
		if path == skipped || strings.HasPrefix(path, skipped+"/") {
			return true
		}
	}
	return strings.HasPrefix(path, "/webhooks/") || strings.HasSuffix(path, "/ws")
}

type observedBody struct {
	body        io.ReadCloser
	contentType string
	buffer      bytes.Buffer
	total       int64
	hasher      hash.Hash
	hashedBytes int
	captureText bool
}

func newObservedBody(body io.ReadCloser, contentType string) *observedBody {
	mediaType := normalizedMediaType(contentType)
	observed := &observedBody{
		body:        body,
		contentType: contentType,
		captureText: isJSONMediaType(mediaType) || isAllowedTextMediaType(mediaType),
	}
	if isBinaryOrMultipart(mediaType) {
		observed.hasher = sha256.New()
	}
	return observed
}

func (b *observedBody) Read(target []byte) (int, error) {
	if b == nil || b.body == nil {
		return 0, io.EOF
	}
	n, err := b.body.Read(target)
	if n > 0 {
		chunk := target[:n]
		b.total += int64(n)
		if b.hasher != nil && b.hashedBytes < maxPayloadBytes {
			remaining := maxPayloadBytes - b.hashedBytes
			if remaining > n {
				remaining = n
			}
			_, _ = b.hasher.Write(chunk[:remaining])
			b.hashedBytes += remaining
		}
		if b.captureText && b.buffer.Len() < maxPayloadBytes {
			remaining := maxPayloadBytes - b.buffer.Len()
			if remaining > n {
				remaining = n
			}
			_, _ = b.buffer.Write(chunk[:remaining])
		}
	}
	return n, err
}

func (b *observedBody) Close() error {
	if b == nil || b.body == nil {
		return nil
	}
	return b.body.Close()
}

func (b *observedBody) sha256() string {
	if b == nil || b.total == 0 || b.hasher == nil || b.total != int64(b.hashedBytes) {
		return ""
	}
	return hex.EncodeToString(b.hasher.Sum(nil))
}

type observedResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	body        bytes.Buffer
	total       int64
	hasher      hash.Hash
	hashedBytes int
}

func (w *observedResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *observedResponseWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	if status >= 400 && isBinaryOrMultipart(normalizedMediaType(w.Header().Get("Content-Type"))) {
		w.hasher = sha256.New()
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *observedResponseWriter) Write(payload []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.status >= 400 {
		w.total += int64(len(payload))
		if w.hasher != nil && w.hashedBytes < maxPayloadBytes {
			remaining := maxPayloadBytes - w.hashedBytes
			if remaining > len(payload) {
				remaining = len(payload)
			}
			_, _ = w.hasher.Write(payload[:remaining])
			w.hashedBytes += remaining
		}
		mediaType := normalizedMediaType(w.Header().Get("Content-Type"))
		if (isJSONMediaType(mediaType) || isAllowedTextMediaType(mediaType)) && w.body.Len() < maxPayloadBytes {
			remaining := maxPayloadBytes - w.body.Len()
			if remaining > len(payload) {
				remaining = len(payload)
			}
			_, _ = w.body.Write(payload[:remaining])
		}
	}
	return w.ResponseWriter.Write(payload)
}

func (w *observedResponseWriter) ReadFrom(reader io.Reader) (int64, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.status < 400 {
		if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
			return readerFrom.ReadFrom(reader)
		}
		return io.Copy(w.ResponseWriter, reader)
	}
	return io.Copy(struct{ io.Writer }{w}, reader)
}

func (w *observedResponseWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *observedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return http.NewResponseController(w.ResponseWriter).Hijack()
}

func (w *observedResponseWriter) Push(target string, options *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, options)
	}
	return http.ErrNotSupported
}

func (w *observedResponseWriter) capturedBytes() []byte {
	if w == nil {
		return nil
	}
	return w.body.Bytes()
}

func (w *observedResponseWriter) sha256() string {
	if w == nil || w.total == 0 || w.hasher == nil || w.total != int64(w.hashedBytes) {
		return ""
	}
	return hex.EncodeToString(w.hasher.Sum(nil))
}

func buildDetail(event Event, request *http.Request, requestBody *observedBody, response *observedResponseWriter) *Detail {
	if event.StatusCode < 400 || event.StatusCode == http.StatusPaymentRequired {
		return nil
	}

	requestPayload := sanitizePayload(request.Header.Get("Content-Type"), requestBody.buffer.Bytes(), requestBody.total)
	requestPayload.SHA256 = requestBody.sha256()
	if isBinaryOrMultipart(normalizedMediaType(request.Header.Get("Content-Type"))) {
		if requestPayload.SHA256 != "" {
			requestPayload.OmissionReason = removeOmissionReason(requestPayload.OmissionReason, "hash_omitted_size")
		} else if requestBody.total > maxPayloadBytes {
			requestPayload.OmissionReason = appendOmissionReason(requestPayload.OmissionReason, "hash_omitted_size")
		}
	}
	if event.StatusCode == http.StatusTooManyRequests {
		requestPayload.Payload = ""
		requestPayload.StoredBytes = 0
		requestPayload.OmissionReason = "rate_limit_request_omitted"
	}
	responsePayload := sanitizePayload(response.Header().Get("Content-Type"), response.body.Bytes(), response.total)
	responsePayload.SHA256 = response.sha256()
	if isBinaryOrMultipart(normalizedMediaType(response.Header().Get("Content-Type"))) {
		if responsePayload.SHA256 != "" {
			responsePayload.OmissionReason = removeOmissionReason(responsePayload.OmissionReason, "hash_omitted_size")
		} else if response.total > maxPayloadBytes {
			responsePayload.OmissionReason = appendOmissionReason(responsePayload.OmissionReason, "hash_omitted_size")
		}
	}

	detail := &Detail{
		EventID:                event.ID,
		OccurredAt:             event.OccurredAt,
		WorkspaceID:            event.WorkspaceID,
		RequestHeaders:         allowlistedHeaders(request.Header),
		ResponseHeaders:        allowlistedHeaders(response.Header()),
		QueryKeys:              queryKeySummary(request.URL.RawQuery),
		RequestPayload:         requestPayload.Payload,
		ResponsePayload:        responsePayload.Payload,
		RequestOriginalBytes:   requestPayload.OriginalBytes,
		RequestStoredBytes:     requestPayload.StoredBytes,
		ResponseOriginalBytes:  responsePayload.OriginalBytes,
		ResponseStoredBytes:    responsePayload.StoredBytes,
		RequestContentType:     normalizedMediaType(request.Header.Get("Content-Type")),
		ResponseContentType:    normalizedMediaType(response.Header().Get("Content-Type")),
		RequestSHA256:          requestPayload.SHA256,
		ResponseSHA256:         responsePayload.SHA256,
		RequestTruncated:       requestPayload.Truncated,
		ResponseTruncated:      responsePayload.Truncated,
		RequestOmissionReason:  requestPayload.OmissionReason,
		ResponseOmissionReason: responsePayload.OmissionReason,
		RedactionPolicyVersion: redactionPolicyVersion,
	}
	boundDetail(detail)
	return detail
}

func boundDetail(detail *Detail) {
	if detail == nil {
		return
	}
	headerBytes, _ := json.Marshal(struct {
		Request  map[string]string
		Response map[string]string
		Query    []string
	}{detail.RequestHeaders, detail.ResponseHeaders, detail.QueryKeys})
	available := detailPayloadBudget - len(headerBytes)
	if available < 0 {
		available = 0
	}
	if len(detail.RequestPayload)+len(detail.ResponsePayload) <= available {
		return
	}
	responseLimit := available / 2
	detail.ResponsePayload = truncateUTF8(detail.ResponsePayload, responseLimit)
	detail.ResponseStoredBytes = len(detail.ResponsePayload)
	detail.ResponseTruncated = true
	requestLimit := available - detail.ResponseStoredBytes
	detail.RequestPayload = truncateUTF8(detail.RequestPayload, requestLimit)
	detail.RequestStoredBytes = len(detail.RequestPayload)
	detail.RequestTruncated = true
}

func extractErrorCode(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		return ""
	}
	for _, key := range []string{"code", "error_code"} {
		if code, ok := value[key].(string); ok && applicationErrorCodePattern.MatchString(code) {
			return code
		}
	}
	if nested, ok := value["error"].(map[string]any); ok {
		for _, key := range []string{"code", "error_code"} {
			if code, ok := nested[key].(string); ok && applicationErrorCodePattern.MatchString(code) {
				return code
			}
		}
	}
	return ""
}

func traceIDFromRequest(request *http.Request) string {
	traceParent := strings.TrimSpace(request.Header.Get("Traceparent"))
	if len(traceParent) > 128 {
		traceParent = ""
	}
	parts := strings.Split(traceParent, "-")
	if len(parts) >= 4 && len(parts[1]) == 32 && traceIDPattern.MatchString(parts[1]) && parts[1] != strings.Repeat("0", 32) {
		return strings.ToLower(parts[1])
	}
	traceID := strings.TrimSpace(request.Header.Get("X-Trace-Id"))
	if len(traceID) > 64 {
		return ""
	}
	if traceIDPattern.MatchString(traceID) {
		return strings.ToLower(traceID)
	}
	return ""
}

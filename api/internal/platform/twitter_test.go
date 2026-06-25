package platform

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type twitterRoundTripFunc func(*http.Request) (*http.Response, error)

func (f twitterRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTwitterUploadMediaChunkedNormalizesParameterizedVideoContentType(t *testing.T) {
	var sawInit bool
	var sawFinalize bool

	adapter := &TwitterAdapter{client: &http.Client{Transport: twitterRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Host == "media.example.com":
			return textResponse(http.StatusOK, "fake-mp4-bytes", map[string]string{
				"Content-Type": "video/mp4;codecs=avc1",
			}), nil
		case req.URL.Host == "api.x.com" && req.URL.Path == "/2/media/upload":
			if req.URL.Query().Get("media_type") != "" || req.URL.Query().Get("media_category") != "" {
				t.Fatalf("INIT metadata must not be sent as query params: %s", req.URL.RawQuery)
			}
			form, err := readMultipartFields(req)
			if err != nil {
				t.Fatalf("read multipart fields: %v", err)
			}
			switch form["command"] {
			case "INIT":
				sawInit = true
				if got := form["media_type"]; got != "video/mp4" {
					t.Fatalf("media_type = %q, want video/mp4", got)
				}
				if got := form["media_category"]; got != "tweet_video" {
					t.Fatalf("media_category = %q, want tweet_video", got)
				}
				if got := form["total_bytes"]; got != "14" {
					t.Fatalf("total_bytes = %q, want 14", got)
				}
				return jsonResponse(http.StatusOK, `{"data":{"id":"media123"}}`), nil
			case "APPEND":
				if got := form["media_id"]; got != "media123" {
					t.Fatalf("append media_id = %q, want media123", got)
				}
				return jsonResponse(http.StatusOK, `{}`), nil
			case "FINALIZE":
				sawFinalize = true
				if got := form["media_id"]; got != "media123" {
					t.Fatalf("finalize media_id = %q, want media123", got)
				}
				return jsonResponse(http.StatusOK, `{"data":{"id":"media123"}}`), nil
			default:
				t.Fatalf("unexpected upload command %q", form["command"])
			}
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		return nil, nil
	})}}

	mediaID, err := adapter.uploadMedia(context.Background(), "token", MediaItem{
		URL:  "https://media.example.com/video.mp4",
		Kind: MediaKindVideo,
	})
	if err != nil {
		t.Fatalf("uploadMedia: %v", err)
	}
	if mediaID != "media123" {
		t.Fatalf("mediaID = %q, want media123", mediaID)
	}
	if !sawInit {
		t.Fatal("did not see INIT request")
	}
	if !sawFinalize {
		t.Fatal("did not see FINALIZE request")
	}
}

func readMultipartFields(req *http.Request) (map[string]string, error) {
	mediaType := req.Header.Get("Content-Type")
	if !strings.HasPrefix(mediaType, "multipart/form-data") {
		return nil, fmt.Errorf("content type = %q, want multipart/form-data", mediaType)
	}
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		return nil, err
	}
	fields := make(map[string]string)
	for key, values := range req.MultipartForm.Value {
		if len(values) > 0 {
			fields[key] = values[0]
		}
	}
	return fields, nil
}

func jsonResponse(status int, body string) *http.Response {
	return textResponse(status, body, map[string]string{"Content-Type": "application/json"})
}

func textResponse(status int, body string, headers map[string]string) *http.Response {
	h := make(http.Header)
	for key, value := range headers {
		h.Set(key, value)
	}
	return &http.Response{
		StatusCode: status,
		Header:     h,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

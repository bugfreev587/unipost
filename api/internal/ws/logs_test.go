package ws

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/requestevents"
)

func TestLogEnvelopeAlwaysUsesLegacyNumericProducerIdentity(t *testing.T) {
	row := db.IntegrationLog{
		ID: 42, WorkspaceID: "workspace-1", Ts: pgtype.Timestamptz{Time: time.Unix(100, 0).UTC(), Valid: true},
		Level: "error", Status: "error", Category: "oauth", Action: "oauth.failed", Source: "oauth", Message: "failed",
	}

	envelope := LogEnvelope(row)
	if envelope["log"].(map[string]any)["id"] != int64(42) {
		t.Fatalf("producer envelope = %#v", envelope)
	}
}

func TestProjectLogEnvelopeAppliesModeAtConsumerBoundary(t *testing.T) {
	legacyAPI, _ := json.Marshal(LogEnvelope(db.IntegrationLog{ID: 42, WorkspaceID: "workspace-1", Category: "api_request"}))
	legacyOAuth, _ := json.Marshal(LogEnvelope(db.IntegrationLog{ID: 43, WorkspaceID: "workspace-1", Category: "oauth"}))
	canonical, _ := json.Marshal(RequestEventLogEnvelope(requestevents.Event{ID: "event-1", WorkspaceID: "workspace-1"}))

	assertProjectedLogID(t, legacyAPI, false, int64(42), true)
	assertProjectedLogID(t, legacyAPI, true, nil, false)
	assertProjectedLogID(t, legacyOAuth, false, int64(43), true)
	assertProjectedLogID(t, legacyOAuth, true, "integration:43", true)
	assertProjectedLogID(t, canonical, false, nil, false)
	assertProjectedLogID(t, canonical, true, "request:event-1", true)
}

func assertProjectedLogID(t *testing.T, input []byte, useV2 bool, want any, wantOK bool) {
	t.Helper()
	projected, ok := ProjectLogEnvelope(input, useV2)
	if ok != wantOK {
		t.Fatalf("ProjectLogEnvelope(useV2=%v) ok=%v want %v; payload=%s", useV2, ok, wantOK, projected)
	}
	if !ok {
		return
	}
	var envelope struct {
		Log map[string]any `json:"log"`
	}
	decoder := json.NewDecoder(bytes.NewReader(projected))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	got := envelope.Log["id"]
	if number, ok := got.(json.Number); ok {
		parsed, _ := number.Int64()
		got = parsed
	}
	if got != want {
		t.Fatalf("projected id = %#v, want %#v", got, want)
	}
}

func TestRequestEventLogEnvelopeUsesCanonicalIdentity(t *testing.T) {
	event := requestevents.Event{
		ID: "00000000000100000000_event-1", WorkspaceID: "workspace-1", APIKeyID: "key-1",
		OccurredAt: time.Unix(100, 0).UTC(), RequestID: "request-1", Method: "POST", RoutePattern: "/v1/test",
		StatusCode: 500, DurationMS: 12, Outcome: requestevents.OutcomeServerError, ErrorCode: "internal_error", HasErrorDetail: true,
	}
	envelope := RequestEventLogEnvelope(event)
	log := envelope["log"].(map[string]any)
	if log["id"] != "request:"+event.ID || log["category"] != "api_request" || log["has_error_detail"] != true {
		t.Fatalf("request envelope = %#v", envelope)
	}
}

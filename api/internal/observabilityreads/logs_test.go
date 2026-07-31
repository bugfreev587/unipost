package observabilityreads

import (
	"reflect"
	"testing"
	"time"
)

func TestLogIDRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		kind    LogSourceKind
		local   string
		encoded string
	}{
		{name: "request", kind: LogSourceRequest, local: "019cafe0-1234-7abc-9def-0123456789ab", encoded: "request:019cafe0-1234-7abc-9def-0123456789ab"},
		{name: "integration", kind: LogSourceIntegration, local: "42", encoded: "integration:42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeLogID(tt.kind, tt.local)
			if err != nil {
				t.Fatalf("EncodeLogID() error = %v", err)
			}
			if encoded != tt.encoded {
				t.Fatalf("EncodeLogID() = %q, want %q", encoded, tt.encoded)
			}

			kind, local, err := DecodeLogID(encoded)
			if err != nil {
				t.Fatalf("DecodeLogID() error = %v", err)
			}
			if kind != tt.kind || local != tt.local {
				t.Fatalf("DecodeLogID() = (%q, %q), want (%q, %q)", kind, local, tt.kind, tt.local)
			}
		})
	}
}

func TestLogIDRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{name: "unqualified legacy ID", id: "42"},
		{name: "unknown source", id: "other:42"},
		{name: "empty local ID", id: "request:"},
		{name: "nonnumeric integration ID", id: "integration:not-a-number"},
		{name: "negative integration ID", id: "integration:-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := DecodeLogID(tt.id); err == nil {
				t.Fatalf("DecodeLogID(%q) unexpectedly succeeded", tt.id)
			}
		})
	}
}

func TestLogIDRejectsNonCanonicalIntegrationIDs(t *testing.T) {
	for _, sourceID := range []string{"01", "001", "+1", "+42"} {
		t.Run(sourceID, func(t *testing.T) {
			if _, err := EncodeLogID(LogSourceIntegration, sourceID); err == nil {
				t.Fatalf("EncodeLogID(integration, %q) unexpectedly succeeded", sourceID)
			}
			if _, _, err := DecodeLogID("integration:" + sourceID); err == nil {
				t.Fatalf("DecodeLogID(integration:%s) unexpectedly succeeded", sourceID)
			}
		})
	}
}

func TestDecodeLogLookupIDAcceptsQualifiedAndCanonicalLegacyIntegrationIDs(t *testing.T) {
	for _, tt := range []struct {
		name      string
		input     string
		wantKind  LogSourceKind
		wantLocal string
	}{
		{name: "qualified request", input: "request:event-1", wantKind: LogSourceRequest, wantLocal: "event-1"},
		{name: "qualified integration", input: "integration:42", wantKind: LogSourceIntegration, wantLocal: "42"},
		{name: "legacy integration", input: "42", wantKind: LogSourceIntegration, wantLocal: "42"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			kind, local, err := DecodeLogLookupID(tt.input)
			if err != nil {
				t.Fatalf("DecodeLogLookupID(%q) error = %v", tt.input, err)
			}
			if kind != tt.wantKind || local != tt.wantLocal {
				t.Fatalf("DecodeLogLookupID(%q) = (%q, %q), want (%q, %q)", tt.input, kind, local, tt.wantKind, tt.wantLocal)
			}
		})
	}
}

func TestDecodeLogLookupIDRejectsInvalidLegacyIntegrationIDs(t *testing.T) {
	for _, id := range []string{"", "0", "-1", "+1", "01", " 1", "1 ", "9223372036854775808"} {
		t.Run(id, func(t *testing.T) {
			if _, _, err := DecodeLogLookupID(id); err == nil {
				t.Fatalf("DecodeLogLookupID(%q) unexpectedly succeeded", id)
			}
		})
	}
}

func TestCursorRoundTripAndOpacity(t *testing.T) {
	want := LogCursor{
		Timestamp:  time.Date(2026, time.July, 29, 12, 34, 56, 789000000, time.UTC),
		SourceKind: LogSourceRequest,
		SourceID:   "019cafe0-1234-7abc-9def-0123456789ab",
	}

	encoded, err := EncodeLogCursor(want)
	if err != nil {
		t.Fatalf("EncodeLogCursor() error = %v", err)
	}
	if encoded == "" || encoded == want.SourceID || encoded == want.Timestamp.Format(time.RFC3339Nano) {
		t.Fatalf("EncodeLogCursor() returned a non-opaque cursor %q", encoded)
	}

	got, err := DecodeLogCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeLogCursor() error = %v", err)
	}
	if !got.Timestamp.Equal(want.Timestamp) || got.SourceKind != want.SourceKind || got.SourceID != want.SourceID {
		t.Fatalf("DecodeLogCursor() = %#v, want %#v", got, want)
	}
}

func TestCursorRejectsInvalidValues(t *testing.T) {
	valid, err := EncodeLogCursor(LogCursor{
		Timestamp:  time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC),
		SourceKind: LogSourceIntegration,
		SourceID:   "7",
	})
	if err != nil {
		t.Fatalf("EncodeLogCursor() error = %v", err)
	}

	tests := []struct {
		name   string
		cursor string
	}{
		{name: "empty", cursor: ""},
		{name: "not base64", cursor: "v1.not/base64"},
		{name: "unsupported version", cursor: "v2." + valid[len("v1."):]},
		{name: "trailing data", cursor: valid + "extra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeLogCursor(tt.cursor); err == nil {
				t.Fatalf("DecodeLogCursor(%q) unexpectedly succeeded", tt.cursor)
			}
		})
	}
}

func TestMergeOrdersIdenticalTimestamps(t *testing.T) {
	ts := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	requests := []LogProjection{
		logRecord(LogSourceRequest, "a", ts),
		logRecord(LogSourceRequest, "c", ts),
	}
	integrations := []LogProjection{
		logRecord(LogSourceIntegration, "2", ts),
		logRecord(LogSourceIntegration, "10", ts),
	}

	page, err := MergeLogPage(requests, integrations, nil, 10)
	if err != nil {
		t.Fatalf("MergeLogPage() error = %v", err)
	}
	want := []string{"request:c", "request:a", "integration:10", "integration:2"}
	if got := projectionIDs(page.Logs); !reflect.DeepEqual(got, want) {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
}

func TestMergePagesWithoutDuplicatesOrGaps(t *testing.T) {
	base := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	allRequests := []LogProjection{
		logRecord(LogSourceRequest, "r5", base.Add(5*time.Minute)),
		logRecord(LogSourceRequest, "r3", base.Add(3*time.Minute)),
		logRecord(LogSourceRequest, "r1", base.Add(time.Minute)),
	}
	allIntegrations := []LogProjection{
		logRecord(LogSourceIntegration, "4", base.Add(4*time.Minute)),
		logRecord(LogSourceIntegration, "2", base.Add(2*time.Minute)),
		logRecord(LogSourceIntegration, "1", base),
	}

	tests := []struct {
		name         string
		requests     []LogProjection
		integrations []LogProjection
		pageSizes    []int
		deleteAfter  string
		want         []string
	}{
		{
			name:      "request only with page size transition",
			requests:  allRequests,
			pageSizes: []int{1, 2},
			want:      []string{"request:r5", "request:r3", "request:r1"},
		},
		{
			name:         "integration only with page size transition",
			integrations: allIntegrations,
			pageSizes:    []int{2, 1},
			want:         []string{"integration:4", "integration:2", "integration:1"},
		},
		{
			name:         "deletion between mixed pages",
			requests:     allRequests,
			integrations: allIntegrations,
			pageSizes:    []int{2, 3},
			deleteAfter:  "request:r5",
			want:         []string{"request:r5", "integration:4", "request:r3", "integration:2", "request:r1", "integration:1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := append([]LogProjection(nil), tt.requests...)
			integrations := append([]LogProjection(nil), tt.integrations...)
			var cursor *LogCursor
			var got []string
			for pageIndex := 0; ; pageIndex++ {
				pageSize := tt.pageSizes[pageIndex%len(tt.pageSizes)]
				page, err := MergeLogPage(requests, integrations, cursor, pageSize)
				if err != nil {
					t.Fatalf("page %d: MergeLogPage() error = %v", pageIndex+1, err)
				}
				got = append(got, projectionIDs(page.Logs)...)

				if pageIndex == 0 && tt.deleteAfter != "" {
					requests = deleteProjection(requests, tt.deleteAfter)
					integrations = deleteProjection(integrations, tt.deleteAfter)
				}
				if page.NextCursor == nil {
					break
				}
				cursor = page.NextCursor
				if pageIndex > len(tt.want) {
					t.Fatal("pagination did not terminate")
				}
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("consecutive page IDs = %v, want %v", got, tt.want)
			}
			assertUnique(t, got)
		})
	}
}

func TestMergeUsesLimitPlusOne(t *testing.T) {
	base := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	logs := []LogProjection{
		logRecord(LogSourceRequest, "3", base.Add(3*time.Minute)),
		logRecord(LogSourceRequest, "2", base.Add(2*time.Minute)),
		logRecord(LogSourceRequest, "1", base.Add(time.Minute)),
	}

	first, err := MergeLogPage(logs, nil, nil, 2)
	if err != nil {
		t.Fatalf("MergeLogPage() error = %v", err)
	}
	if got := projectionIDs(first.Logs); !reflect.DeepEqual(got, []string{"request:3", "request:2"}) {
		t.Fatalf("first page IDs = %v", got)
	}
	if first.NextCursor == nil || first.NextCursor.SourceID != "2" {
		t.Fatalf("first page cursor = %#v, want cursor for final returned row", first.NextCursor)
	}

	last, err := MergeLogPage(logs, nil, first.NextCursor, 2)
	if err != nil {
		t.Fatalf("MergeLogPage() second page error = %v", err)
	}
	if got := projectionIDs(last.Logs); !reflect.DeepEqual(got, []string{"request:1"}) {
		t.Fatalf("last page IDs = %v", got)
	}
	if last.NextCursor != nil {
		t.Fatalf("last page cursor = %#v, want nil", last.NextCursor)
	}
}

func TestMergePagesAcrossSourceBoundaryAtSameTimestamp(t *testing.T) {
	ts := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	requests := []LogProjection{
		logRecord(LogSourceRequest, "r3", ts),
		logRecord(LogSourceRequest, "r2", ts),
		logRecord(LogSourceRequest, "r1", ts),
	}
	integrations := []LogProjection{
		logRecord(LogSourceIntegration, "3", ts),
		logRecord(LogSourceIntegration, "2", ts),
		logRecord(LogSourceIntegration, "1", ts),
	}

	var cursor *LogCursor
	var got []string
	for pageNumber := 1; ; pageNumber++ {
		page, err := MergeLogPage(requests, integrations, cursor, 2)
		if err != nil {
			t.Fatalf("page %d: MergeLogPage() error = %v", pageNumber, err)
		}
		got = append(got, projectionIDs(page.Logs)...)
		if page.NextCursor == nil {
			break
		}
		cursor = page.NextCursor
		if pageNumber > 3 {
			t.Fatal("pagination did not terminate")
		}
	}

	want := []string{
		"request:r3", "request:r2", "request:r1",
		"integration:3", "integration:2", "integration:1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("consecutive page IDs = %v, want %v", got, want)
	}
	assertUnique(t, got)
}

func TestMergeComparatorTreatsEqualCanonicalIDsAsEqual(t *testing.T) {
	ts := time.Date(2026, time.July, 29, 12, 0, 0, 0, time.UTC)
	equal := logRecord(LogSourceIntegration, "3", ts)
	if got := CompareLogProjections(equal, equal); got != 0 {
		t.Fatalf("CompareLogProjections(equal, equal) = %d, want 0", got)
	}
}

func logRecord(source LogSourceKind, sourceID string, timestamp time.Time) LogProjection {
	id, err := EncodeLogID(source, sourceID)
	if err != nil {
		panic(err)
	}
	return LogProjection{ID: id, SourceKind: source, SourceID: sourceID, Timestamp: timestamp}
}

func projectionIDs(logs []LogProjection) []string {
	ids := make([]string, len(logs))
	for i := range logs {
		ids[i] = logs[i].ID
	}
	return ids
}

func deleteProjection(logs []LogProjection, id string) []LogProjection {
	result := make([]LogProjection, 0, len(logs))
	for _, log := range logs {
		if log.ID != id {
			result = append(result, log)
		}
	}
	return result
}

func assertUnique(t *testing.T, ids []string) {
	t.Helper()
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate ID %q across consecutive pages", id)
		}
		seen[id] = struct{}{}
	}
}

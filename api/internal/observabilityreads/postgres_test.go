package observabilityreads

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/featureflags"
)

type stubPublicEvaluator struct {
	enabled bool
	err     error
	keys    []string
}

type panicOnPublicEvaluator struct{}

func (*panicOnPublicEvaluator) Public(context.Context, string) (bool, error) {
	panic("Public must not be called through a typed-nil evaluator")
}

type staticReadPathSelector bool

func (s staticReadPathSelector) UseV2(context.Context) bool { return bool(s) }

type activationLockedStore struct{ globalCalls int }

func (*activationLockedStore) List(context.Context) ([]featureflags.Flag, error) { return nil, nil }
func (*activationLockedStore) Set(context.Context, string, bool, string) (featureflags.Flag, error) {
	return featureflags.Flag{}, nil
}
func (s *activationLockedStore) GlobalEnabled(context.Context, string) (bool, error) {
	s.globalCalls++
	return true, nil
}
func (*activationLockedStore) WorkspaceOwner(context.Context, string) (string, error) { return "", nil }

func TestReadSelectorStaysLegacyWithoutStoreQueryForActivationLockedFlag(t *testing.T) {
	store := &activationLockedStore{}
	selector := NewReadSelector(featureflags.NewEvaluator(store, nil), slog.Default())
	if selector.UseV2(context.Background()) {
		t.Fatal("activation-locked stored true value selected v2")
	}
	if store.globalCalls != 0 {
		t.Fatalf("GlobalEnabled calls = %d, want 0", store.globalCalls)
	}
}

type panicLogReader struct{}

func (*panicLogReader) ListWorkspace(context.Context, string, LogFilters, *LogCursor, int) (LogPage, error) {
	panic("typed-nil LogReader must not be called")
}
func (*panicLogReader) ListAdmin(context.Context, LogFilters, *LogCursor, int) (LogPage, error) {
	panic("typed-nil LogReader must not be called")
}
func (*panicLogReader) GetWorkspace(context.Context, string, string) (LogDetail, error) {
	panic("typed-nil LogReader must not be called")
}
func (*panicLogReader) GetAdmin(context.Context, string) (LogDetail, error) {
	panic("typed-nil LogReader must not be called")
}

func TestSelectLogReaderTreatsTypedNilAsUnavailable(t *testing.T) {
	var reader LogReader = (*panicLogReader)(nil)
	if selected := SelectLogReader(context.Background(), staticReadPathSelector(true), reader); selected != nil {
		t.Fatalf("SelectLogReader() = %T, want nil legacy fallback", selected)
	}
}

func TestReadSelectorTypedNilEvaluatorFallsBackWithoutCallingPublic(t *testing.T) {
	var evaluator PublicEvaluator = (*panicOnPublicEvaluator)(nil)
	selector := NewReadSelector(evaluator, slog.Default())
	if selector.UseV2(context.Background()) {
		t.Fatal("typed-nil evaluator selected v2; want legacy fallback")
	}
}

func (s *stubPublicEvaluator) Public(_ context.Context, key string) (bool, error) {
	s.keys = append(s.keys, key)
	return s.enabled, s.err
}

func TestReadSelectorUsesOnlyGlobalPublicEvaluation(t *testing.T) {
	tests := []struct {
		name      string
		evaluator *stubPublicEvaluator
		want      bool
		wantCalls int
		wantWarn  bool
	}{
		{name: "off", evaluator: &stubPublicEvaluator{}, wantCalls: 1},
		{name: "on", evaluator: &stubPublicEvaluator{enabled: true}, want: true, wantCalls: 1},
		{name: "evaluator error falls back to legacy", evaluator: &stubPublicEvaluator{err: errors.New("flag store unavailable")}, wantCalls: 1, wantWarn: true},
		{name: "nil evaluator falls back to legacy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			selector := NewReadSelector(tt.evaluator, slog.New(slog.NewJSONHandler(&logs, nil)))
			if got := selector.UseV2(context.Background()); got != tt.want {
				t.Fatalf("UseV2() = %v, want %v", got, tt.want)
			}
			if tt.evaluator != nil {
				if len(tt.evaluator.keys) != tt.wantCalls {
					t.Fatalf("Public calls = %d, want %d", len(tt.evaluator.keys), tt.wantCalls)
				}
				for _, key := range tt.evaluator.keys {
					if key != "observability_reads_v2" {
						t.Fatalf("Public key = %q, want observability_reads_v2", key)
					}
				}
			}
			if got := strings.Contains(logs.String(), `"event":"observability_reads_v2_evaluation_failed"`); got != tt.wantWarn {
				t.Fatalf("structured warning present = %v, want %v; log=%s", got, tt.wantWarn, logs.String())
			}
		})
	}
}

func TestListQueriesAreMetadataOnlyBoundedAndSourceAware(t *testing.T) {
	for name, query := range map[string]string{
		"request":     requestLogListSQL,
		"integration": integrationLogListSQL,
	} {
		t.Run(name, func(t *testing.T) {
			lower := strings.ToLower(query)
			for _, forbidden := range []string{"request_payload", "response_payload", "request_headers", "response_headers", " offset "} {
				if strings.Contains(lower, forbidden) {
					t.Fatalf("list query contains %q: %s", forbidden, query)
				}
			}
			if !strings.Contains(lower, "limit $22") {
				t.Fatalf("list query must bound candidates per source with LIMIT $22: %s", query)
			}
			cursorIDType := "$21::text"
			if name == "integration" {
				cursorIDType = "$21::bigint"
			}
			if !strings.Contains(lower, "$20::text") || !strings.Contains(lower, cursorIDType) {
				t.Fatalf("list query lacks source-aware cursor predicate: %s", query)
			}
		})
	}
	if !strings.Contains(strings.ToLower(integrationLogListSQL), "category <> 'api_request'") {
		t.Fatal("integration list must exclude legacy api_request duplicates")
	}
}

func TestAdminGeneralSearchMatchesOwnerEmailInBothSources(t *testing.T) {
	for name, query := range map[string]string{
		"request":     requestLogListSQL,
		"integration": integrationLogListSQL,
	} {
		t.Run(name, func(t *testing.T) {
			searchStart := strings.Index(query, "$14::TEXT = ''")
			searchEnd := strings.Index(query[searchStart:], ")\n  AND ($15::TEXT")
			if searchStart < 0 || searchEnd < 0 {
				t.Fatalf("could not isolate $14 general-search predicate: %s", query)
			}
			search := strings.ToLower(query[searchStart : searchStart+searchEnd])
			if !strings.Contains(search, "u.email ilike '%' || $14 || '%'") {
				t.Fatalf("$14 general search does not match owner email: %s", search)
			}
		})
	}
}

func TestPostgresLogStoreDetailsRejectUnconfiguredStore(t *testing.T) {
	timestampedID := fmt.Sprintf("request:%020d_event", time.Now().UnixMicro())
	stores := []struct {
		name  string
		store *PostgresLogStore
	}{
		{name: "nil receiver"},
		{name: "nil database", store: NewPostgresLogStore(nil)},
	}
	for _, tt := range stores {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.store.GetWorkspace(context.Background(), "workspace", timestampedID); err == nil {
				t.Fatal("GetWorkspace() error = nil, want configuration error")
			}
			if _, err := tt.store.GetAdmin(context.Background(), timestampedID); err == nil {
				t.Fatal("GetAdmin() error = nil, want configuration error")
			}
		})
	}
}

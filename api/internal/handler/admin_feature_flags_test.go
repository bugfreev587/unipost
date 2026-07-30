package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
)

type adminFeatureFlagMutation struct {
	key     string
	enabled bool
	actor   string
}

type adminFeatureFlagAuditStore struct {
	enabled   map[string]bool
	mutations []adminFeatureFlagMutation
}

func (s *adminFeatureFlagAuditStore) List(context.Context) ([]featureflags.Flag, error) {
	return nil, nil
}

func (s *adminFeatureFlagAuditStore) Set(_ context.Context, key string, enabled bool, actor string) (featureflags.Flag, error) {
	previous := s.enabled[key]
	s.enabled[key] = enabled
	if previous != enabled {
		s.mutations = append(s.mutations, adminFeatureFlagMutation{key: key, enabled: enabled, actor: actor})
	}
	return featureflags.Flag{Key: key, Enabled: enabled, UpdatedBy: actor}, nil
}

func TestAdminCannotEnableObservabilityReadsUntilPublicSDKsAreCompatible(t *testing.T) {
	store := &adminFeatureFlagAuditStore{enabled: map[string]bool{featureflags.ObservabilityReadsV2: false}}
	h := NewFeatureFlagsHandler(store, nil)
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/feature-flags/"+featureflags.ObservabilityReadsV2, strings.NewReader(`{"enabled":true}`))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "admin_1"))
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("key", featureflags.ObservabilityReadsV2)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	rec := httptest.NewRecorder()

	h.UpdateAdmin(rec, req)

	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "FLAG_NOT_READY") {
		t.Fatalf("status=%d body=%s, want locked 409", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "until its SDK, canonical live-publisher, historical Metrics coverage/parity, retention-safety, and exact-SHA prerequisites are met") {
		t.Fatalf("body=%s, want complete activation prerequisites", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "until its read path is implemented and accepted") {
		t.Fatalf("body=%s, contains obsolete read-path prerequisite", rec.Body.String())
	}
	if len(store.mutations) != 0 || store.enabled[featureflags.ObservabilityReadsV2] {
		t.Fatalf("locked enable mutated state: enabled=%v mutations=%#v", store.enabled, store.mutations)
	}
}

func TestCustomerMeFeaturesEndpointNeverExposesInternalObservabilityReadSwitch(t *testing.T) {
	flagStore := &handlerFeatureFlagStore{flags: map[string]bool{
		featureflags.ObservabilityReadsV2: true,
	}}
	h := NewMeHandler(db.New(meFeaturesCompatDB{planID: "basic"}), nil, nil).
		SetFeatureFlags(featureflags.NewEvaluator(flagStore, nil))
	req := httptest.NewRequest(http.MethodGet, "/v1/me/features", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_1"))
	rec := httptest.NewRecorder()

	h.FeatureFlagsCompat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data struct {
			Flags map[string]bool `json:"flags"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if _, exposed := response.Data.Flags[featureflags.ObservabilityReadsV2]; exposed {
		t.Fatalf("customer feature response exposed internal observability switch: %s", rec.Body.String())
	}
}

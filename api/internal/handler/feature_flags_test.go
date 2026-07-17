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
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
)

type handlerFeatureFlagStore struct {
	flags map[string]bool
}

func (s *handlerFeatureFlagStore) List(context.Context) ([]featureflags.Flag, error) {
	result := make([]featureflags.Flag, 0, len(featureflags.Definitions()))
	for _, definition := range featureflags.Definitions() {
		result = append(result, featureflags.Flag{
			Key:         definition.Key,
			Enabled:     s.flags[definition.Key],
			Description: definition.Description,
		})
	}
	return result, nil
}

func (s *handlerFeatureFlagStore) Set(_ context.Context, key string, enabled bool, actor string) (featureflags.Flag, error) {
	if _, ok := featureflags.DefinitionFor(key); !ok {
		return featureflags.Flag{}, featureflags.ErrUnknownFlag
	}
	s.flags[key] = enabled
	return featureflags.Flag{Key: key, Enabled: enabled, UpdatedBy: actor}, nil
}

func (s *handlerFeatureFlagStore) GlobalEnabled(_ context.Context, key string) (bool, error) {
	return s.flags[key], nil
}

func (s *handlerFeatureFlagStore) WorkspaceOwner(context.Context, string) (string, error) {
	return "owner_1", nil
}

func TestFeatureFlagsHandlerListAndUpdate(t *testing.T) {
	store := &handlerFeatureFlagStore{flags: map[string]bool{}}
	h := NewFeatureFlagsHandler(store, featureflags.NewEvaluator(store, nil))

	listReq := httptest.NewRequest(http.MethodGet, "/v1/admin/feature-flags", nil)
	listRec := httptest.NewRecorder()
	h.ListAdmin(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var listBody struct {
		Data []struct {
			Key     string `json:"key"`
			Label   string `json:"label"`
			Enabled bool   `json:"enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if len(listBody.Data) != 2 || listBody.Data[0].Key != featureflags.XDMSV1 || listBody.Data[0].Label != "X DMs" {
		t.Fatalf("unexpected list response: %s", listRec.Body.String())
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/v1/admin/feature-flags/x_dms_v1", strings.NewReader(`{"enabled":true}`))
	updateReq = updateReq.WithContext(context.WithValue(updateReq.Context(), auth.UserIDKey, "admin_1"))
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("key", featureflags.XDMSV1)
	updateReq = updateReq.WithContext(context.WithValue(updateReq.Context(), chi.RouteCtxKey, routeContext))
	updateRec := httptest.NewRecorder()
	h.UpdateAdmin(updateRec, updateReq)
	if updateRec.Code != http.StatusOK || !store.flags[featureflags.XDMSV1] {
		t.Fatalf("update status = %d, body = %s", updateRec.Code, updateRec.Body.String())
	}
}

func TestFeatureFlagsHandlerRejectsUnknownAndInvalidUpdates(t *testing.T) {
	store := &handlerFeatureFlagStore{flags: map[string]bool{}}
	h := NewFeatureFlagsHandler(store, featureflags.NewEvaluator(store, nil))

	for _, tt := range []struct {
		name string
		key  string
		body string
	}{
		{name: "unknown", key: "unknown", body: `{"enabled":true}`},
		{name: "missing boolean", key: featureflags.XDMSV1, body: `{}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/v1/admin/feature-flags/"+tt.key, strings.NewReader(tt.body))
			req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "admin_1"))
			routeContext := chi.NewRouteContext()
			routeContext.URLParams.Add("key", tt.key)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
			rec := httptest.NewRecorder()
			h.UpdateAdmin(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestFeatureFlagsHandlerPublicReturnsOnlyGlobalValues(t *testing.T) {
	store := &handlerFeatureFlagStore{flags: map[string]bool{
		featureflags.XDMSV1:            false,
		featureflags.XCreditsBillingV1: true,
	}}
	h := NewFeatureFlagsHandler(store, featureflags.NewEvaluator(store, nil))
	req := httptest.NewRequest(http.MethodGet, "/v1/public/features", nil)
	rec := httptest.NewRecorder()
	h.Public(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Flags map[string]bool `json:"flags"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Flags) != 2 || body.Data.Flags[featureflags.XDMSV1] || !body.Data.Flags[featureflags.XCreditsBillingV1] {
		t.Fatalf("unexpected public response: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "updated_by") {
		t.Fatalf("public response leaks admin metadata: %s", rec.Body.String())
	}
}

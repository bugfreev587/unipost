package worker

import (
	"context"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestInboxWorkerManagedSyncNotificationCounts(t *testing.T) {
	counts := newWorkerInboxSyncNotificationCounts()
	counts.Record("workspace-1", pgtype.Text{String: "managed-a", Valid: true})
	counts.Record("workspace-1", pgtype.Text{String: "managed-a", Valid: true})
	counts.Record("workspace-1", pgtype.Text{String: "managed-b", Valid: true})
	counts.Record("workspace-1", pgtype.Text{})
	counts.Record("workspace-2", pgtype.Text{String: "managed-c", Valid: true})

	managed := map[string]int{}
	aggregate := map[string]int{}
	counts.Notify(
		context.Background(),
		func(_ context.Context, workspaceID, externalUserID string, event map[string]any) {
			if event["type"] != "inbox.sync_complete" {
				t.Fatalf("managed event type = %#v", event)
			}
			managed[workspaceID+":"+externalUserID] = event["new_items"].(int)
		},
		func(_ context.Context, workspaceID string, event map[string]any) {
			if event["type"] != "inbox.sync_complete" {
				t.Fatalf("aggregate event type = %#v", event)
			}
			aggregate[workspaceID] = event["new_items"].(int)
		},
	)

	if want := map[string]int{"workspace-1:managed-a": 2, "workspace-1:managed-b": 1, "workspace-2:managed-c": 1}; !reflect.DeepEqual(managed, want) {
		t.Fatalf("managed counts = %#v, want %#v", managed, want)
	}
	if want := map[string]int{"workspace-1": 4, "workspace-2": 1}; !reflect.DeepEqual(aggregate, want) {
		t.Fatalf("aggregate counts = %#v, want %#v", aggregate, want)
	}
}

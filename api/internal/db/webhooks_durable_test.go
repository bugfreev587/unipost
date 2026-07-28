package db

import (
	"context"
	"strings"
	"testing"
)

func TestCreateWebhookDeliveryWithEventIDReservesMappingCreatesDeliveryAndBackfills(t *testing.T) {
	normalized := strings.Join(strings.Fields(strings.ToLower(createWebhookDeliveryWithEventID)), " ")
	for _, want := range []string{
		"cast($1 as text) as webhook_id",
		"cast($2 as text) as event",
		"cast($3 as jsonb) as payload",
		"cast($4 as text) as event_id",
		"insert into terminal_post_event_webhook_deliveries (event_id, webhook_id)",
		"on conflict (event_id, webhook_id) do nothing",
		"returning event_id, webhook_id",
		"insert into webhook_deliveries (webhook_id, event, payload)",
		"select reserved.webhook_id, input.event, input.payload",
		"update terminal_post_event_webhook_deliveries mapping set webhook_delivery_id = created.id",
		"where mapping.event_id = (select input.event_id from input) and mapping.webhook_id = created.webhook_id",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("CreateWebhookDeliveryWithEventID query missing %q:\n%s", want, createWebhookDeliveryWithEventID)
		}
	}

	reserve := strings.Index(normalized, "insert into terminal_post_event_webhook_deliveries")
	create := strings.Index(normalized, "insert into webhook_deliveries")
	backfill := strings.Index(normalized, "update terminal_post_event_webhook_deliveries")
	if reserve < 0 || create <= reserve || backfill <= create {
		t.Fatalf("durable webhook query must reserve mapping, create delivery, then backfill mapping:\n%s", createWebhookDeliveryWithEventID)
	}
	if strings.Contains(normalized, "insert into webhook_deliveries (webhook_id, event, payload, event_id)") {
		t.Fatal("durable webhook query must not require an event_id column on the legacy webhook_deliveries table")
	}
}

func TestCreateWebhookDeliveryWithEventIDGeneratedMethodIsAvailable(t *testing.T) {
	var generatedMethod func(*Queries, context.Context, CreateWebhookDeliveryWithEventIDParams) error = (*Queries).CreateWebhookDeliveryWithEventID
	if generatedMethod == nil {
		t.Fatal("generated durable webhook insert method is nil")
	}
}

func TestCreateWebhookDeliveryKeepsLegacyTableShape(t *testing.T) {
	normalized := strings.Join(strings.Fields(strings.ToLower(createWebhookDelivery)), " ")
	if !strings.Contains(normalized, "insert into webhook_deliveries (webhook_id, event, payload) values ($1, $2, $3)") {
		t.Fatalf("legacy webhook delivery insert changed unexpectedly:\n%s", createWebhookDelivery)
	}
	if strings.Contains(normalized, "event_id") {
		t.Fatalf("legacy webhook delivery query must not depend on durable mapping schema:\n%s", createWebhookDelivery)
	}
}

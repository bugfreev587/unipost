import type { Metadata } from "next";
import { filterDocsNavigation } from "@/lib/docs-feature-flags";
import { getPublicDocsFeatureFlags } from "@/lib/public-feature-flags-server";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";
import type { ApiFieldItem } from "../../_components/doc-components";

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs/api/inbox/sync" },
};

const AUTH: ApiFieldItem[] = [{ name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key. Inbox requires the Basic plan or higher." }];
const BODY: ApiFieldItem[] = [
  { name: "x_backfill", type: "object", description: "Include this object to run a bounded X reply/DM backfill. Omit it for the existing non-X Inbox sync." },
  { name: "x_backfill.account_id", type: "string", optional: true, description: "Limit the operation to one eligible connected X account." },
  { name: "x_backfill.lookback_days", type: "integer", optional: true, defaultValue: 7, description: "1-30 days. X reply lookup is bounded to 7 days; DM lookup can use up to 30 days." },
  { name: "x_backfill.max_items", type: "integer", optional: true, defaultValue: 20, description: "1-500 resources per source and account." },
  { name: "x_backfill.include_replies", type: "boolean", optional: true, description: "Fetch eligible mentions and replies." },
  { name: "x_backfill.include_dms", type: "boolean", optional: true, description: "Fetch legacy direct-message events only when x_dms_v1 is enabled for the workspace." },
  { name: "x_backfill.confirmation_token", type: "string", optional: true, description: "When the estimate exceeds the safe threshold, repeat the exact request with the returned one-time token before it expires." },
];
const RESPONSE: ApiFieldItem[] = [
  { name: "data.estimated_x_credits", type: "integer", description: "Upper-bound managed-X estimate. workspace_x_app accounts contribute zero UniPost X Credits." },
  { name: "data.confirmation_required", type: "boolean", description: "true when a second confirmed call is required before paid reads begin." },
  { name: "data.confirmation_token", type: "string", optional: true, description: "Short-lived token bound to the exact account set, request, and estimate." },
  { name: "data.accepted", type: "integer", description: "New items admitted to Inbox." },
  { name: "data.suppressed", type: "integer", description: "Items stopped by the managed-X monthly allowance or inbound daily cap." },
  { name: "data.details[].stop_reason", type: "string", optional: true, description: "Includes x_monthly_usage_limit_exceeded, x_inbound_daily_cap_exceeded, or reconnect_required." },
  { name: "data.details[].missing_scopes", type: "string[]", optional: true, description: "X permissions that require reconnect." },
];
const ERRORS: ApiFieldItem[] = [
  { name: "feature_not_available", type: "403", description: "A DM-only backfill was requested while X DMs are unavailable to the workspace." },
  { name: "plan_feature_not_available", type: "402", description: "Inbox is unavailable below Basic." },
  { name: "validation_error", type: "400/409", description: "The confirmation token is invalid, expired, already consumed, or no longer matches the frozen request." },
  { name: "not_found", type: "404", description: "No eligible X account matched the request." },
];

export default async function InboxSyncPage() {
  const publicFeatureFlags = await getPublicDocsFeatureFlags();
  const xDMsEnabled = publicFeatureFlags.x_dms_v1;
  const xCreditsEnabled = publicFeatureFlags.x_credits_billing_v1;
  const body = BODY
    .filter((field) => xDMsEnabled || field.name !== "x_backfill.include_dms")
    .map((field) => {
      if (field.name === "x_backfill" && !xDMsEnabled) {
        return { ...field, description: "Include this object to run a bounded X public-reply backfill. Omit it for the existing non-X Inbox sync." };
      }
      if (field.name === "x_backfill.lookback_days" && !xDMsEnabled) {
        return { ...field, description: "1-7 days for X public-reply lookup." };
      }
      return field;
    });
  const response = RESPONSE
    .filter((field) => xCreditsEnabled || field.name !== "data.estimated_x_credits")
    .map((field) => (
      field.name === "data.suppressed" && !xCreditsEnabled
        ? { ...field, description: "Items stopped by the inbound daily safety cap." }
        : field
    ));
  const errors = ERRORS.filter((field) => xDMsEnabled || field.name !== "feature_not_available");
  const syncBody = `{"x_backfill":{"account_id":"sa_x_01","lookback_days":7,"max_items":50,"include_replies":true${xDMsEnabled ? ',"include_dms":true' : ""}}}`;
  const responseBody = xCreditsEnabled
    ? `{
  "data": {
    "estimated_x_credits": 1750,
    "confirmation_required": true,
    "confirmation_operation_id": "xop_01",
    "confirmation_token": "opaque-one-time-token",
    "confirmation_expires_at": "2026-07-16T19:10:00Z",
    "accounts_checked": 1,
    "accepted": 0,
    "suppressed": 0
  }
}`
    : `{
  "data": {
    "confirmation_required": true,
    "confirmation_operation_id": "xop_01",
    "confirmation_token": "opaque-one-time-token",
    "confirmation_expires_at": "2026-07-16T19:10:00Z",
    "accounts_checked": 1,
    "accepted": 0,
    "suppressed": 0
  }
}`;
  const guideLinks = filterDocsNavigation([
    { label: "Sync X comments", href: "/docs/guides/x/comments" },
    { label: "Sync X direct messages", href: "/docs/guides/x/direct-messages" },
    { label: "Reconnect X permissions", href: "/docs/guides/x/reconnect-permissions" },
  ], publicFeatureFlags);

  return (
    <SingleEndpointReferencePage
      section="inbox"
      title="Sync and backfill Inbox"
      description={xDMsEnabled
        ? "Runs existing Inbox polling or a bounded X public-reply or direct-message backfill. The inbound safety cap remains active."
        : "Runs existing Inbox polling or a bounded X public-reply backfill. The inbound safety cap remains active."}
      method="POST"
      path="/v1/inbox/sync"
      requestSections={[{ title: "Authorization", items: AUTH }, { title: "Request Body", items: body }]}
      responses={[{ code: "200", fields: response }, { code: "400", fields: errors }, { code: "402", fields: errors }, { code: "409", fields: errors }]}
      snippets={[{ lang: "curl", label: "cURL", code: `curl -X POST "https://api.unipost.dev/v1/inbox/sync" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '${syncBody}'` }]}
      responseSnippets={[{ lang: "json", label: "200", code: responseBody }, { lang: "json", label: "400", code: `{ "error": { "code": "VALIDATION_ERROR", "message": "invalid X backfill confirmation token" } }` }, { lang: "json", label: "402", code: `{ "error": { "code": "PLAN_FEATURE_NOT_AVAILABLE", "message": "Inbox requires the Basic plan or higher." } }` }, { lang: "json", label: "409", code: `{ "error": { "code": "VALIDATION_ERROR", "message": "X backfill account selection or request changed after confirmation" } }` }]}
      guideLinks={guideLinks}
    />
  );
}

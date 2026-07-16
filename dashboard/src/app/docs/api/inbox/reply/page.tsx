"use client";

import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";
import type { ApiFieldItem } from "../../_components/doc-components";

const AUTH: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key. Inbox requires the Basic plan or higher." },
  { name: "Idempotency-Key", type: "string", meta: "In header", description: "Required for x_reply and x_dm. Reuse the same key only for the exact same item and text." },
];
const PATH: ApiFieldItem[] = [{ name: "id", type: "string", description: "Inbound Inbox item ID." }];
const BODY: ApiFieldItem[] = [{ name: "text", type: "string", description: "Non-empty public reply or private-message text." }];
const RESPONSE: ApiFieldItem[] = [
  { name: "data.id", type: "string", description: "Persisted outbound Inbox item ID." },
  { name: "data.source", type: "string", description: "x_reply or x_dm for an X response." },
  { name: "data.is_own", type: "boolean", description: "true for the sent item." },
  { name: "data.x_credits_counted", type: "integer", description: "Weighted units charged. It is zero for workspace_x_app." },
  { name: "data.x_credit_operation", type: "string", description: "post.reply_summoned for eligible replies or dm.send for direct messages." },
  { name: "data.x_credit_billing_mode", type: "string", description: "unipost_managed_app or workspace_x_app." },
];
const ERRORS: ApiFieldItem[] = [
  { name: "x_monthly_usage_limit_exceeded", type: "402", description: "Managed-X allowance exhausted. Do not retry until capacity resets or changes." },
  { name: "x_reconnect_required", type: "409", description: "Reconnect the account with the required X scopes." },
  { name: "idempotency_key_conflict", type: "409", description: "The key was previously used with different text or a different item." },
  { name: "x_write_outcome_pending", type: "409", description: "X may have accepted the write. Reuse the key to inspect state; UniPost does not resend it." },
  { name: "x_write_needs_reconciliation", type: "409", description: "The outcome needs manual reconciliation; do not create another key to resend." },
  { name: "x_remote_accepted_reconciling", type: "202", description: "X accepted the write and UniPost is finishing local persistence." },
];

export default function InboxReplyPage() {
  return (
    <SingleEndpointReferencePage
      section="inbox"
      title="Reply to an Inbox item"
      description="Sends an eligible public reply or legacy direct message. UniPost-managed X operations consume X Credits; workspace-owned X app operations use that app's X access and do not consume UniPost X Credits."
      method="POST"
      path="/v1/inbox/:id/reply"
      requestSections={[{ title: "Authorization", items: AUTH }, { title: "Path Params", items: PATH }, { title: "Request Body", items: BODY }]}
      responses={[{ code: "200", fields: RESPONSE }, { code: "202", fields: ERRORS }, { code: "402", fields: ERRORS }, { code: "409", fields: ERRORS }]}
      snippets={[{ lang: "curl", label: "cURL", code: `curl -X POST "https://api.unipost.dev/v1/inbox/inbox_x_01/reply" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: reply-inbox-x-01-v1" \\
  -H "Content-Type: application/json" \\
  -d '{"text":"Release notes are available at docs.example.com/releases."}'` }]}
      responseSnippets={[{ lang: "json", label: "200", code: `{
  "data": {
    "id": "inbox_x_02",
    "source": "x_reply",
    "body": "Release notes are available at docs.example.com/releases.",
    "is_own": true,
    "x_credits_counted": 10,
    "x_credit_operation": "post.reply_summoned",
    "x_credit_billing_mode": "unipost_managed_app"
  }
}` }, { lang: "json", label: "202", code: `{ "error": { "code": "X_REMOTE_ACCEPTED_RECONCILING", "message": "X accepted the reply; UniPost is reconciling the local Inbox result" } }` }, { lang: "json", label: "402", code: `{ "error": { "code": "X_MONTHLY_USAGE_LIMIT_EXCEEDED", "normalized_code": "x_monthly_usage_limit_exceeded" } }` }, { lang: "json", label: "409", code: `{ "error": { "code": "X_RECONNECT_REQUIRED", "message": "Reconnect the X account to grant missing scopes" } }` }]}
      guideLinks={[
        { label: "Reply to X comments", href: "/docs/guides/x/comments" },
        { label: "Reply to X direct messages", href: "/docs/guides/x/direct-messages" },
      ]}
    />
  );
}

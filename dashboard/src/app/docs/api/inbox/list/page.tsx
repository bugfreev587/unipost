import type { Metadata } from "next";
import { filterDocsNavigation } from "@/lib/docs-feature-flags";
import { getPublicDocsFeatureFlags } from "@/lib/public-feature-flags-server";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";
import type { ApiFieldItem } from "../../_components/doc-components";

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs/api/inbox/list" },
};

const AUTH: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key kept server-side. Inbox requires the Basic plan or higher." },
];
const QUERY: ApiFieldItem[] = [
  { name: "inbox_scope", type: "string", description: "Required for API-key requests. Use managed_user for one app user or workspace with a creator-bound owner/admin key." },
  { name: "external_user_id", type: "string", optional: true, description: "Required only for managed_user. Derive it from the authenticated app user on your server." },
  { name: "source", type: "string", optional: true, description: "Filter by ig_comment, ig_dm, threads_reply, fb_comment, fb_dm, x_reply, or x_dm." },
  { name: "is_read", type: "boolean", optional: true, description: "Return only read or unread items." },
  { name: "is_own", type: "boolean", optional: true, description: "Return outbound items when true or inbound items when false." },
  { name: "limit", type: "integer", optional: true, defaultValue: 50, description: "Number of items, from 1 through 500." },
];
const RESPONSE: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Items ordered by received_at descending." },
  { name: "data[].id", type: "string", description: "UniPost Inbox item ID used by the reply endpoint." },
  { name: "data[].source", type: "string", description: "Normalized Inbox source." },
  { name: "data[].thread_key", type: "string", description: "Stable conversation or reply-tree grouping key." },
  { name: "data[].parent_external_id", type: "string", optional: true, description: "Parent post, reply, conversation, or participant identifier." },
  { name: "data[].body", type: "string", optional: true, description: "Message or reply text. Keep private DM content out of logs and analytics." },
  { name: "data[].url", type: "string", optional: true, description: "X permalink when the source exposes one." },
  { name: "data[].x_credits_counted", type: "integer", optional: true, description: "Managed-X weighted units attached to this item; workspace_x_app activity reports no UniPost charge." },
  { name: "data[].x_credit_billing_mode", type: "string", optional: true, description: "unipost_managed_app or workspace_x_app." },
];
const ERRORS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "INBOX_SCOPE_REQUIRED, INBOX_SCOPE_INVALID, MANAGED_USER_NOT_FOUND, INSUFFICIENT_ROLE, API_KEY_CREATOR_REQUIRED, UNAUTHORIZED, FEATURE_NOT_AVAILABLE, or PLAN_FEATURE_NOT_AVAILABLE." },
  { name: "request_id", type: "string", description: "Request identifier for support." },
];

export default async function InboxListPage() {
  const publicFeatureFlags = await getPublicDocsFeatureFlags();
  const xDMsEnabled = publicFeatureFlags.x_dms_v1;
  const xCreditsEnabled = publicFeatureFlags.x_credits_billing_v1;
  const query = QUERY.map((field) => (
    field.name === "source"
      ? {
          ...field,
          description: `Filter by ig_comment, ig_dm, threads_reply, fb_comment, fb_dm, x_reply${xDMsEnabled ? ", or x_dm" : ""}.`,
        }
      : field
  ));
  const response = RESPONSE
    .filter((field) => xCreditsEnabled || !field.name.startsWith("data[].x_credit"))
    .map((field) => (
      field.name === "data[].body" && !xDMsEnabled
        ? { ...field, description: "Comment or reply text." }
        : field
    ));
  const errors = ERRORS.filter((field) => xDMsEnabled || field.name !== "error.code");
  const guideLinks = filterDocsNavigation([
    { label: "Work with X comments", href: "/docs/guides/x/comments" },
    { label: "Work with X direct messages", href: "/docs/guides/x/direct-messages" },
  ], publicFeatureFlags);

  return (
    <SingleEndpointReferencePage
      section="inbox"
      title="List Inbox items"
      description={xDMsEnabled
        ? "Returns the normalized Inbox contract for Instagram, Facebook, Threads, and X. X replies use source x_reply; legacy X direct messages use x_dm."
        : "Returns the normalized Inbox contract for Instagram, Facebook, Threads, and X. X public replies use source x_reply."}
      method="GET"
      path="/v1/inbox"
      requestSections={[{ title: "Authorization", items: AUTH }, { title: "Query Params", items: query }]}
      responses={[{ code: "200", fields: response }, { code: "400", fields: errors }, { code: "403", fields: errors }, { code: "404", fields: errors }, { code: "402", fields: errors }]}
      snippets={[{ lang: "curl", label: "cURL", code: `curl "https://api.unipost.dev/v1/inbox?inbox_scope=managed_user&external_user_id=user_123&source=x_reply&is_own=false&limit=50" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"` }]}
      responseSnippets={[{ lang: "json", label: "200", code: `{
  "data": [{
    "id": "inbox_x_01",
    "social_account_id": "sa_x_01",
    "source": "x_reply",
    "external_id": "190418337001",
    "thread_key": "x_reply:190418337001",
    "author_name": "Mara Liu",
    "body": "@brand Could you share the release notes?",
    "is_read": false,
    "is_own": false,
    "received_at": "2026-07-16T18:42:11Z",
    "url": "https://x.com/maraliu/status/190418337001"
  }]
}` }, { lang: "json", label: "402", code: `{
  "error": { "code": "PLAN_FEATURE_NOT_AVAILABLE", "message": "Inbox requires the Basic plan or higher." },
  "request_id": "req_01"
}` }]}
      guideLinks={guideLinks}
    />
  );
}

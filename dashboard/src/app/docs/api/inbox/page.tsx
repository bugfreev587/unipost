import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  EnumValues,
  type ApiFieldItem,
} from "../_components/doc-components";
import { getPublicDocsFeatureFlags } from "@/lib/public-feature-flags-server";

const AVAILABILITY_FIELDS: ApiFieldItem[] = [
  {
    name: "Status",
    type: "supported",
    description: "Inbox APIs are available for supported plans and are gated by the workspace plan.",
  },
  {
    name: "Current support",
    type: "6 available + 1 controlled source",
    description: "Instagram, Facebook, Threads, and X replies are available; legacy X DMs are controlled by workspace rollout eligibility.",
  },
  {
    name: "Auth model",
    type: "Bearer <token>",
    meta: "In header",
    description: "Use a workspace API key or an authenticated dashboard token with workspace access.",
  },
];

const ENDPOINT_FIELDS: ApiFieldItem[] = [
  {
    name: "GET /v1/inbox",
    type: "list",
    description: <>List inbox items for the current workspace. See the <a href="/docs/api/inbox/list">endpoint reference</a>.</>,
  },
  {
    name: "GET /v1/inbox/unread-count",
    type: "read",
    description: "Return the current unread item count for sidebar badges and notification surfaces.",
  },
  {
    name: "POST /v1/inbox/sync",
    type: "sync",
    description: <>Trigger polling or a bounded X backfill. See the <a href="/docs/api/inbox/sync">endpoint reference</a>.</>,
  },
  {
    name: "GET /v1/inbox/{id}",
    type: "read",
    description: "Fetch a single normalized inbox item.",
  },
  {
    name: "POST /v1/inbox/{id}/reply",
    type: "write",
    description: <>Reply to a supported comment, DM, or thread. See the <a href="/docs/api/inbox/reply">endpoint reference</a>.</>,
  },
  {
    name: "POST /v1/inbox/{id}/thread-state",
    type: "workflow",
    description: "Update workflow state such as open, assigned, or resolved.",
  },
];

const SOURCE_FIELDS: ApiFieldItem[] = [
  {
    name: "ig_comment",
    type: "supported",
    description: "Comments on connected Instagram Business or Creator accounts.",
  },
  {
    name: "ig_dm",
    type: "supported",
    description: "Instagram DMs when the connected account and permissions are eligible.",
  },
  {
    name: "threads_reply",
    type: "supported",
    description: "Comments and replies on connected Threads content.",
  },
  {
    name: "fb_comment",
    type: "supported",
    description: "Comments on connected Facebook Page posts.",
  },
  {
    name: "fb_dm",
    type: "supported",
    description: "Private messages for connected Facebook Pages.",
  },
  {
    name: "x_reply",
    type: "supported",
    description: "Eligible public replies that summon a connected X account.",
  },
  {
    name: "x_dm",
    type: "controlled availability",
    description: "Legacy X direct-message lookup/send when x_dms_v1 is enabled for the workspace. Private real-time subscription provisioning is not currently available.",
  },
  {
    name: "more_sources",
    type: "coming soon",
    description: "Additional inbox sources and workflow coverage will be added over time.",
  },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  {
    name: "source?",
    type: "string",
    description: <>Filter by normalized source.<EnumValues values={["ig_comment", "ig_dm", "threads_reply", "fb_comment", "fb_dm", "x_reply", "x_dm"]} /></>,
  },
  {
    name: "is_read?",
    type: "boolean",
    description: "Filter unread or read items.",
  },
  {
    name: "is_own?",
    type: "boolean",
    description: "Filter items authored by the connected account or by an external user.",
  },
  {
    name: "limit?",
    type: "integer",
    description: "Maximum number of items to return. Defaults to 50 and caps at 500.",
  },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Normalized inbox items sorted by received_at descending." },
  { name: "data[].id", type: "string", description: "Stable UniPost inbox item identifier." },
  { name: "data[].source", type: "string", description: "One of ig_comment, ig_dm, threads_reply, fb_comment, fb_dm, x_reply, or x_dm." },
  { name: "data[].social_account_id", type: "string", description: "Connected account that owns the item." },
  { name: "data[].thread_key", type: "string", description: "Stable key used to group related messages and replies." },
  { name: "data[].thread_status", type: "string", description: <>Workflow state.<EnumValues values={["open", "assigned", "resolved"]} /></> },
  { name: "data[].author_name", type: "string", description: "Best available author display name." },
  { name: "data[].body", type: "string", description: "Comment or message body." },
  { name: "data[].is_read", type: "boolean", description: "Whether the item has been read in UniPost." },
  { name: "data[].is_own", type: "boolean", description: "Whether the item was authored by the connected account." },
  { name: "data[].received_at", type: "string", description: "Inbound or outbound activity timestamp." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "UNAUTHORIZED, FEATURE_NOT_AVAILABLE, PLAN_FEATURE_NOT_AVAILABLE, or VALIDATION_ERROR." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const REQUEST_SNIPPETS = [
  {
    lang: "curl",
    label: "List",
    code: `curl "https://api.unipost.dev/v1/inbox?source=ig_comment&is_read=false&is_own=false&limit=100" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "curl",
    label: "Reply",
    code: `curl -X POST "https://api.unipost.dev/v1/inbox/inbox_item_123/reply" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"text":"Thanks for reaching out. We will take a look."}'`,
  },
  {
    lang: "curl",
    label: "Sync",
    code: `curl -X POST "https://api.unipost.dev/v1/inbox/sync" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "id": "inbox_item_123",
      "social_account_id": "sa_instagram_123",
      "workspace_id": "ws_123",
      "source": "ig_comment",
      "external_id": "17895695668004550",
      "thread_key": "ig_comment:17895695668004550",
      "thread_status": "open",
      "author_name": "Nora Valdez",
      "body": "Can you share the launch date?",
      "is_read": false,
      "is_own": false,
      "received_at": "2026-04-22T18:40:00Z",
      "created_at": "2026-04-22T18:40:03Z"
    }
  ],
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "402",
    code: `{
  "error": {
    "code": "PLAN_FEATURE_NOT_AVAILABLE",
    "message": "Inbox requires the Basic plan or higher."
  },
  "request_id": "req_123"
}`,
  },
];

export default async function InboxPage() {
  const publicFeatureFlags = await getPublicDocsFeatureFlags();
  const xDMsEnabled = publicFeatureFlags.x_dms_v1;
  const availabilityFields = AVAILABILITY_FIELDS.map((field) => (
    field.name === "Current support"
      ? {
          ...field,
          type: xDMsEnabled ? "7 available sources" : "6 available sources",
          description: xDMsEnabled
            ? "Instagram, Facebook, Threads, X replies, and legacy X DMs are available."
            : "Instagram, Facebook, Threads, and X replies are available.",
        }
      : field
  ));
  const endpointFields = ENDPOINT_FIELDS.map((field) => (
    field.name === "POST /v1/inbox/{id}/reply" && !xDMsEnabled
      ? { ...field, description: <>Reply to a supported comment or thread. See the <a href="/docs/api/inbox/reply">endpoint reference</a>.</> }
      : field
  ));
  const sourceFields = SOURCE_FIELDS.filter((field) => xDMsEnabled || field.name !== "x_dm");
  const sourceValues = ["ig_comment", "ig_dm", "threads_reply", "fb_comment", "fb_dm", "x_reply"];
  if (xDMsEnabled) sourceValues.push("x_dm");
  const queryFields = QUERY_FIELDS.map((field) => (
    field.name === "source?"
      ? { ...field, description: <>Filter by normalized source.<EnumValues values={sourceValues} /></> }
      : field
  ));
  const responseFields = RESPONSE_FIELDS.map((field) => (
    field.name === "data[].source"
      ? {
          ...field,
          description: `One of ${sourceValues.join(", ")}.`,
        }
      : field
  ));
  const errorFields = ERROR_FIELDS.map((field) => (
    field.name === "error.code" && !xDMsEnabled
      ? { ...field, description: "UNAUTHORIZED, PLAN_FEATURE_NOT_AVAILABLE, or VALIDATION_ERROR." }
      : field
  ));

  return (
    <ApiReferencePage
      section="inbox"
      title="Inbox"
      description="Use UniPost Inbox APIs to list, sync, and reply to normalized Instagram, Facebook, Threads, and X conversations. X Inbox requires the Basic plan or higher."
    >
      <ApiReferenceGrid
        left={
          <div className="api-reference-left-flow" style={{ display: "grid", gap: 16 }}>
            <div className="api-field-sections">
              <section className="api-field-section" style={{ paddingTop: 0 }}>
                <h2 className="api-field-section-title">Availability</h2>
                <ApiFieldList items={availabilityFields} />
              </section>

              <section className="api-field-section">
                <h2 className="api-field-section-title">Supported endpoints</h2>
                <ApiFieldList items={endpointFields} />
              </section>

              <section className="api-field-section">
                <h2 className="api-field-section-title">Supported sources</h2>
                <ApiFieldList items={sourceFields} />
              </section>

              <section className="api-field-section">
                <h2 className="api-field-section-title">Query params</h2>
                <ApiFieldList items={queryFields} />
              </section>
            </div>

            <section className="api-field-section api-response-field-section">
              <h2 className="api-field-section-title">Response</h2>
              <ApiAccordion title="200 OK" defaultOpen>
                <ApiFieldList items={responseFields} />
              </ApiAccordion>
              <ApiAccordion title="Errors">
                <ApiFieldList items={errorFields} />
              </ApiAccordion>
            </section>
          </div>
        }
        right={
          <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
            <CodeTabs snippets={REQUEST_SNIPPETS} />
            <CodeTabs snippets={RESPONSE_SNIPPETS} />
          </div>
        }
      />
    </ApiReferencePage>
  );
}

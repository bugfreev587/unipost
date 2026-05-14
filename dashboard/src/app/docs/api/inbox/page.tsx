"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  type ApiFieldItem,
} from "../_components/doc-components";

const AVAILABILITY_FIELDS: ApiFieldItem[] = [
  {
    name: "Status",
    type: "planned",
    description: "Public Inbox REST APIs are reserved but not documented for general availability yet.",
  },
  {
    name: "Current surface",
    type: "dashboard",
    description: "Inbox workflows are available in the hosted UniPost dashboard for supported plans.",
  },
  {
    name: "Auth model",
    type: "Bearer <token>",
    meta: "planned",
    description: "The public API surface will use the same workspace API key model as the rest of the UniPost API.",
  },
];

const RESOURCE_FIELDS: ApiFieldItem[] = [
  {
    name: "threads",
    type: "planned",
    description: "List and inspect inbox threads across connected destinations.",
  },
  {
    name: "messages",
    type: "planned",
    description: "Read normalized message history from one API surface.",
  },
  {
    name: "replies",
    type: "planned",
    description: "Send responses from your own product instead of only the hosted dashboard.",
  },
  {
    name: "moderation",
    type: "planned",
    description: "Support review, escalation, and response workflows.",
  },
];

const SOURCE_FIELDS: ApiFieldItem[] = [
  {
    name: "instagram_comments",
    type: "supported in dashboard",
    description: "Comments for connected Instagram Business or Creator accounts.",
  },
  {
    name: "instagram_dm",
    type: "supported in dashboard",
    description: "Instagram DMs when the account and permissions are eligible.",
  },
  {
    name: "facebook_comments",
    type: "supported in dashboard",
    description: "Comments on connected Facebook Page posts.",
  },
  {
    name: "threads_reply",
    type: "supported in dashboard",
    description: "Replies on connected Threads content.",
  },
];

const PREVIEW_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Inbox thread records once the public API is available." },
  { name: "data[].id", type: "string", description: "Stable thread identifier." },
  { name: "data[].source", type: "string", description: "Normalized inbox source such as instagram_dm or threads_reply." },
  { name: "data[].account_id", type: "string", description: "Connected account that owns the thread." },
  { name: "data[].state", type: "string", description: "Thread state such as open, archived, or resolved." },
  { name: "data[].last_message_at", type: "string", description: "Most recent inbound or outbound activity timestamp." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const STATUS_SNIPPETS = [
  {
    lang: "json",
    label: "Status",
    code: `{
  "surface": "inbox",
  "public_api": "planned",
  "current_access": "dashboard",
  "reserved_resources": [
    "threads",
    "messages",
    "replies",
    "moderation"
  ]
}`,
  },
];

const PREVIEW_SNIPPETS = [
  {
    lang: "json",
    label: "Preview",
    code: `{
  "data": [
    {
      "id": "inbox_thread_123",
      "source": "instagram_dm",
      "account_id": "sa_instagram_123",
      "state": "open",
      "last_message_at": "2026-04-22T18:40:00Z"
    }
  ],
  "request_id": "req_123"
}`,
  },
];

export default function InboxPage() {
  return (
    <ApiReferencePage
      section="inbox"
      title="Inbox"
      description="UniPost Inbox is the next major public API surface for bringing conversations, moderation, and response workflows into customer-facing products instead of limiting them to the hosted dashboard."
    >
      <ApiReferenceGrid
        left={
          <div className="api-reference-left-flow" style={{ display: "grid", gap: 16 }}>
            <div className="api-field-sections">
              <section className="api-field-section" style={{ paddingTop: 0 }}>
                <h2 className="api-field-section-title">Availability</h2>
                <ApiFieldList items={AVAILABILITY_FIELDS} />
              </section>

              <section className="api-field-section">
                <h2 className="api-field-section-title">Planned resources</h2>
                <ApiFieldList items={RESOURCE_FIELDS} />
              </section>

              <section className="api-field-section">
                <h2 className="api-field-section-title">Supported sources</h2>
                <ApiFieldList items={SOURCE_FIELDS} />
              </section>
            </div>

            <section className="api-field-section api-response-field-section">
              <h2 className="api-field-section-title">Response Preview</h2>
              <ApiAccordion title="Preview" defaultOpen>
                <ApiFieldList items={PREVIEW_FIELDS} />
              </ApiAccordion>
            </section>
          </div>
        }
        right={
          <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
            <CodeTabs snippets={STATUS_SNIPPETS} />
            <CodeTabs snippets={PREVIEW_SNIPPETS} />
          </div>
        }
      />
    </ApiReferencePage>
  );
}

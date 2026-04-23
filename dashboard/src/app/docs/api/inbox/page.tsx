"use client";

import {
  ApiReferencePage,
  ApiEndpointCard,
  ApiFieldList,
  type ApiFieldItem,
} from "../_components/doc-components";

const PLANNED_FIELDS: ApiFieldItem[] = [
  {
    name: "Conversations",
    type: "planned",
    description: "List and inspect inbox threads across connected destinations.",
  },
  {
    name: "Messages",
    type: "planned",
    description: "Read normalized message history from one API surface.",
  },
  {
    name: "Replies",
    type: "planned",
    description: "Send responses from your own product instead of only the hosted dashboard.",
  },
  {
    name: "Moderation",
    type: "planned",
    description: "Support review, escalation, and response workflows.",
  },
];

export default function InboxPage() {
  return (
    <ApiReferencePage
      section="inbox"
      title="Inbox"
      description="UniPost Inbox is the next major public API surface for bringing conversations, moderation, and response workflows into customer-facing products instead of limiting them to the hosted dashboard."
    >
      <div style={{ display: "grid", gap: 18 }}>
        <ApiEndpointCard method="GET" path="inbox">
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Status</div>
            <div style={{ fontSize: 15, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
              Public Inbox APIs are not documented yet, but this section is reserved for the unified resource model and endpoint surface.
            </div>
          </div>
        </ApiEndpointCard>

        <ApiEndpointCard method="GET" path="inbox">
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Planned scope</div>
            <ApiFieldList items={PLANNED_FIELDS} />
          </div>
        </ApiEndpointCard>
      </div>
    </ApiReferencePage>
  );
}

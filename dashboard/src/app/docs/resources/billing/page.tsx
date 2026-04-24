"use client";

import {
  ApiReferencePage,
  ApiEndpointCard,
  ApiFieldList,
  type ApiFieldItem,
} from "../../api/_components/doc-components";

const CONCEPT_FIELDS: ApiFieldItem[] = [
  {
    name: "Plan",
    type: "workspace",
    description: "Understand which features or limits apply to the current workspace.",
  },
  {
    name: "Usage",
    type: "metering",
    description: "Track how much of the current billing period has been consumed.",
  },
  {
    name: "Warnings",
    type: "limits",
    description: "Surface approaching-limit or over-limit states in your own UI.",
  },
];

const HEADER_FIELDS: ApiFieldItem[] = [
  {
    name: "X-UniPost-Usage",
    type: "response header",
    description: "Current workspace usage information on publish-related responses.",
  },
  {
    name: "X-UniPost-Warning",
    type: "response header",
    description: "Near-limit or policy warnings your app can surface directly.",
  },
];

export default function BillingPage() {
  return (
    <ApiReferencePage
      section="insights"
      title="Billing and usage"
      description="Billing and usage endpoints help your app understand plan state, publish usage, and workspace limits. This matters most when you are building a SaaS on top of UniPost."
    >
      <div style={{ display: "grid", gap: 18 }}>
        <ApiEndpointCard method="GET" path="billing">
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>What it covers</div>
            <ApiFieldList items={CONCEPT_FIELDS} />
          </div>
        </ApiEndpointCard>

        <ApiEndpointCard method="GET" path="billing">
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Where billing shows up</div>
            <ApiFieldList items={HEADER_FIELDS} />
          </div>
        </ApiEndpointCard>
      </div>
    </ApiReferencePage>
  );
}

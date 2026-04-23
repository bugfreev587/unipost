import { DocsPage, DocsTable } from "../../_components/docs-shell";

export default function InboxPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Inbox"
      lead="UniPost Inbox is the next major public API surface for bringing conversations, moderation, and response workflows into customer-facing products instead of limiting them to the hosted dashboard."
    >
      <div className="docs-callout">
        <strong>Coming soon:</strong> public Inbox APIs are not documented yet, but this section is reserved for the unified resource model and endpoint surface.
      </div>

      <h2 id="planned-scope">Planned scope</h2>
      <DocsTable
        columns={["Area", "What will live here"]}
        rows={[
          ["Conversations", "List and inspect inbox threads across connected destinations"],
          ["Messages", "Read message history and normalize message payloads"],
          ["Replies", "Send responses from one unified API surface"],
          ["Moderation", "Support review and escalation workflows inside your own product"],
        ]}
      />
    </DocsPage>
  );
}

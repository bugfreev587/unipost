import type { Metadata } from "next";
import {
  ApiInlineLink,
  ApiReferencePage,
  ApiEndpointCard,
  Breadcrumbs,
  CodeTabs,
  InfoBox,
  RelatedEndpoints,
  ResponseBlock,
} from "../_components/doc-components";

export const metadata: Metadata = {
  title: "Webhooks — Event Reference | UniPost API Docs",
  description: "Developer webhook reference for UniPost post lifecycle and account events. Includes payload shapes, signature verification, retries, and endpoint links.",
  keywords: ["unipost webhooks", "social media api webhooks", "post published webhook", "webhook signature verification"],
};

const EVENTS = [
  ["post.published", "All platform results finished successfully."],
  ["post.partial", "At least one platform succeeded and at least one failed."],
  ["post.failed", "Every platform result ended in failure."],
  ["account.connected", "A user connected a new account through Connect or the dashboard."],
  ["account.disconnected", "A connected account was disconnected or permanently expired."],
];

const PUBLISHED_PAYLOAD = `{
  "event": "post.published",
  "timestamp": "2026-04-23T18:00:00Z",
  "data": {
    "id": "post_abc123",
    "status": "published",
    "execution_mode": "async",
    "created_at": "2026-04-23T17:59:48Z",
    "results": [
      {
        "id": "spr_123",
        "social_account_id": "sa_bluesky_123",
        "platform": "bluesky",
        "status": "published",
        "external_id": "at://did:plc:example/app.bsky.feed.post/abc",
        "published_at": "2026-04-23T18:00:00Z"
      }
    ]
  }
}`;

const VERIFY_SNIPPETS = [
  {
    lang: "js",
    label: "Node.js",
    code: `import { verifyWebhookSignature } from "@unipost/sdk";

app.post("/webhooks/unipost", async (req, res) => {
  const payload = JSON.stringify(req.body);
  const signature = req.headers["x-unipost-signature"];

  const isValid = await verifyWebhookSignature({
    payload,
    signature,
    secret: process.env.UNIPOST_WEBHOOK_SECRET,
  });

  if (!isValid) {
    return res.status(401).json({ error: "Invalid signature" });
  }

  const { event, data } = req.body;
  console.log(event, data.id);
  return res.status(200).json({ received: true });
});`,
  },
];

export default function WebhooksPage() {
  return (
    <ApiReferencePage
      section="developer-webhooks"
      title="Developer webhooks"
      description="Use developer webhooks when your own backend needs push delivery for async post results or account lifecycle events. This is the machine-facing surface for product integrations, not the dashboard notifications channel."
    >
      <Breadcrumbs items={[
        { label: "Docs", href: "/docs" },
        { label: "API Reference", href: "/docs/api" },
        { label: "Developer Webhooks" },
      ]} />

      <div style={{ display: "grid", gap: 18 }}>
        <ApiEndpointCard method="POST" path="/v1/webhooks">
          <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 12 }}>How it fits into async publishing</div>
            <p style={{ margin: 0, fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
              <ApiInlineLink endpoint="POST /v1/posts" /> accepts the request, creates result rows, and queues background delivery jobs. Final outcome is then available by polling <ApiInlineLink endpoint="GET /v1/posts/:post_id" /> or by subscribing to these webhooks.
            </p>
          </div>
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 12 }}>Live events</div>
            <div style={{ border: "1px solid var(--docs-border)", borderRadius: 10, overflow: "hidden", background: "var(--docs-bg-elevated)" }}>
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
                <thead>
                  <tr style={{ background: "var(--docs-bg-muted)" }}>
                    <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Event</th>
                    <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>When it fires</th>
                  </tr>
                </thead>
                <tbody>
                  {EVENTS.map(([event, description], index) => (
                    <tr key={event} style={{ borderBottom: index < EVENTS.length - 1 ? "1px solid var(--docs-border)" : undefined }}>
                      <td style={{ padding: "10px 14px", fontFamily: "var(--docs-mono)", color: "var(--docs-text)", fontWeight: 500 }}>{event}</td>
                      <td style={{ padding: "10px 14px", color: "var(--docs-text-soft)" }}>{description}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </ApiEndpointCard>

        <ApiEndpointCard method="POST" path="/v1/webhooks">
          <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 12 }}>Payload example</div>
            <ResponseBlock title="post.published" code={PUBLISHED_PAYLOAD} />
          </div>
          <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 12 }}>Signature verification</div>
            <p style={{ margin: "0 0 14px", fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
              Every request includes <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>X-UniPost-Signature</code> with the format <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>sha256=&lt;hex&gt;</code>. Verify it before processing the payload.
            </p>
            <CodeTabs snippets={VERIFY_SNIPPETS} />
          </div>
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 12 }}>Retry behavior</div>
            <InfoBox>
              UniPost currently retries failed webhook deliveries up to 3 attempts total. In the current backend, retries are scheduled at approximately 1 minute, 5 minutes, and 30 minutes, and the outbound HTTP timeout is 5 seconds.
            </InfoBox>
          </div>
        </ApiEndpointCard>

        <ApiEndpointCard method="POST" path="/v1/webhooks">
          <div style={{ padding: "18px" }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Endpoint map</div>
            <RelatedEndpoints items={[
              { method: "POST", path: "/v1/webhooks", label: "Create webhook", href: "/docs/api/webhooks/create" },
              { method: "GET", path: "/v1/webhooks", label: "List webhooks", href: "/docs/api/webhooks/list" },
              { method: "GET", path: "/v1/webhooks/:id", label: "Get webhook", href: "/docs/api/webhooks/get" },
              { method: "PATCH", path: "/v1/webhooks/:id", label: "Update webhook", href: "/docs/api/webhooks/update" },
              { method: "POST", path: "/v1/webhooks/:id/rotate", label: "Rotate secret", href: "/docs/api/webhooks/rotate" },
              { method: "GET", path: "/v1/posts/:post_id/queue", label: "Inspect queue state", href: "/docs/api/posts/get" },
            ]} />
          </div>
        </ApiEndpointCard>
      </div>
    </ApiReferencePage>
  );
}

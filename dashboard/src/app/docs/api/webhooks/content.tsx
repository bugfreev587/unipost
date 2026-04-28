"use client";

import {
  ApiInlineLink,
  Breadcrumbs, DocSection, CodeTabs, ResponseBlock, InfoBox, RelatedEndpoints,
} from "../_components/doc-components";

interface WebhookEvent {
  event: string;
  description: string;
  status: "live" | "coming_soon";
}

const EVENTS: WebhookEvent[] = [
  { event: "post.published", description: "All platforms published successfully", status: "live" },
  { event: "post.failed", description: "All platforms failed", status: "live" },
  { event: "post.partial", description: "Some platforms succeeded, some failed", status: "live" },
  { event: "account.connected", description: "User connected a social account (via Connect flow or dashboard)", status: "live" },
  { event: "account.disconnected", description: "Account disconnected or token permanently expired", status: "live" },
  { event: "post.scheduled", description: "Post successfully queued for future publishing", status: "coming_soon" },
  { event: "account.refreshed", description: "Managed account token refreshed successfully", status: "coming_soon" },
  { event: "account.quota_warning", description: "Account reached 80% of monthly per-account limit", status: "coming_soon" },
  { event: "account.quota_exceeded", description: "Account exceeded monthly per-account limit", status: "coming_soon" },
];

const PAYLOAD_PUBLISHED = `{
  "event": "post.published",
  "timestamp": "2026-04-08T10:00:00Z",
  "data": {
    "id": "post_abc123",
    "caption": "Hello from UniPost!",
    "status": "published",
    "created_at": "2026-04-08T10:00:00Z",
    "results": [
      {
        "social_account_id": "sa_instagram_123",
        "platform": "instagram",
        "status": "published",
        "external_id": "17841234567890",
        "published_at": "2026-04-08T10:00:01Z"
      }
    ]
  }
}`;

const PAYLOAD_CONNECTED = `{
  "event": "account.connected",
  "timestamp": "2026-04-08T10:00:00Z",
  "data": {
    "social_account_id": "sa_twitter_789",
    "platform": "twitter",
    "account_name": "@example",
    "external_user_id": "your_user_123",
    "connection_type": "managed"
  }
}`;

const VERIFY_SNIPPETS = [
  { lang: "curl", label: "cURL", code: `# UniPost POSTs the webhook event to your URL with these headers:
#
#   X-UniPost-Signature: sha256=<hex-of-hmac-sha256(secret, body)>
#   X-UniPost-Event: post.published
#   X-UniPost-Delivery: dlv_abc123
#
# Replay locally to test your handler:
curl -X POST "https://your-app.example.com/webhooks/unipost" \\
  -H "Content-Type: application/json" \\
  -H "X-UniPost-Signature: sha256=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | awk '{print $2}')" \\
  -H "X-UniPost-Event: post.published" \\
  --data "$BODY"` },
  { lang: "js", label: "Node.js", code: `import { verifyWebhookSignature } from "@unipost/sdk";

app.post("/webhooks/unipost", async (req, res) => {
  const isValid = await verifyWebhookSignature({
    payload: JSON.stringify(req.body),
    signature: req.headers["x-unipost-signature"],
    secret: process.env.WEBHOOK_SECRET,
  });

  if (!isValid) {
    return res.status(401).json({ error: "Invalid signature" });
  }

  const { event, data } = req.body;
  console.log(event, data.id);

  res.status(200).json({ received: true });
});` },
  { lang: "python", label: "Python", code: `from unipost import verify_webhook_signature
from flask import Flask, request, jsonify
import os

app = Flask(__name__)

@app.post('/webhooks/unipost')
def handle_webhook():
    signature = request.headers.get('X-UniPost-Signature', '')
    is_valid = verify_webhook_signature(
        request.get_data(),
        signature,
        os.environ['WEBHOOK_SECRET'],
    )

    if not is_valid:
        return jsonify({'error': 'Invalid signature'}), 401

    payload = request.get_json()
    if payload['event'] == 'post.published':
        print(f"Published: {payload['data']['id']}")

    return jsonify({'received': True})` },
  { lang: "go", label: "Go", code: `package main

import (
  "encoding/json"
  "io"
  "net/http"
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

func handleWebhook(w http.ResponseWriter, r *http.Request) {
  body, err := io.ReadAll(r.Body)
  if err != nil {
    http.Error(w, "read error", http.StatusBadRequest)
    return
  }

  signature := r.Header.Get("X-UniPost-Signature")
  if !unipost.VerifyWebhookSignature(body, signature, os.Getenv("WEBHOOK_SECRET")) {
    http.Error(w, "invalid signature", http.StatusUnauthorized)
    return
  }

  var event struct {
    Event string         \`json:"event"\`
    Data  map[string]any \`json:"data"\`
  }
  if err := json.Unmarshal(body, &event); err != nil {
    http.Error(w, "bad payload", http.StatusBadRequest)
    return
  }

  w.WriteHeader(http.StatusOK)
  _, _ = w.Write([]byte(\`{"received":true}\`))
}` },
];

export function WebhooksContent() {
  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify({
        "@context": "https://schema.org", "@type": "TechArticle",
        name: "UniPost API — Webhooks Reference",
        description: "Webhook events for social media post lifecycle and account status changes",
        url: "https://unipost.dev/docs/api/webhooks",
        author: { "@type": "Organization", name: "UniPost" }, dateModified: "2026-04-09",
      })}} />

      <Breadcrumbs items={[
        { label: "Docs", href: "/docs" },
        { label: "API Reference" },
        { label: "Webhooks" },
      ]} />

      <div style={{ marginBottom: 32 }}>
        <h1 style={{ fontSize: 32, fontWeight: 900, letterSpacing: "-.8px", marginBottom: 12 }}>Webhooks</h1>
      </div>

      {/* Setup */}
      <DocSection id="setup" title="Setup">
        <p style={{ fontSize: 14, color: "var(--docs-text-soft)", lineHeight: 1.6, marginBottom: 16 }}>
          Create a webhook subscription via the API. The signing secret is returned <strong style={{ color: "var(--docs-text)" }}>once</strong> in the create response — store it securely. Use <ApiInlineLink endpoint="POST /v1/webhooks/:id/rotate" /> to generate a new secret if needed.
        </p>
        <InfoBox>
          <strong style={{ color: "var(--docs-link)" }}>SDK note</strong><br />
          Webhook subscription management is not in the SDK yet. Keep webhook create / rotate on the REST API for now, then use the SDK helper below to verify incoming signatures in your app.
        </InfoBox>
      </DocSection>

      {/* Events table */}
      <DocSection id="events" title="Events">
        <div style={{ border: "1px solid var(--docs-border)", borderRadius: 10, overflow: "hidden", background: "var(--docs-bg-elevated)" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
            <thead>
              <tr style={{ background: "var(--docs-bg-muted)" }}>
                <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Event</th>
                <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Description</th>
                <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Status</th>
              </tr>
            </thead>
            <tbody>
              {EVENTS.map((e, i) => (
                <tr key={e.event} style={{ borderBottom: i < EVENTS.length - 1 ? "1px solid var(--docs-border)" : undefined }}>
                  <td style={{ padding: "10px 14px", fontFamily: "var(--docs-mono)", color: "var(--docs-text)", fontSize: 12.5, fontWeight: 500 }}>{e.event}</td>
                  <td style={{ padding: "10px 14px", color: "var(--docs-text-soft)" }}>{e.description}</td>
                  <td style={{ padding: "10px 14px" }}>
                    {e.status === "live" ? (
                      <span style={{ fontSize: 11, padding: "2px 7px", borderRadius: 4, background: "#10b98118", color: "#10b981", fontWeight: 600, fontFamily: "var(--docs-mono)" }}>Live</span>
                    ) : (
                      <span style={{ fontSize: 11, padding: "2px 7px", borderRadius: 4, background: "#f59e0b14", color: "#f59e0b", fontWeight: 600, fontFamily: "var(--docs-mono)" }}>Coming soon</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </DocSection>

      {/* Payloads */}
      <DocSection id="payloads" title="Payload Examples">
        <p style={{ fontSize: 13, fontWeight: 600, color: "var(--docs-text-muted)", marginBottom: 10, fontFamily: "var(--docs-mono)" }}>post.published</p>
        <ResponseBlock title="Payload" code={PAYLOAD_PUBLISHED} />
        <p style={{ fontSize: 13, fontWeight: 600, color: "var(--docs-text-muted)", marginBottom: 10, marginTop: 24, fontFamily: "var(--docs-mono)" }}>account.connected</p>
        <ResponseBlock title="Payload" code={PAYLOAD_CONNECTED} />
      </DocSection>

      {/* Verification */}
      <DocSection id="verification" title="Signature Verification">
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 16 }}>
          Every webhook request includes an <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>X-UniPost-Signature</code> header with the format <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>sha256=&lt;hex&gt;</code>. Always verify this signature before processing the payload.
        </p>
        <InfoBox>
          <strong style={{ color: "#ef4444" }}>Security: always verify signatures</strong><br />
          Without verification, any HTTP client can POST to your webhook URL and impersonate UniPost. Use <code>timingSafeEqual</code> (Node.js) or <code>hmac.compare_digest</code> (Python) to prevent timing attacks.
        </InfoBox>
        <CodeTabs snippets={VERIFY_SNIPPETS} />
      </DocSection>

      {/* Retry behavior */}
      <DocSection id="retry" title="Retry Behavior">
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 12 }}>
          UniPost retries failed webhook deliveries up to <strong style={{ color: "var(--docs-text)" }}>3 times</strong> with exponential backoff (30s, 2min, 10min). A delivery is considered failed when:
        </p>
        <ul style={{ paddingLeft: 18, margin: 0 }}>
          <li style={{ fontSize: 14, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 4 }}>Your endpoint returns a non-2xx status code</li>
          <li style={{ fontSize: 14, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 4 }}>Your endpoint doesn&apos;t respond within 10 seconds</li>
          <li style={{ fontSize: 14, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 4 }}>The connection is refused or times out</li>
        </ul>
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7, marginTop: 12 }}>
          After 3 failures on the same event, the delivery is marked as permanently failed. Persistent failures across multiple events may trigger automatic webhook disabling — check the webhook status via <ApiInlineLink endpoint="GET /v1/webhooks/:id" />.
        </p>
      </DocSection>

      {/* Related */}
      <DocSection id="related" title="Related">
        <RelatedEndpoints items={[
          { method: "POST", path: "/v1/posts", label: "Create post (triggers post.published)", href: "/docs/api/posts/create" },
          { method: "POST", path: "/v1/connect/sessions", label: "Connect session (triggers account.connected)", href: "/docs/api/connect/sessions" },
          { method: "GET", path: "/v1/accounts", label: "List accounts", href: "/docs/api/accounts/list" },
        ]} />
      </DocSection>

      <div style={{ marginTop: 48, paddingTop: 24, borderTop: "1px solid var(--docs-border)", fontSize: 13, color: "var(--docs-text-faint)" }}>
        <a href="/docs" style={{ color: "var(--docs-link)", textDecoration: "none" }}>&larr; View full docs</a>
        <span style={{ margin: "0 12px" }}>|</span>
        <span>Last updated: April 2026 &middot; API v1</span>
      </div>
    </>
  );
}

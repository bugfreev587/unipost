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

const PARTIAL_PAYLOAD = `{
  "event": "post.partial",
  "timestamp": "2026-04-23T18:00:00Z",
  "data": {
    "id": "post_partial_123",
    "status": "partial",
    "execution_mode": "async",
    "created_at": "2026-04-23T17:59:48Z",
    "results": [
      {
        "id": "spr_ok",
        "social_account_id": "sa_twitter_789",
        "platform": "twitter",
        "status": "published",
        "external_id": "191234567890",
        "url": "https://x.com/unipost/status/191234567890",
        "published_at": "2026-04-23T18:00:00Z"
      },
      {
        "id": "spr_fail",
        "social_account_id": "sa_linkedin_456",
        "platform": "linkedin",
        "status": "failed",
        "error_message": "LinkedIn rejected the caption because it exceeded the platform limit."
      }
    ]
  }
}`;

const FAILED_PAYLOAD = `{
  "event": "post.failed",
  "timestamp": "2026-04-23T18:00:00Z",
  "data": {
    "id": "post_failed_123",
    "status": "failed",
    "execution_mode": "async",
    "created_at": "2026-04-23T17:59:48Z",
    "results": [
      {
        "id": "spr_a",
        "social_account_id": "sa_instagram_123",
        "platform": "instagram",
        "status": "failed",
        "error_message": "Instagram rejected the media because the aspect ratio was unsupported."
      },
      {
        "id": "spr_b",
        "social_account_id": "sa_threads_456",
        "platform": "threads",
        "status": "failed",
        "error_message": "Threads rejected the request because the token was expired."
      }
    ]
  }
}`;

const VERIFY_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `# UniPost POSTs the webhook event to your URL with these headers:
#
#   X-UniPost-Signature: sha256=<hex-of-hmac-sha256(secret, body)>
#   X-UniPost-Event: post.published
#   X-UniPost-Delivery: dlv_abc123
#
# Replay locally to test your handler:
curl -X POST "https://your-app.example.com/webhooks/unipost" \\
  -H "Content-Type: application/json" \\
  -H "X-UniPost-Signature: sha256=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$UNIPOST_WEBHOOK_SECRET" | awk '{print $2}')" \\
  -H "X-UniPost-Event: post.published" \\
  --data "$BODY"`,
  },
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
  {
    lang: "python",
    label: "Python",
    code: `from unipost import verify_webhook_signature
from flask import Flask, request, jsonify
import os

app = Flask(__name__)

@app.post("/webhooks/unipost")
def handle_webhook():
    signature = request.headers.get("X-UniPost-Signature", "")
    is_valid = verify_webhook_signature(
        request.get_data(),
        signature,
        os.environ["UNIPOST_WEBHOOK_SECRET"],
    )

    if not is_valid:
        return jsonify({"error": "Invalid signature"}), 401

    payload = request.get_json()
    print(payload["event"], payload["data"]["id"])
    return jsonify({"received": True})`,
  },
  {
    lang: "go",
    label: "Go",
    code: `package main

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
  if !unipost.VerifyWebhookSignature(body, signature, os.Getenv("UNIPOST_WEBHOOK_SECRET")) {
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
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;

void handleWebhook(HttpExchange exchange) throws Exception {
    byte[] body = exchange.getRequestBody().readAllBytes();
    String signature = exchange.getRequestHeaders().getFirst("X-UniPost-Signature");
    String secret = System.getenv("UNIPOST_WEBHOOK_SECRET");

    if (!UniPost.verifyWebhookSignature(body, signature, secret)) {
        exchange.sendResponseHeaders(401, -1);
        return;
    }

    JsonNode payload = new ObjectMapper().readTree(body);
    System.out.println(payload.get("event").asText() + " " + payload.get("data").get("id").asText());

    byte[] response = "{\\"received\\":true}".getBytes(StandardCharsets.UTF_8);
    exchange.getResponseHeaders().add("Content-Type", "application/json");
    exchange.sendResponseHeaders(200, response.length);
    try (OutputStream os = exchange.getResponseBody()) {
        os.write(response);
    }
}`,
  },
];

export default function WebhooksPage() {
  return (
    <ApiReferencePage
      section="developer-webhooks"
      title="Developer webhooks"
      description="Use developer webhooks when your own backend needs push delivery for async post results or account lifecycle events. Create each subscription with a name, a destination URL, and an event set. This is the machine-facing surface for product integrations, not the dashboard notifications channel."
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
            <div style={{ marginTop: 18 }} />
            <ResponseBlock title="post.partial" code={PARTIAL_PAYLOAD} />
            <div style={{ marginTop: 18 }} />
            <ResponseBlock title="post.failed" code={FAILED_PAYLOAD} />
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
            <InfoBox>
              <strong style={{ color: "var(--docs-text)" }}>How to read webhook results:</strong> the <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>data</code> object is the same aggregate post shape your client would receive from <ApiInlineLink endpoint="GET /v1/posts/:post_id" />. Use the top-level status for the summary, then inspect <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>results[]</code> for destination-by-destination detail.
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

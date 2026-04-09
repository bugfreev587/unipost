"use client";

import {
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
  { lang: "js", label: "Node.js", code: `const crypto = require('crypto');

function verifyWebhookSignature(payload, signature, secret) {
  const expected = crypto
    .createHmac('sha256', secret)
    .update(payload, 'utf8')
    .digest('hex');

  const expectedSig = 'sha256=' + expected;

  // Use timingSafeEqual to prevent timing attacks
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(expectedSig)
  );
}

// In your webhook handler:
app.post('/webhooks/unipost', (req, res) => {
  const signature = req.headers['x-unipost-signature'];
  const isValid = verifyWebhookSignature(
    JSON.stringify(req.body),
    signature,
    process.env.WEBHOOK_SECRET  // whsec_xxxx from POST /v1/webhooks
  );

  if (!isValid) {
    return res.status(401).json({ error: 'Invalid signature' });
  }

  const { event, data } = req.body;
  switch (event) {
    case 'post.published':
      console.log('Published:', data.id);
      break;
    case 'post.failed':
      console.error('Failed:', data.id, data.results);
      break;
    case 'account.connected':
      console.log('New account:', data.social_account_id);
      break;
  }

  res.status(200).json({ received: true });
});` },
  { lang: "python", label: "Python", code: `import hmac
import hashlib

def verify_webhook(payload: bytes, signature: str, secret: str) -> bool:
    expected = hmac.new(
        secret.encode('utf-8'),
        payload,
        hashlib.sha256
    ).hexdigest()
    expected_sig = f'sha256={expected}'
    return hmac.compare_digest(signature, expected_sig)

# In your Flask/FastAPI handler:
@app.post('/webhooks/unipost')
def handle_webhook(request):
    signature = request.headers.get('X-UniPost-Signature')
    is_valid = verify_webhook(
        request.body,
        signature,
        os.environ['WEBHOOK_SECRET']
    )

    if not is_valid:
        return {'error': 'Invalid signature'}, 401

    event = request.json['event']
    data = request.json['data']

    if event == 'post.published':
        print(f"Published: {data['id']}")

    return {'received': True}` },
];

const SETUP_SNIPPETS = [
  { lang: "curl", label: "cURL", code: `# Create a webhook subscription
curl -X POST https://api.unipost.dev/v1/webhooks \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "url": "https://yourapp.com/webhooks/unipost",
    "events": ["post.published", "post.failed", "account.connected"]
  }'

# Response (save the secret — it's shown only once):
# {
#   "data": {
#     "id": "wh_abc123",
#     "url": "https://yourapp.com/webhooks/unipost",
#     "events": ["post.published", "post.failed", "account.connected"],
#     "secret": "whsec_a1b2c3d4e5f6..."
#   }
# }

# Rotate the signing secret
curl -X POST https://api.unipost.dev/v1/webhooks/wh_abc123/rotate \\
  -H "Authorization: Bearer up_live_xxxx"` },
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
        <p style={{ fontSize: 15, color: "#aaa", lineHeight: 1.6 }}>
          Receive real-time notifications when posts are published, accounts are connected, or tokens expire. UniPost sends HMAC-signed HTTP POST requests to the URL you configure.
        </p>
      </div>

      {/* Setup */}
      <DocSection id="setup" title="Setup">
        <p style={{ fontSize: 14, color: "#aaa", lineHeight: 1.6, marginBottom: 16 }}>
          Create a webhook subscription via the API. The signing secret is returned <strong style={{ color: "#f0f0f0" }}>once</strong> in the create response — store it securely. Use <code style={{ color: "#10b981", fontFamily: "var(--mono)", fontSize: 13 }}>POST /v1/webhooks/:id/rotate</code> to generate a new secret if needed.
        </p>
        <CodeTabs snippets={SETUP_SNIPPETS} />
      </DocSection>

      {/* Events table */}
      <DocSection id="events" title="Events">
        <div style={{ border: "1px solid #1a1a1a", borderRadius: 10, overflow: "hidden" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
            <thead>
              <tr style={{ background: "#0a0a0a" }}>
                <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>Event</th>
                <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>Description</th>
                <th style={{ textAlign: "left", padding: "10px 14px", color: "#666", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid #1a1a1a" }}>Status</th>
              </tr>
            </thead>
            <tbody>
              {EVENTS.map((e, i) => (
                <tr key={e.event} style={{ borderBottom: i < EVENTS.length - 1 ? "1px solid #111" : undefined }}>
                  <td style={{ padding: "10px 14px", fontFamily: "var(--mono)", color: "#f0f0f0", fontSize: 12.5, fontWeight: 500 }}>{e.event}</td>
                  <td style={{ padding: "10px 14px", color: "#999" }}>{e.description}</td>
                  <td style={{ padding: "10px 14px" }}>
                    {e.status === "live" ? (
                      <span style={{ fontSize: 11, padding: "2px 7px", borderRadius: 4, background: "#10b98118", color: "#10b981", fontWeight: 600, fontFamily: "var(--mono)" }}>Live</span>
                    ) : (
                      <span style={{ fontSize: 11, padding: "2px 7px", borderRadius: 4, background: "#f59e0b14", color: "#f59e0b", fontWeight: 600, fontFamily: "var(--mono)" }}>Coming soon</span>
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
        <p style={{ fontSize: 13, fontWeight: 600, color: "#888", marginBottom: 10, fontFamily: "var(--mono)" }}>post.published</p>
        <ResponseBlock title="Payload" code={PAYLOAD_PUBLISHED} />
        <p style={{ fontSize: 13, fontWeight: 600, color: "#888", marginBottom: 10, marginTop: 24, fontFamily: "var(--mono)" }}>account.connected</p>
        <ResponseBlock title="Payload" code={PAYLOAD_CONNECTED} />
      </DocSection>

      {/* Verification */}
      <DocSection id="verification" title="Signature Verification">
        <p style={{ fontSize: 14.5, color: "#aaa", lineHeight: 1.7, marginBottom: 16 }}>
          Every webhook request includes an <code style={{ color: "#10b981", fontFamily: "var(--mono)", fontSize: 13 }}>X-UniPost-Signature</code> header with the format <code style={{ color: "#10b981", fontFamily: "var(--mono)", fontSize: 13 }}>sha256=&lt;hex&gt;</code>. Always verify this signature before processing the payload.
        </p>
        <InfoBox>
          <strong style={{ color: "#ef4444" }}>Security: always verify signatures</strong><br />
          Without verification, any HTTP client can POST to your webhook URL and impersonate UniPost. Use <code>timingSafeEqual</code> (Node.js) or <code>hmac.compare_digest</code> (Python) to prevent timing attacks.
        </InfoBox>
        <CodeTabs snippets={VERIFY_SNIPPETS} />
      </DocSection>

      {/* Retry behavior */}
      <DocSection id="retry" title="Retry Behavior">
        <p style={{ fontSize: 14.5, color: "#aaa", lineHeight: 1.7, marginBottom: 12 }}>
          UniPost retries failed webhook deliveries up to <strong style={{ color: "#f0f0f0" }}>3 times</strong> with exponential backoff (30s, 2min, 10min). A delivery is considered failed when:
        </p>
        <ul style={{ paddingLeft: 18, margin: 0 }}>
          <li style={{ fontSize: 14, color: "#aaa", lineHeight: 1.7, marginBottom: 4 }}>Your endpoint returns a non-2xx status code</li>
          <li style={{ fontSize: 14, color: "#aaa", lineHeight: 1.7, marginBottom: 4 }}>Your endpoint doesn&apos;t respond within 10 seconds</li>
          <li style={{ fontSize: 14, color: "#aaa", lineHeight: 1.7, marginBottom: 4 }}>The connection is refused or times out</li>
        </ul>
        <p style={{ fontSize: 14.5, color: "#aaa", lineHeight: 1.7, marginTop: 12 }}>
          After 3 failures on the same event, the delivery is marked as permanently failed. Persistent failures across multiple events may trigger automatic webhook disabling — check the webhook status via <code style={{ color: "#10b981", fontFamily: "var(--mono)", fontSize: 13 }}>GET /v1/webhooks/:id</code>.
        </p>
      </DocSection>

      {/* Related */}
      <DocSection id="related" title="Related">
        <RelatedEndpoints items={[
          { method: "POST", path: "/v1/social-posts", label: "Create post (triggers post.published)", href: "/docs/api/posts/create" },
          { method: "POST", path: "/v1/connect/sessions", label: "Connect session (triggers account.connected)", href: "/docs/api/connect/sessions" },
          { method: "GET", path: "/v1/social-accounts", label: "List accounts", href: "/docs/api/accounts/list" },
        ]} />
      </DocSection>

      <div style={{ marginTop: 48, paddingTop: 24, borderTop: "1px solid #1a1a1a", fontSize: 13, color: "#555" }}>
        <a href="/docs" style={{ color: "#0ea5e9", textDecoration: "none" }}>&larr; View full docs</a>
        <span style={{ margin: "0 12px" }}>|</span>
        <span>Last updated: April 2026 &middot; API v1</span>
      </div>
    </>
  );
}

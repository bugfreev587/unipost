"use client";

import { useState } from "react";

const LANGS = [
  { id: "js", label: "JavaScript" },
  { id: "python", label: "Python" },
  { id: "go", label: "Go" },
  { id: "curl", label: "cURL" },
] as const;

const CODE_SNIPPETS: Record<(typeof LANGS)[number]["id"], string> = {
  js: `// First reserve a local upload with POST /v1/media, then PUT the file to upload_url.
const response = await fetch(
  'https://api.unipost.dev/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxxx',
      'Content-Type':  'application/json',
    },
    body: JSON.stringify({
      platform_posts: [
        {
          account_id: 'sa_x_123',
          caption: 'Shipped validate + preview today',
          media_ids: ['med_uploaded_image_1'],
        },
        {
          account_id: 'sa_youtube_456',
          caption: 'Quarterly product update is live',
          media_ids: ['med_uploaded_video_1'],
        },
      ],
      idempotency_key: 'launch-2026-04-13-001',
    }),
  }
);

const { data } = await response.json();
console.log(data.id); // post_abc123`,
  python: `import requests

# First reserve a local upload with POST /v1/media, then PUT the file to upload_url.

response = requests.post(
    'https://api.unipost.dev/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxxx',
        'Content-Type':  'application/json',
    },
    json={
        'platform_posts': [
            {
                'account_id': 'sa_x_123',
                'caption': 'Shipped validate + preview today',
                'media_ids': ['med_uploaded_image_1'],
            },
            {
                'account_id': 'sa_youtube_456',
                'caption': 'Quarterly product update is live',
                'media_ids': ['med_uploaded_video_1'],
            },
        ],
        'idempotency_key': 'launch-2026-04-13-001',
    }
)

data = response.json()['data']
print(data['id'])  # post_abc123`,
  go: `// First reserve a local upload with POST /v1/media, then PUT the file to upload_url.
req, _ := http.NewRequest("POST",
    "https://api.unipost.dev/v1/social-posts",
    strings.NewReader(\`{
      "platform_posts": [
        {
          "account_id": "sa_x_123",
          "caption": "Shipped validate + preview today",
          "media_ids": ["med_uploaded_image_1"]
        },
        {
          "account_id": "sa_youtube_456",
          "caption": "Quarterly product update is live",
          "media_ids": ["med_uploaded_video_1"]
        }
      ],
      "idempotency_key": "launch-2026-04-13-001"
    }\`),
)

req.Header.Set("Authorization", "Bearer up_live_xxxx")
req.Header.Set("Content-Type",  "application/json")

resp, _ := http.DefaultClient.Do(req)
// resp.StatusCode == 200 ✓`,
  curl: `# First reserve a local upload with POST /v1/media, then PUT the file to upload_url.
curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_x_123",
        "caption": "Shipped validate + preview today",
        "media_ids": ["med_uploaded_image_1"]
      },
      {
        "account_id": "sa_youtube_456",
        "caption": "Quarterly product update is live",
        "media_ids": ["med_uploaded_video_1"]
      }
    ],
    "idempotency_key": "launch-2026-04-13-001"
  }'`,
};

export function LandingCodeTabs() {
  const [activeLang, setActiveLang] = useState<(typeof LANGS)[number]["id"]>("js");

  return (
    <div className="lp-integ-right">
      <div className="lp-code-tabs-bar">
        {LANGS.map((lang) => (
          <button
            key={lang.id}
            className={`lp-code-tab ${activeLang === lang.id ? "active" : ""}`}
            onClick={() => setActiveLang(lang.id)}
          >
            {lang.label}
          </button>
        ))}
      </div>
      <div className="lp-editor">
        <pre className="lp-editor-code">
          {CODE_SNIPPETS[activeLang].split("\n").map((line, index) => (
            <div key={index} className="lp-editor-line">
              <span className="lp-editor-ln">{index + 1}</span>
              <span className="lp-editor-text">{line}</span>
            </div>
          ))}
        </pre>
      </div>
    </div>
  );
}

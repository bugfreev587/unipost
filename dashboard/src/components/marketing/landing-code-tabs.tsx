"use client";

import { useState } from "react";

const LANGS = [
  { id: "js", label: "JavaScript" },
  { id: "python", label: "Python" },
  { id: "go", label: "Go" },
  { id: "curl", label: "cURL" },
] as const;

const CODE_SNIPPETS: Record<(typeof LANGS)[number]["id"], string> = {
  js: `const response = await fetch(
  'https://api.unipost.dev/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxxx',
      'Content-Type':  'application/json',
    },
    body: JSON.stringify({
      caption:     'Hello from UniPost! 🚀',
      account_ids: ['sa_instagram_123', 'sa_linkedin_456'],
    }),
  }
);

const { data } = await response.json();
console.log(data.id); // post_abc123`,
  python: `import requests

response = requests.post(
    'https://api.unipost.dev/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxxx',
        'Content-Type':  'application/json',
    },
    json={
        'caption':     'Hello from UniPost! 🚀',
        'account_ids': ['sa_instagram_123', 'sa_linkedin_456'],
    }
)

data = response.json()['data']
print(data['id'])  # post_abc123`,
  go: `req, _ := http.NewRequest("POST",
    "https://api.unipost.dev/v1/social-posts",
    strings.NewReader(\`{
      "caption":     "Hello from UniPost! 🚀",
      "account_ids": ["sa_instagram_123", "sa_linkedin_456"]
    }\`),
)

req.Header.Set("Authorization", "Bearer up_live_xxxx")
req.Header.Set("Content-Type",  "application/json")

resp, _ := http.DefaultClient.Do(req)
// resp.StatusCode == 200 ✓`,
  curl: `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption":     "Hello from UniPost! 🚀",
    "account_ids": ["sa_instagram_123", "sa_linkedin_456"]
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

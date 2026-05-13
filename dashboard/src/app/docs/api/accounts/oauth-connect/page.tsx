"use client";

import Link from "next/link";
import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "platform", type: "string", description: <>OAuth platform key such as <code>linkedin</code>, <code>twitter</code>, <code>youtube</code>, <code>instagram</code>, <code>threads</code>, <code>tiktok</code>, or <code>pinterest</code>. See <Link href="/docs/platforms#platform-names">available platforms</Link>.</> },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "redirect_url?", type: "string", description: "Optional app URL where the browser should land after OAuth completes." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "auth_url", type: "string", description: "Browser URL to open for the OAuth authorization flow." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Common values include "UNAUTHORIZED", "VALIDATION_ERROR", and "FACEBOOK_DISABLED".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/oauth/connect/linkedin?redirect_url=https://app.acme.com/integrations/done"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `const res = await fetch(
  "https://api.unipost.dev/v1/oauth/connect/linkedin?redirect_url=https://app.acme.com/integrations/done",
  {
    headers: {
      Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\`,
    },
  }
);

const body = await res.json();
console.log(body.data.auth_url);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `import os
import requests

res = requests.get(
  "https://api.unipost.dev/v1/oauth/connect/linkedin",
  params={"redirect_url": "https://app.acme.com/integrations/done"},
  headers={"Authorization": f"Bearer {os.environ['UNIPOST_API_KEY']}"},
)

print(res.json()["data"]["auth_url"])`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "auth_url": "https://www.linkedin.com/oauth/v2/authorization?response_type=code&client_id=..."
  }
}`,
  },
];

export default function OAuthConnectPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Get OAuth connect URL"
      description="Returns an auth_url for connecting one self-owned social account in the current workspace context. Open the returned URL in a browser to complete OAuth, then list accounts to find the new UniPost account ID."
      method="GET"
      path="/v1/oauth/connect/:platform"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Parameters", items: PATH_FIELDS },
        { title: "Query Parameters", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "403", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}

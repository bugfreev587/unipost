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
    code: `curl "https://api.unipost.dev/v1/oauth/connect/linkedin" \
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `const { auth_url } = await client.connect.getConnectUrl({
  profileId: "pr_brand_us",
  platform: "linkedin",
});

console.log(auth_url);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `connect = client.connect.get_connect_url(
  profile_id="pr_brand_us",
  platform="linkedin",
)

print(connect.auth_url)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `connect, err := client.Connect.GetConnectURL(ctx, &unipost.GetConnectURLParams{
  ProfileID: "pr_brand_us",
  Platform:  "linkedin",
})
if err != nil {
  log.Fatal(err)
}

fmt.Println(connect.AuthURL)`,
  },
  {
    lang: "java",
    label: "Java",
    code: `var connect = client.connect().getConnectUrl(Map.of(
    "profile_id", "pr_brand_us",
    "platform", "linkedin"
));

System.out.println(connect.get("auth_url").asText());`,
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
      title="Connect account (OAuth)"
      description="Starts an OAuth account connection flow by returning an auth_url you open in the browser. Use this for OAuth platforms like LinkedIn, X, YouTube, Instagram, Threads, TikTok, and Pinterest, then list accounts afterward to find the new UniPost account ID."
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

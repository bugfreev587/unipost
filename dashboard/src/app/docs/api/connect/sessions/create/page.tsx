"use client";

import Link from "next/link";
import { EnumValues, type ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "platform", type: "string", description: <>Destination platform for the hosted onboarding flow.<EnumValues values={["twitter", "linkedin", "bluesky", "youtube", "tiktok", "instagram", "threads", "facebook", "pinterest"]} /></> },
  { name: "profile_id?", type: "string", description: "Profile that should own the resulting connected account. Required when the workspace has multiple profiles." },
  { name: "external_user_id", type: "string", description: "Your stable end-user identifier." },
  { name: "external_user_email?", type: "string", description: "Optional email for reconciliation and support." },
  { name: "return_url?", type: "string", description: "Where UniPost redirects the user after completion, cancellation, or a handled OAuth failure. This is not the OAuth callback URL or platform redirect_uri." },
  { name: "allow_quickstart_creds?", type: "boolean", description: "Optional escape hatch for OAuth platforms. Defaults to false. When false, the workspace must already have Platform Credentials uploaded for that platform. When true, UniPost may fall back to the shared Quickstart OAuth app if no workspace credentials exist; workspace credentials still take priority when present." },
];

const RESPONSE_201_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Connect session ID." },
  { name: "url", type: "string", description: "Hosted onboarding URL to redirect the user to." },
  { name: "allow_quickstart_creds", type: "boolean", description: "Whether this session is allowed to fall back to UniPost's shared Quickstart OAuth app when no workspace-specific credentials exist. Workspace credentials still take priority when present." },
  { name: "managed_account_id", type: "string", description: "Present after the session completes. Alias of completed_social_account_id for hosted Connect callers." },
  { name: "status", type: "string", description: <>Session lifecycle state. Create responses start as pending.<EnumValues values={["pending", "completed", "expired", "cancelled"]} /></> },
  { name: "expires_at", type: "string | null", description: "Expiration timestamp for the hosted session. New sessions expire after 30 minutes." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/connect/sessions" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "linkedin",
    "profile_id": "pr_brand_us",
    "external_user_id": "user_123",
    "external_user_email": "alex@acme.com",
    "return_url": "https://app.acme.com/integrations/done",
    "allow_quickstart_creds": true
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const session = await client.connect.createSession({
  platform: "pinterest",
  profileId: "pr_brand_us",
  externalUserId: "user_123",
  externalUserEmail: "alex@acme.com",
  returnUrl: "https://app.acme.com/integrations/done",
  allowQuickstartCreds: true,
});

console.log(session.url);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

session = client.connect.create_session(
  platform="linkedin",
  profile_id="pr_brand_us",
  external_user_id="user_123",
  external_user_email="alex@acme.com",
  return_url="https://app.acme.com/integrations/done",
  allow_quickstart_creds=True,
)

print(session["data"]["url"])`,
  },
  {
    lang: "go",
    label: "Go",
    code: `package main

import (
  "context"
  "fmt"
  "log"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient()

  session, err := client.Connect.CreateSession(context.Background(), &unipost.CreateConnectSessionParams{
    Platform:             "instagram",
    ProfileID:            "pr_brand_us",
    ExternalUserID:       "user_123",
    ExternalUserEmail:    "alex@acme.com",
    ReturnURL:            "https://app.acme.com/integrations/done",
    AllowQuickstartCreds: true,
  })
  if err != nil {
    log.Fatal(err)
  }

  fmt.Println(session.URL)
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

import java.util.Map;

UniPost client = new UniPost();

var session = client.connect().createSession(Map.of(
    "platform", "linkedin",
    "profile_id", "pr_brand_us",
    "external_user_id", "user_123",
    "external_user_email", "alex@acme.com",
    "return_url", "https://app.acme.com/integrations/done",
    "allow_quickstart_creds", true
));

System.out.println(session.get("url").asText());`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "201",
    code: `{
  "data": {
    "id": "cs_abc123",
    "platform": "linkedin",
    "url": "https://app.unipost.dev/connect/linkedin?session=cs_abc123&state=state_123",
    "allow_quickstart_creds": true,
    "status": "pending",
    "expires_at": "2026-04-22T18:00:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "401",
    code: `{
  "error": {
    "code": "UNAUTHORIZED",
    "normalized_code": "unauthorized",
    "message": "Missing or invalid API key."
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "422",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "workspace is missing tiktok platform credentials; upload workspace Platform Credentials first or pass allow_quickstart_creds=true"
  },
  "request_id": "req_123"
}`,
  },
];

export default function CreateConnectSessionPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Create connect session"
      description={
        <>
          Creates a hosted onboarding session for a customer-owned social account.
          Use the returned URL to send the end user into UniPost&apos;s managed
          Connect flow. For OAuth platforms, this endpoint defaults to
          workspace-credential mode: the workspace must already have Platform Credentials
          uploaded unless you explicitly pass <code>allow_quickstart_creds=true</code>.
          Facebook Page sessions connect the first publishable Page returned by
          Meta for the authorizing user.
          See the <Link href="/docs/connect-sessions">Connect Sessions guide</Link>{" "}
          for shared-app and workspace-credential behavior.
        </>
      }
      method="POST"
      path="/v1/connect/sessions"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "201", fields: RESPONSE_201_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "402", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <section className="api-field-section">
        <h2 className="api-field-section-title">Callback URLs</h2>
        <p>
          <code>return_url</code> only controls where the browser lands after
          UniPost finishes handling the session. Platform OAuth callback URLs are
          generated by UniPost. Quickstart sessions use UniPost&apos;s registered
          callbacks; workspace Platform Credentials must allow-list the exact
          callback URL shown in the <Link href="/docs/platform-credentials">Platform Credentials guides</Link>.
        </p>
      </section>
    </SingleEndpointReferencePage>
  );
}

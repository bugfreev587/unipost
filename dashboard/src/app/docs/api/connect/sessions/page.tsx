import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";

const CREATE_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const session = await client.connect.createSession({
  platform: "twitter",
  externalUserId: "user_123",
  externalUserEmail: "alex@acme.com",
  returnUrl: "https://app.acme.com/integrations/done",
});

console.log(session.url);`,
  },
];

const GET_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const session = await client.connect.getSession("cs_abc123");
console.log(session.status);`,
  },
];

export default function ConnectSessionsPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Connect Sessions"
      lead="Connect sessions are the hosted onboarding layer for customer-owned social accounts. They let your app create a session, redirect an end user into UniPost&apos;s hosted flow, and receive a managed social account when the flow completes."
    >
      <h2 id="when-to-use">When to use Connect sessions</h2>
      <p>Use Connect sessions when you are building a product where your customers connect their own end-user-owned social accounts. This is how UniPost becomes account-onboarding infrastructure rather than just a direct posting API.</p>
      <p>New to this pattern? Start with the <Link href="/docs/white-label">white-label guide</Link> — it walks through the full setup (OAuth app, branding, credentials) and returns here for the endpoint-level reference.</p>

      <h2 id="flow">Flow</h2>
      <DocsTable
        columns={["Step", "What happens"]}
        rows={[
          ["Create session", "Your backend creates a session and receives a hosted URL"],
          ["User completes hosted flow", "UniPost handles OAuth or Bluesky credential collection"],
          ["Managed account is created", "UniPost stores and refreshes the resulting social account"],
          ["Webhook or poll", "Your app learns the flow completed and can start publishing"],
        ]}
      />

      <h2 id="create">Create a session</h2>
      <p>Create a connect session with the platform, your own stable end-user identifier, and a return URL. The resulting URL is what you send or redirect your user to.</p>
      <DocsCodeTabs snippets={CREATE_SNIPPETS} />

      <h2 id="fields">Important fields</h2>
      <DocsTable
        columns={["Field", "Required", "Notes"]}
        rows={[
          ["platform", "Yes", "Currently used to choose the hosted flow and downstream platform semantics"],
          ["external_user_id", "Yes", "Your stable identifier for the end user"],
          ["external_user_email", "No", "Helpful for your own reconciliation and support workflows"],
          ["return_url", "Recommended", "Where UniPost redirects the user after completion"],
        ]}
      />

      <h2 id="read">Read session status</h2>
      <p>Polling is available for development and simple integrations. In production, the recommended path is to listen for the account-connected webhook and then look up the resulting account.</p>
      <DocsCodeTabs snippets={GET_SNIPPETS} />

      <h2 id="status-model">Status model</h2>
      <DocsTable
        columns={["Status", "Meaning"]}
        rows={[
          ["pending", "The user has not completed the hosted flow yet"],
          ["completed", "The session produced a managed social account"],
          ["expired", "The hosted flow was not completed before expiration"],
        ]}
      />
    </DocsPage>
  );
}

import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";

const USERS_SNIPPETS = [
  {
    label: "cURL",
    code: `curl https://api.unipost.dev/v1/users \\
  -H "Authorization: Bearer up_live_xxxx"`,
  },
  {
    label: "cURL (detail)",
    code: `curl https://api.unipost.dev/v1/users/user_abc \\
  -H "Authorization: Bearer up_live_xxxx"`,
  },
];

export default function ManagedUsersPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Managed Users"
      lead="Managed Users groups customer-owned social accounts by your own `external_user_id`. This is the bridge between your product&apos;s user model and the accounts that UniPost manages on their behalf."
    >
      <h2 id="overview">Overview</h2>
      <p>When you use Connect sessions, UniPost stores the resulting social accounts and associates them with the `external_user_id` you passed in. Managed Users gives you a way to inspect that grouping directly.</p>

      <h2 id="list">List users</h2>
      <p>Use the users list when you want to know which of your end users have connected accounts, how many platforms they have connected, and whether any reconnect work is required.</p>
      <DocsCodeTabs snippets={USERS_SNIPPETS} />

      <h2 id="responses">What you get back</h2>
      <DocsTable
        columns={["Field", "Meaning"]}
        rows={[
          ["external_user_id", "Your own stable end-user identifier"],
          ["external_user_email", "Optional email captured during onboarding"],
          ["account_count", "Total number of connected accounts for that user"],
          ["platform_counts", "Breakdown by social platform"],
          ["reconnect_count", "How many accounts need reconnect attention"],
        ]}
      />

      <h2 id="detail">User detail</h2>
      <p>The detail endpoint returns the actual social account rows for one end user. That is the right endpoint for rendering a &ldquo;connected accounts&rdquo; page in your own product.</p>
    </DocsPage>
  );
}

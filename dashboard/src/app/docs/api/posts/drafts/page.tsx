import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";

const DRAFT_SNIPPETS = [
  {
    label: "Create draft",
    code: `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "account_ids": ["sa_twitter_1"],
    "caption": "Work in progress",
    "status": "draft"
  }'`,
  },
  {
    label: "Publish draft",
    code: `curl -X POST https://api.unipost.dev/v1/social-posts/post_abc123/publish \\
  -H "Authorization: Bearer up_live_xxxx"`,
  },
  {
    label: "Preview link",
    code: `curl -X POST https://api.unipost.dev/v1/social-posts/post_abc123/preview-link \\
  -H "Authorization: Bearer up_live_xxxx"`,
  },
];

export default function DraftsPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Drafts and Preview Links"
      lead="Drafts are real social-post rows stored with status `draft`. They let you save content before publish, run review flows, and generate signed preview links that show platform-specific output before anything goes live."
    >
      <h2 id="draft-model">Draft model</h2>
      <p>Drafts are not a separate resource type. They are created with the same endpoint as normal posts, but with <code>status: &quot;draft&quot;</code>. That means the same request body, validation behavior, and future publish semantics all stay aligned.</p>

      <h2 id="create">Create a draft</h2>
      <DocsCodeTabs snippets={[DRAFT_SNIPPETS[0]]} />

      <h2 id="publish">Publish a draft</h2>
      <p>When you are ready, promote the draft into a live publish using the publish endpoint. UniPost uses optimistic locking so concurrent draft promotions do not create duplicate platform posts.</p>
      <DocsCodeTabs snippets={[DRAFT_SNIPPETS[1]]} />

      <h2 id="preview">Generate a preview link</h2>
      <p>Preview links are signed and time-bounded. They let a human review the resolved content before you commit to publish. This is especially useful when an LLM or automation generated the first draft.</p>
      <DocsCodeTabs snippets={[DRAFT_SNIPPETS[2]]} />

      <h2 id="when-to-use">When drafts are the right choice</h2>
      <DocsTable
        columns={["Use case", "Why drafts help"]}
        rows={[
          ["Human review required", "A reviewer can inspect content before the publish step"],
          ["AI-generated first draft", "The model can draft, validate, and wait for approval"],
          ["Internal editorial workflows", "Drafts give you a save-now, publish-later state"],
        ]}
      />
    </DocsPage>
  );
}

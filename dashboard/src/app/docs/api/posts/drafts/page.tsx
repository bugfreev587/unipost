import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";

const DRAFT_SNIPPETS = [
  {
    label: "Create draft",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const draft = await client.posts.create({
  accountIds: ["sa_twitter_1"],
  caption: "Work in progress",
  status: "draft",
});`,
  },
  {
    label: "Publish draft",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

await client.posts.publish("post_abc123");`,
  },
  {
    label: "Preview link",
    code: ``,
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
      <p>Preview-link generation is not in the SDK yet. Use SDK draft creation + publish today, and keep preview-link generation on the REST endpoint if your workflow needs it.</p>

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

import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";

const MEDIA_SNIPPETS = [
  {
    label: "Reserve upload",
    code: `curl -X POST https://api.unipost.dev/v1/media \\
  -H "Authorization: Bearer up_live_xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "filename": "photo.jpg",
    "content_type": "image/jpeg",
    "size_bytes": 284192
  }'`,
  },
  {
    label: "Use media_ids in post",
    code: `{
  "platform_posts": [
    {
      "account_id": "sa_instagram_1",
      "caption": "Launch day.",
      "media_ids": ["med_abc123"]
    }
  ]
}`,
  },
];

export default function MediaPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Media"
      lead="The media library gives you a two-step upload flow backed by UniPost-managed storage. Use it when you do not want to depend on public asset URLs or when you want UniPost to resolve media IDs server-side during publish."
    >
      <h2 id="reserve">Reserve an upload</h2>
      <p>Create a media row and receive a presigned upload URL. The media stays in a pending state until the file lands and UniPost resolves its final metadata.</p>
      <DocsCodeTabs snippets={[MEDIA_SNIPPETS[0]]} />

      <h2 id="flow">Flow</h2>
      <DocsTable
        columns={["Step", "What happens"]}
        rows={[
          ["POST /v1/media", "Reserve a media row and receive a presigned URL"],
          ["PUT to storage", "Upload directly to storage using the returned URL"],
          ["GET /v1/media/{id}", "Read the resolved media row once the file is available"],
          ["Use media_ids in publish", "Reference the media row from `platform_posts[]`"],
        ]}
      />

      <h2 id="use-in-posts">Use media IDs in posts</h2>
      <p>Once uploaded, pass <code>media_ids</code> on a platform post entry. UniPost resolves those IDs to short-lived download URLs internally before handing off to platform adapters.</p>
      <DocsCodeTabs snippets={[MEDIA_SNIPPETS[1]]} />
    </DocsPage>
  );
}

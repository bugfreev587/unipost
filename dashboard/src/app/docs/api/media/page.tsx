import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";

const MEDIA_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const { mediaId, uploadUrl } = await client.media.upload({
  filename: "photo.jpg",
  contentType: "image/jpeg",
  sizeBytes: 284192,
});

await fetch(uploadUrl, {
  method: "PUT",
  body: fileBuffer,
});

console.log(mediaId);`,
  },
  {
    label: "Publish with mediaIds",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const mediaId = await client.media.uploadFile("./photo.jpg");

const post = await client.posts.create({
  platformPosts: [
    {
      accountId: "sa_instagram_1",
      caption: "Launch day.",
      mediaIds: [mediaId],
    },
  ],
});

console.log(post.id);`,
  },
];

export default function MediaPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Media"
      lead="The media library gives you a two-step upload flow backed by UniPost-managed storage. Use it when you do not want to depend on public asset URLs or when you want UniPost to resolve media IDs server-side during publish."
    >
      <h2 id="best-for">Best for</h2>
      <p>The media library is the recommended path for large local files, especially video destined for YouTube. It is also the same flow the dashboard uses before publish.</p>

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

      <h2 id="youtube-note">YouTube note</h2>
      <p>If you are publishing a local video to YouTube, prefer <code>media_ids</code> over trying to inline the file body or rely on an MCP client to move the raw bytes. The dashboard already follows this upload-to-storage-then-publish workflow because it is more reliable for larger assets.</p>
    </DocsPage>
  );
}

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";
import test from "node:test";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const adminPostsPath = path.join(root, "src/app/admin/posts/page.tsx");

test("Admin posts table opens a detail drawer with copyable raw JSON", async () => {
  const source = await readFile(adminPostsPath, "utf8");

  assert.match(source, /const \[selectedPostId,\s*setSelectedPostId\]/);
  assert.match(source, /const \[drawerTab,\s*setDrawerTab\]\s*=\s*useState<"attributes" \| "raw">\("attributes"\)/);
  assert.match(source, /const \[rawCopied,\s*setRawCopied\]/);
  assert.match(source, /const selectedPost = useMemo/);
  assert.match(source, /const selectedPostForDisplay = useMemo/);
  assert.match(source, /function postKey\(post: AdminPostRow,\s*idx: number\)/);
  assert.match(source, /const openPostDetail = useCallback/);
  assert.match(source, /const copyRawPost = useCallback/);
  assert.match(source, /navigator\.clipboard\.writeText\(JSON\.stringify\(selectedPostForDisplay,\s*null,\s*2\)\)/);

  const tableBody = source.slice(source.indexOf("<tbody>"), source.indexOf("</tbody>"));
  assert.match(tableBody, /role="button"/);
  assert.match(tableBody, /tabIndex=\{0\}/);
  assert.match(tableBody, /aria-label=\{`Open post details for \$\{post\.post_id\}`\}/);
  assert.match(tableBody, /onClick=\{\(\) => openPostDetail\(post,\s*idx\)\}/);
  assert.match(tableBody, /onKeyDown=\{\(event\) => handlePostKeyDown\(event,\s*\(\) => openPostDetail\(post,\s*idx\)\)\}/);
  assert.match(tableBody, /aria-pressed=\{selected\}/);

  assert.match(source, /className="posts-detail-drawer"/);
  assert.match(source, /aria-label="Post detail"/);
  assert.match(source, /<DrawerTabs\s+active=\{drawerTab\}/);
  assert.match(source, /aria-label="Copy raw post JSON"/);
  assert.match(source, /<pre style=\{drawerRawJsonStyle\}>\{JSON\.stringify\(selectedPostForDisplay,\s*null,\s*2\)\}<\/pre>/);
  assert.match(source, /function DrawerTabs/);
  assert.match(source, /function FieldChip/);
  assert.match(source, /function handlePostKeyDown/);
  assert.match(source, /const stopLinkClick = useCallback/);
  assert.match(source, /onClick=\{stopLinkClick\}/);
});

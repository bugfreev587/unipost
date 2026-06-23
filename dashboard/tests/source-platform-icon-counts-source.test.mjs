import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const statsPath = path.join(root, "src/components/dashboard/connection-stats.tsx");

test("Source platform stats render icon counts without platform account labels", async () => {
  const source = await readFile(statsPath, "utf8");
  const sourcePlatformCard = source.slice(
    source.indexOf('label="Source platform"'),
    source.indexOf('{profiles.length > 1 &&'),
  );

  assert.match(sourcePlatformCard, /AccountDestinationIcon/);
  assert.match(sourcePlatformCard, /fontWeight: 700/);
  assert.match(sourcePlatformCard, /gap: "8px 30px"/);
  assert.match(sourcePlatformCard, /gap: 10/);
  assert.doesNotMatch(sourcePlatformCard, /quickstartSourceLabel/);
  assert.doesNotMatch(sourcePlatformCard, /account/);
  assert.doesNotMatch(sourcePlatformCard, /channel/);
  assert.doesNotMatch(sourcePlatformCard, /Page/);
});

test("By Profile stat rows keep profile names and counts visually grouped", async () => {
  const source = await readFile(statsPath, "utf8");
  const byProfileCard = source.slice(
    source.indexOf('label="By Profile"'),
    source.indexOf("// ── Managed Users Stats"),
  );

  assert.match(byProfileCard, /gridTemplateColumns: "minmax\(0, 120px\) max-content"/);
  assert.match(byProfileCard, /columnGap: 24/);
  assert.doesNotMatch(byProfileCard, /justifyContent: "space-between"/);
});

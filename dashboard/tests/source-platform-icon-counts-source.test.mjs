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
  assert.doesNotMatch(sourcePlatformCard, /quickstartSourceLabel/);
  assert.doesNotMatch(sourcePlatformCard, /account/);
  assert.doesNotMatch(sourcePlatformCard, /channel/);
  assert.doesNotMatch(sourcePlatformCard, /Page/);
});

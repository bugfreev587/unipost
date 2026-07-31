import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(
  new URL("../src/components/posts/calendar/posts-calendar-view.tsx", import.meta.url),
  "utf8",
);

test("calendar create action cannot accept clicks before React hydration", () => {
  assert.match(source, /const \[createActionReady, setCreateActionReady\] = useState\(false\)/);
  assert.match(source, /useEffect\(\(\) => \{\s*setCreateActionReady\(true\);\s*\}, \[\]\)/);
  assert.match(
    source,
    /className="posts-calendar-create"[\s\S]*?disabled=\{!createActionReady\}[\s\S]*?onClick=\{\(\) => setDrawerOpen\(true\)\}/,
  );
});

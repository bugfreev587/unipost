import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const apiSource = await readFile(new URL("../src/lib/api.ts", import.meta.url), "utf8");
const workspaceLogsSource = await readFile(
  new URL("../src/app/(dashboard)/projects/[id]/logs/page.tsx", import.meta.url),
  "utf8",
);
const adminLogsSource = await readFile(
  new URL("../src/app/admin/logs/page.tsx", import.meta.url),
  "utf8",
);

function boundedSource(source, startMarker, endMarker) {
  const start = source.indexOf(startMarker);
  assert.notEqual(start, -1, `missing start marker: ${startMarker}`);
  const end = source.indexOf(endMarker, start + startMarker.length);
  assert.notEqual(end, -1, `missing end marker after ${startMarker}: ${endMarker}`);
  return source.slice(start, end);
}

test("log API accepts legacy numeric and v2 opaque IDs", () => {
  const logInterface = boundedSource(
    apiSource,
    "export interface IntegrationLog {",
    "export interface IntegrationLogListParams {",
  );
  const workspaceDetailFunction = boundedSource(
    apiSource,
    "export async function getIntegrationLog(",
    "export async function listAdminIntegrationLogs(",
  );
  const adminDetailFunction = boundedSource(
    apiSource,
    "export async function getAdminIntegrationLog(",
    "export async function listAdminSearchHistory(",
  );

  assert.match(logInterface, /^export interface IntegrationLog \{\n  id: number \| string;/);
  assert.ok(
    workspaceDetailFunction.includes(
      'return request(`/v1/logs/${encodeURIComponent(String(id))}`, token);',
    ),
  );
  assert.ok(
    adminDetailFunction.includes(
      'return request(`/v1/admin/logs/${encodeURIComponent(String(id))}`, token);',
    ),
  );
});

test("workspace and admin log drawers select legacy numeric or v2 opaque IDs", () => {
  for (const source of [workspaceLogsSource, adminLogsSource]) {
    assert.ok(
      source.includes(
        "const [selectedLogId, setSelectedLogId] = useState<number | string | null>(null);",
      ),
    );
  }
  const openLogFunction = boundedSource(
    workspaceLogsSource,
    "const openLog = async (id: number | string) => {",
    "const copyRawLog = async () => {",
  );
  assert.ok(openLogFunction.includes("setSelectedLogId(id);"));
  assert.ok(openLogFunction.includes("await getIntegrationLog(token, id);"));
});

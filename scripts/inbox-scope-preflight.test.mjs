import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));

test("Inbox scope preflight is a read-only audit with hashed provider identities", () => {
  const source = readFileSync(join(scriptDirectory, "inbox-scope-preflight.sql"), "utf8");
  const executable = source
    .replace(/--.*$/gm, "")
    .replace(/^\\set.*$/gm, "")
    .trim();

  assert.match(executable, /\bBEGIN\s*;/i);
  assert.match(executable, /SET\s+TRANSACTION\s+READ\s+ONLY\s*;/i);
  assert.match(executable, /MD5\s*\(\s*provider_identity\s*\)/i);
  assert.match(executable, /\bROLLBACK\s*;/i);
  assert.doesNotMatch(executable, /\b(INSERT|UPDATE|DELETE|MERGE|ALTER|DROP|CREATE|TRUNCATE|GRANT|REVOKE|CALL|COPY|DO|VACUUM|ANALYZE|REFRESH)\b/i);
  assert.doesNotMatch(source, /\brepair\b/i);

  const statements = executable.split(";").map((statement) => statement.trim()).filter(Boolean);
  for (const statement of statements) {
    assert.match(
      statement,
      /^(BEGIN|SET\s+TRANSACTION\s+READ\s+ONLY|SELECT\b[\s\S]*|WITH\b[\s\S]*|ROLLBACK)$/i,
      `unexpected preflight statement: ${statement}`,
    );
  }
  assert.equal(statements.at(-1)?.toUpperCase(), "ROLLBACK");
});

#!/usr/bin/env node
import { readFile, writeFile } from "node:fs/promises";
import { applyCandidateToReleasesSource, validateCandidatePayload } from "./lib.mjs";

const releasesPath = process.env.CHANGELOG_RELEASES_PATH || "dashboard/src/app/changelog/releases.ts";

async function loadCandidate() {
  const file = process.env.CHANGELOG_CANDIDATE_FILE;
  if (file) {
    return JSON.parse(await readFile(file, "utf8"));
  }

  const apiBase = process.env.CHANGELOG_API_BASE;
  const token = process.env.CHANGELOG_AUTOMATION_TOKEN;
  const candidateID = process.env.CHANGELOG_CANDIDATE_ID;
  if (!apiBase || !token || !candidateID) {
    throw new Error("Set CHANGELOG_CANDIDATE_FILE or CHANGELOG_API_BASE, CHANGELOG_AUTOMATION_TOKEN, and CHANGELOG_CANDIDATE_ID");
  }
  const res = await fetch(`${apiBase.replace(/\/+$/, "")}/internal/changelog-candidates/${encodeURIComponent(candidateID)}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!res.ok) throw new Error(`candidate fetch failed: HTTP ${res.status}`);
  const body = await res.json();
  return body.data.payload;
}

const payload = await loadCandidate();
validateCandidatePayload(payload);
if (!payload.hasCandidate) {
  throw new Error("cannot apply a no-candidate payload");
}

const source = await readFile(releasesPath, "utf8");
const next = applyCandidateToReleasesSource(source, payload.candidate);
await writeFile(releasesPath, next);
console.log(`applied changelog candidate ${payload.candidate.id}`);

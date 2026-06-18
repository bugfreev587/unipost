#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import { validateCandidatePayload } from "./lib.mjs";

const file = process.argv[2] || process.env.CHANGELOG_CANDIDATE_FILE;
if (!file) {
  console.error("Usage: validate-candidate.mjs <candidate.json>");
  process.exit(2);
}

const payload = JSON.parse(await readFile(file, "utf8"));
validateCandidatePayload(payload);
console.log("candidate valid");

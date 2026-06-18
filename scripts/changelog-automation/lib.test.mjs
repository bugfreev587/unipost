import assert from "node:assert/strict";
import { test } from "node:test";

import {
  applyCandidateToReleasesSource,
  computePreviousLosAngelesWindow,
  normalizeSourceHash,
  renderDiscordCandidateMessage,
  validateCandidatePayload,
} from "./lib.mjs";

const candidatePayload = {
  hasCandidate: true,
  candidate: {
    id: "developer-logs-api",
    date: "2026-06-18",
    displayDate: "June 18, 2026",
    title: "Developer Logs API",
    summary: "Workspace-scoped developer logs are available over REST and SSE.",
    category: "reliability",
    impact: "new",
    isBreaking: false,
    sdkVersions: [],
    links: [{ label: "Logs docs", href: "/docs/api/logs" }],
    sourceLinks: [{ label: "Release PR", href: "https://github.com/bugfreev587/unipost/pull/67" }],
    confidence: "high",
    whyUserVisible: "Developers can inspect delivery and API logs without support help.",
    excludedCommits: [],
  },
};

test("computePreviousLosAngelesWindow handles PDT and PST offsets", () => {
  assert.deepEqual(computePreviousLosAngelesWindow(new Date("2026-06-18T15:00:00.000Z")), {
    localDate: "2026-06-17",
    startISO: "2026-06-17T07:00:00.000Z",
    endISO: "2026-06-18T07:00:00.000Z",
  });

  assert.deepEqual(computePreviousLosAngelesWindow(new Date("2026-12-18T16:00:00.000Z")), {
    localDate: "2026-12-17",
    startISO: "2026-12-17T08:00:00.000Z",
    endISO: "2026-12-18T08:00:00.000Z",
  });
});

test("validateCandidatePayload rejects missing sources and forbidden SDK package names", () => {
  const missingSources = structuredClone(candidatePayload);
  missingSources.candidate.sourceLinks = [];
  assert.throws(() => validateCandidatePayload(missingSources), /sourceLinks/);

  const sdkJS = structuredClone(candidatePayload);
  sdkJS.candidate.category = "sdk";
  sdkJS.candidate.sdkVersions = [{
    ecosystem: "npm",
    packageName: "@unipost/sdk-js",
    version: "0.4.1",
    href: "https://www.npmjs.com/package/@unipost/sdk-js",
  }];
  assert.throws(() => validateCandidatePayload(sdkJS), /@unipost\/sdk/);
});

test("normalizeSourceHash is stable and order independent", () => {
  const first = normalizeSourceHash(["pr:72", "commit:abc", "commit:def"]);
  const second = normalizeSourceHash(["commit:def", "commit:abc", "pr:72"]);
  assert.equal(first, second);
  assert.match(first, /^[a-f0-9]{64}$/);
});

test("renderDiscordCandidateMessage includes markdown action links", () => {
  const message = renderDiscordCandidateMessage(candidatePayload, {
    publish: "https://app.unipost.dev/admin/changelog-actions?action=publish",
    save: "https://app.unipost.dev/admin/changelog-actions?action=save",
    discard: "https://app.unipost.dev/admin/changelog-actions?action=discard",
  });

  assert.match(message, /Developer Logs API/);
  assert.match(message, /\[Publish\]\(https:\/\/app\.unipost\.dev/);
  assert.match(message, /\[Save for later\]\(https:\/\/app\.unipost\.dev/);
  assert.match(message, /\[Discard\]\(https:\/\/app\.unipost\.dev/);
});

test("applyCandidateToReleasesSource inserts candidate once at the top of releases array", () => {
  const source = `export const changelogReleases: ChangelogRelease[] = [
  {
    id: "older-release",
    date: "2026-06-01",
    title: "Older release",
    summary: "Older release.",
    category: "dx",
    impact: "improved",
    isBreaking: false,
    links: [],
    sourceLinks: [],
  },
];`;

  const next = applyCandidateToReleasesSource(source, candidatePayload.candidate);
  assert.ok(next.indexOf('id: "developer-logs-api"') < next.indexOf('id: "older-release"'));
  assert.throws(() => applyCandidateToReleasesSource(next, candidatePayload.candidate), /already exists/);
});

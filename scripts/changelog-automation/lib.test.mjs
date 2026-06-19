import assert from "node:assert/strict";
import { test } from "node:test";

import {
  applyCandidateToReleasesSource,
  candidateSourceHash,
  computePreviousLosAngelesWindow,
  extractAnthropicCandidateContent,
  isDiscordWebhookURL,
  isLosAngelesHour,
  normalizeCandidatePayload,
  normalizeCandidatePayloads,
  normalizeSourceHash,
  parseAIJSONContent,
  renderDiscordCandidateMessage,
  selectReviewPayloads,
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

test("isLosAngelesHour handles PDT and PST schedule guards", () => {
  assert.equal(isLosAngelesHour(new Date("2026-06-18T15:05:00.000Z"), 8), true);
  assert.equal(isLosAngelesHour(new Date("2026-06-18T16:05:00.000Z"), 8), false);
  assert.equal(isLosAngelesHour(new Date("2026-12-18T15:05:00.000Z"), 8), false);
  assert.equal(isLosAngelesHour(new Date("2026-12-18T16:05:00.000Z"), 8), true);
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

test("isDiscordWebhookURL accepts only Discord webhook URLs", () => {
  assert.equal(isDiscordWebhookURL("https://discord.com/api/webhooks/123/token"), true);
  assert.equal(isDiscordWebhookURL("https://discordapp.com/api/webhooks/123/token"), true);
  assert.equal(isDiscordWebhookURL("https://hooks.slack.com/services/T000/B000/xxx"), false);
  assert.equal(isDiscordWebhookURL(""), false);
});

test("extractAnthropicCandidateContent reads text content from messages responses", () => {
  const content = extractAnthropicCandidateContent({
    content: [
      { type: "text", text: "  " },
      { type: "text", text: JSON.stringify(candidatePayload) },
    ],
  });

  assert.equal(content, JSON.stringify(candidatePayload));
});

test("parseAIJSONContent accepts raw and fenced JSON", () => {
  assert.deepEqual(parseAIJSONContent(JSON.stringify(candidatePayload)), candidatePayload);
  assert.deepEqual(parseAIJSONContent(`\`\`\`json\n${JSON.stringify(candidatePayload)}\n\`\`\``), candidatePayload);
});

test("normalizeCandidatePayload coerces links and injects verified commit sources", () => {
  const payload = structuredClone(candidatePayload);
  payload.candidate.links = ["/docs/api/logs"];
  payload.candidate.sourceLinks = ["https://github.com/bugfreev587/made-up-branch"];

  const normalized = normalizeCandidatePayload(payload, {
    repo: "bugfreev587/unipost",
    commits: [{ sha: "abcdef1234567890", subject: "fix: something", author: "UniPost" }],
  });

  assert.deepEqual(normalized.candidate.links, [{ label: "logs", href: "/docs/api/logs" }]);
  assert.deepEqual(normalized.candidate.sourceLinks, [{
    label: "Commit abcdef1",
    href: "https://github.com/bugfreev587/unipost/commit/abcdef1234567890",
  }]);
});

test("normalizeCandidatePayloads keeps source links scoped to each candidate", () => {
  const aiPayload = {
    hasCandidate: true,
    candidates: [
      {
        ...structuredClone(candidatePayload.candidate),
        id: "webhook-validation",
        title: "Webhook validation",
        sourceCommitShas: ["aaaaaaa111111111"],
        sourceLinks: [],
      },
      {
        ...structuredClone(candidatePayload.candidate),
        id: "sdk-release",
        title: "SDK release",
        category: "sdk",
        sourceCommitShas: ["bbbbbbb222222222"],
        sourceLinks: [],
      },
    ],
  };

  const payloads = normalizeCandidatePayloads(aiPayload, {
    repo: "bugfreev587/unipost",
    commits: [
      { sha: "aaaaaaa111111111", subject: "fix: webhook validation", author: "UniPost" },
      { sha: "bbbbbbb222222222", subject: "feat: sdk release", author: "UniPost" },
    ],
  });

  assert.equal(payloads.length, 2);
  assert.deepEqual(payloads[0].candidate.sourceLinks, [{
    label: "Commit aaaaaaa",
    href: "https://github.com/bugfreev587/unipost/commit/aaaaaaa111111111",
  }]);
  assert.deepEqual(payloads[1].candidate.sourceLinks, [{
    label: "Commit bbbbbbb",
    href: "https://github.com/bugfreev587/unipost/commit/bbbbbbb222222222",
  }]);
  assert.notEqual(candidateSourceHash(payloads[0]), candidateSourceHash(payloads[1]));
});

test("selectReviewPayloads caps daily review to two highest-impact candidates", () => {
  const low = structuredClone(candidatePayload);
  low.candidate.id = "docs-copy";
  low.candidate.title = "Docs copy cleanup";
  low.candidate.category = "dx";
  low.candidate.impact = "fixed";

  const medium = structuredClone(candidatePayload);
  medium.candidate.id = "dashboard-flow";
  medium.candidate.title = "Dashboard flow";
  medium.candidate.category = "dashboard";
  medium.candidate.impact = "improved";

  const high = structuredClone(candidatePayload);
  high.candidate.id = "api-release";
  high.candidate.title = "API release";
  high.candidate.category = "api";
  high.candidate.impact = "new";

  const selected = selectReviewPayloads([
    { payload: low, status: "new" },
    { payload: medium, status: "new" },
    { payload: high, status: "new" },
  ], { limit: 2 });

  assert.deepEqual(selected.map((entry) => entry.payload.candidate.id), ["api-release", "dashboard-flow"]);
});

test("selectReviewPayloads gives saved candidates another review slot", () => {
  const saved = structuredClone(candidatePayload);
  saved.candidate.id = "saved-webhook";
  saved.candidate.title = "Saved webhook";
  saved.candidate.category = "dx";
  saved.candidate.impact = "fixed";

  const high = structuredClone(candidatePayload);
  high.candidate.id = "api-release";
  high.candidate.title = "API release";
  high.candidate.category = "api";
  high.candidate.impact = "new";

  const medium = structuredClone(candidatePayload);
  medium.candidate.id = "dashboard-flow";
  medium.candidate.title = "Dashboard flow";
  medium.candidate.category = "dashboard";
  medium.candidate.impact = "improved";

  const selected = selectReviewPayloads([
    { payload: high, status: "new" },
    { payload: medium, status: "new" },
    { payload: saved, status: "saved" },
  ], { limit: 2 });

  assert.deepEqual(selected.map((entry) => entry.payload.candidate.id), ["saved-webhook", "api-release"]);
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

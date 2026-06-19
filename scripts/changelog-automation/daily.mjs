#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import {
  candidateSourceHash,
  computePreviousLosAngelesWindow,
  extractAnthropicCandidateContent,
  isDiscordWebhookURL,
  isLosAngelesHour,
  normalizeCandidatePayloads,
  normalizeSourceHash,
  parseAIJSONContent,
  renderDiscordCandidateMessage,
  selectReviewPayloads,
  validateCandidatePayload,
} from "./lib.mjs";

const outDir = process.env.CHANGELOG_OUT_DIR || "artifacts/changelog";
const reviewLimit = reviewCandidateLimit();
await mkdir(outDir, { recursive: true });

if (process.env.CHANGELOG_REQUIRE_LA_HOUR) {
  const requiredHour = Number(process.env.CHANGELOG_REQUIRE_LA_HOUR);
  if (!Number.isInteger(requiredHour) || requiredHour < 0 || requiredHour > 23) {
    throw new Error("CHANGELOG_REQUIRE_LA_HOUR must be an integer from 0 to 23");
  }
  if (!isLosAngelesHour(new Date(), requiredHour)) {
    console.log(`Skipping scheduled changelog run because it is not ${requiredHour}:00 in Los Angeles.`);
    process.exit(0);
  }
}

if (process.env.CHANGELOG_WEBHOOK_TEST === "true") {
  await sendDiscord("UniPost changelog automation webhook test.");
  console.log("Changelog Discord webhook test sent.");
  process.exit(0);
}

const windowInfo = process.env.CHANGELOG_WINDOW_START && process.env.CHANGELOG_WINDOW_END
  ? {
      localDate: process.env.CHANGELOG_LOCAL_DATE || process.env.CHANGELOG_WINDOW_START.slice(0, 10),
      startISO: process.env.CHANGELOG_WINDOW_START,
      endISO: process.env.CHANGELOG_WINDOW_END,
    }
  : computePreviousLosAngelesWindow(new Date());

const commits = collectCommits(windowInfo.startISO, windowInfo.endISO);
const sourceHash = normalizeSourceHash(commits.map((commit) => `commit:${commit.sha}`));
const draftPayload = await draftCandidate({ commits, windowInfo });
const draftedPayloads = normalizeCandidatePayloads(draftPayload, {
  commits,
  repo: process.env.CHANGELOG_REPO,
});
const savedEntries = await listSavedCandidates(reviewLimit);
const selectedEntries = selectReviewPayloads([
  ...savedEntries,
  ...draftedPayloads.map((payload) => ({ payload, status: "new" })),
], { limit: reviewLimit });

const firstSelectedPayload = selectedEntries[0]?.payload || draftPayload;

await writeFile(`${outDir}/candidate.json`, `${JSON.stringify(firstSelectedPayload, null, 2)}\n`);
await writeFile(`${outDir}/candidates.json`, `${JSON.stringify({
  selected: selectedEntries.map(({ payload, status }) => ({ status, payload })),
  drafted: draftedPayloads,
  saved: savedEntries.map(({ payload, status }) => ({ status, payload })),
}, null, 2)}\n`);
await writeFile(`${outDir}/source.json`, `${JSON.stringify({ window: windowInfo, commits, sourceHash }, null, 2)}\n`);

if (selectedEntries.length === 0) {
  console.log(`No changelog candidate: ${draftPayload.reason || "no reason provided"}`);
  if (process.env.CHANGELOG_DISCORD_WEBHOOK_URL && process.env.CHANGELOG_NOTIFY_EMPTY === "true") {
    await sendDiscord(renderDiscordCandidateMessage(draftPayload, {}));
  }
  process.exit(0);
}

let promptedCount = 0;
for (const entry of selectedEntries) {
  validateCandidatePayload(entry.payload);
  if (entry.status === "saved") {
    if (process.env.CHANGELOG_DISCORD_WEBHOOK_URL) {
      await sendDiscord(renderDiscordCandidateMessage(entry.payload, entry.actions));
    }
    promptedCount += 1;
    console.log(`Requeued saved changelog candidate ${entry.payload.candidate.id}`);
    continue;
  }
  const createResponse = await createCandidate(entry.payload, candidateSourceHash(entry.payload), windowInfo);
  if (createResponse.data?.candidate?.status && !["pending", "saved", "failed"].includes(createResponse.data.candidate.status)) {
    console.log(`Candidate ${entry.payload.candidate.id} already ${createResponse.data.candidate.status}; skipping Discord prompt.`);
    continue;
  }
  const actions = createResponse.data.actions;
  const messagePayload = createResponse.data?.candidate?.payload || entry.payload;
  if (process.env.CHANGELOG_DISCORD_WEBHOOK_URL) {
    await sendDiscord(renderDiscordCandidateMessage(messagePayload, actions));
  }
  promptedCount += 1;
  console.log(`Created changelog candidate ${messagePayload.candidate.id}`);
}
console.log(`Prompted ${promptedCount} changelog candidate${promptedCount === 1 ? "" : "s"}`);

function collectCommits(startISO, endISO) {
  const raw = execFileSync("git", [
    "log",
    `--since=${startISO}`,
    `--until=${endISO}`,
    "--pretty=format:%H%x09%s%x09%an",
  ], { encoding: "utf8" }).trim();
  if (!raw) return [];
  return raw.split("\n").map((line) => {
    const [sha, subject, author] = line.split("\t");
    return { sha, subject, author };
  });
}

async function draftCandidate({ commits, windowInfo }) {
  if (commits.length === 0) {
    return {
      hasCandidate: false,
      reason: `No commits found for ${windowInfo.localDate}.`,
      excludedCommits: [],
    };
  }
  const apiKey = process.env.CHANGELOG_AI_API_KEY || process.env.OPENAI_API_KEY;
  const anthropicKey = process.env.CHANGELOG_ANTHROPIC_API_KEY || process.env.ANTHROPIC_API_KEY;
  if (!apiKey && !anthropicKey) {
    return {
      hasCandidate: false,
      reason: "No changelog AI key is configured; no AI candidate was generated.",
      excludedCommits: commits.map((commit) => commit.sha),
    };
  }
  const model = process.env.CHANGELOG_AI_MODEL || "gpt-4.1-mini";
  const messages = [
    {
      role: "system",
      content: [
        "You write sparse, factual UniPost public changelog candidates.",
        "Return strict JSON only with hasCandidate and candidates fields.",
        "Return at most five candidates; the workflow will review at most two.",
        "Group related commits into one candidate. Do not create one candidate per commit.",
        "Include sourceCommitShas on each candidate with only the commit SHAs that support that candidate.",
        "Only include user-visible shipped work. Do not invent SDK versions.",
        "Never use @unipost/sdk-js. The JavaScript package is @unipost/sdk.",
        "Do not treat @unipost/agentpost as an SDK.",
      ].join(" "),
    },
    {
      role: "user",
      content: JSON.stringify({
        date: windowInfo.localDate,
        commits,
        schema: {
          hasCandidate: true,
          candidates: [
            {
              id: "stable-kebab-id",
              date: windowInfo.localDate,
              displayDate: "Month D, YYYY",
              title: "Release title",
              summary: "One factual sentence.",
              category: "api|sdk|dashboard|platform|dx|reliability",
              impact: "new|improved|changed|fixed",
              isBreaking: false,
              sdkVersions: [],
              links: [],
              sourceCommitShas: [],
              sourceLinks: [],
              confidence: "low|medium|high",
              whyUserVisible: "Concrete user-visible reason.",
              excludedCommits: [],
            },
          ],
        },
      }),
    },
  ];
  if (anthropicKey && !apiKey) {
    const res = await fetch(process.env.CHANGELOG_ANTHROPIC_URL || "https://api.anthropic.com/v1/messages", {
      method: "POST",
      headers: {
        "x-api-key": anthropicKey,
        "anthropic-version": "2023-06-01",
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model: process.env.CHANGELOG_ANTHROPIC_MODEL || "claude-haiku-4-5-20251001",
        max_tokens: 1600,
        temperature: 0.2,
        system: messages[0].content,
        messages: messages.slice(1),
      }),
    });
    if (!res.ok) throw new Error(`Anthropic request failed: HTTP ${res.status}`);
    return parseAIJSONContent(extractAnthropicCandidateContent(await res.json()));
  }
  const res = await fetch(process.env.CHANGELOG_AI_URL || "https://api.openai.com/v1/chat/completions", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${apiKey}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      model,
      temperature: 0.2,
      response_format: { type: "json_object" },
      messages,
    }),
  });
  if (!res.ok) throw new Error(`AI request failed: HTTP ${res.status}`);
  const body = await res.json();
  const content = body.choices?.[0]?.message?.content;
  if (!content) throw new Error("AI response did not include message content");
  return parseAIJSONContent(content);
}

async function listSavedCandidates(limit) {
  const apiBase = process.env.CHANGELOG_API_BASE;
  const token = process.env.CHANGELOG_AUTOMATION_TOKEN;
  if (!apiBase || !token) return [];
  const url = new URL(`${apiBase.replace(/\/+$/, "")}/internal/changelog-candidates`);
  url.searchParams.set("status", "saved");
  url.searchParams.set("limit", String(limit));
  const res = await fetch(url, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  if (!res.ok) throw new Error(`saved candidate list failed: HTTP ${res.status} ${await res.text()}`);
  const body = await res.json();
  return (body.data?.candidates || [])
    .filter((item) => item?.candidate?.payload?.hasCandidate)
    .map((item) => ({
      payload: item.candidate.payload,
      status: "saved",
      actions: item.actions,
      record: item.candidate,
    }));
}

async function createCandidate(payload, sourceHash, windowInfo) {
  const apiBase = process.env.CHANGELOG_API_BASE;
  const token = process.env.CHANGELOG_AUTOMATION_TOKEN;
  if (!apiBase || !token) {
    throw new Error("CHANGELOG_API_BASE and CHANGELOG_AUTOMATION_TOKEN are required when a candidate exists");
  }
  const res = await fetch(`${apiBase.replace(/\/+$/, "")}/internal/changelog-candidates`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      payload,
      source_hash: sourceHash,
      window_start: windowInfo.startISO,
      window_end: windowInfo.endISO,
    }),
  });
  if (!res.ok) throw new Error(`candidate create failed: HTTP ${res.status} ${await res.text()}`);
  return res.json();
}

function reviewCandidateLimit() {
  const raw = process.env.CHANGELOG_DAILY_REVIEW_LIMIT || "2";
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error("CHANGELOG_DAILY_REVIEW_LIMIT must be a positive integer");
  }
  return Math.min(parsed, 2);
}

async function sendDiscord(content) {
  if (!isDiscordWebhookURL(process.env.CHANGELOG_DISCORD_WEBHOOK_URL)) {
    throw new Error("CHANGELOG_DISCORD_WEBHOOK_URL must be a Discord webhook URL");
  }
  const res = await fetch(process.env.CHANGELOG_DISCORD_WEBHOOK_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      content,
      allowed_mentions: { parse: [] },
    }),
  });
  if (!res.ok) throw new Error(`Discord webhook failed: HTTP ${res.status}`);
}

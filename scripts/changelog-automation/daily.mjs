#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { mkdir, writeFile } from "node:fs/promises";
import {
  computePreviousLosAngelesWindow,
  extractAnthropicCandidateContent,
  isDiscordWebhookURL,
  normalizeSourceHash,
  renderDiscordCandidateMessage,
  validateCandidatePayload,
} from "./lib.mjs";

const outDir = process.env.CHANGELOG_OUT_DIR || "artifacts/changelog";
await mkdir(outDir, { recursive: true });

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
const payload = await draftCandidate({ commits, windowInfo });

await writeFile(`${outDir}/candidate.json`, `${JSON.stringify(payload, null, 2)}\n`);
await writeFile(`${outDir}/source.json`, `${JSON.stringify({ window: windowInfo, commits, sourceHash }, null, 2)}\n`);

if (!payload.hasCandidate) {
  console.log(`No changelog candidate: ${payload.reason || "no reason provided"}`);
  if (process.env.CHANGELOG_DISCORD_WEBHOOK_URL && process.env.CHANGELOG_NOTIFY_EMPTY === "true") {
    await sendDiscord(renderDiscordCandidateMessage(payload, {}));
  }
  process.exit(0);
}

validateCandidatePayload(payload);
const createResponse = await createCandidate(payload, sourceHash, windowInfo);
if (createResponse.data?.candidate?.status && !["pending", "saved", "failed"].includes(createResponse.data.candidate.status)) {
  console.log(`Candidate already ${createResponse.data.candidate.status}; skipping Discord prompt.`);
  process.exit(0);
}

const actions = createResponse.data.actions;
if (process.env.CHANGELOG_DISCORD_WEBHOOK_URL) {
  await sendDiscord(renderDiscordCandidateMessage(payload, actions));
}
console.log(`Created changelog candidate ${payload.candidate.id}`);

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
        "Return strict JSON only with hasCandidate and candidate fields.",
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
          candidate: {
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
            sourceLinks: [],
            confidence: "low|medium|high",
            whyUserVisible: "Concrete user-visible reason.",
            excludedCommits: [],
          },
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
        model: process.env.CHANGELOG_ANTHROPIC_MODEL || "claude-3-5-haiku-latest",
        max_tokens: 1600,
        temperature: 0.2,
        system: messages[0].content,
        messages: messages.slice(1),
      }),
    });
    if (!res.ok) throw new Error(`Anthropic request failed: HTTP ${res.status}`);
    return JSON.parse(extractAnthropicCandidateContent(await res.json()));
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
  return JSON.parse(content);
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

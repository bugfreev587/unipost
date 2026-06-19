import { createHash } from "node:crypto";

const LA_TIME_ZONE = "America/Los_Angeles";
const VALID_CATEGORIES = new Set(["api", "sdk", "dashboard", "platform", "dx", "reliability"]);
const VALID_IMPACTS = new Set(["new", "improved", "changed", "fixed"]);
const VALID_ECOSYSTEMS = new Set(["npm", "pip", "go", "maven"]);

function localParts(date, timeZone = LA_TIME_ZONE) {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).formatToParts(date);
  const out = {};
  for (const part of parts) {
    if (part.type !== "literal") out[part.type] = Number(part.value);
  }
  return {
    year: out.year,
    month: out.month,
    day: out.day,
    hour: out.hour,
    minute: out.minute,
    second: out.second,
  };
}

function zonedTimeToUTC({ year, month, day, hour = 0, minute = 0, second = 0 }, timeZone = LA_TIME_ZONE) {
  const target = Date.UTC(year, month - 1, day, hour, minute, second);
  let guess = target;
  for (let i = 0; i < 4; i += 1) {
    const parts = localParts(new Date(guess), timeZone);
    const actual = Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute, parts.second);
    const diff = actual - target;
    if (diff === 0) break;
    guess -= diff;
  }
  return new Date(guess);
}

function isoDate({ year, month, day }) {
  return `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}

function addLocalDays(parts, days) {
  const utc = new Date(Date.UTC(parts.year, parts.month - 1, parts.day + days, 12, 0, 0));
  return {
    year: utc.getUTCFullYear(),
    month: utc.getUTCMonth() + 1,
    day: utc.getUTCDate(),
  };
}

export function computePreviousLosAngelesWindow(now = new Date()) {
  const today = localParts(now, LA_TIME_ZONE);
  const previous = addLocalDays(today, -1);
  const next = addLocalDays(previous, 1);
  const start = zonedTimeToUTC({ ...previous, hour: 0, minute: 0, second: 0 }, LA_TIME_ZONE);
  const end = zonedTimeToUTC({ ...next, hour: 0, minute: 0, second: 0 }, LA_TIME_ZONE);
  return {
    localDate: isoDate(previous),
    startISO: start.toISOString(),
    endISO: end.toISOString(),
  };
}

export function isLosAngelesHour(now = new Date(), expectedHour) {
  return localParts(now, LA_TIME_ZONE).hour === Number(expectedHour);
}

export function normalizeSourceHash(parts) {
  const cleaned = [...new Set((parts || []).map((part) => String(part || "").trim()).filter(Boolean))].sort();
  return createHash("sha256").update(cleaned.join("\n")).digest("hex");
}

export function validateCandidatePayload(payload) {
  if (!payload || typeof payload !== "object") throw new Error("candidate payload is required");
  if (!payload.hasCandidate) return payload;
  const candidate = payload.candidate;
  if (!candidate || typeof candidate !== "object") throw new Error("candidate is required");
  for (const key of ["id", "date", "title", "summary", "whyUserVisible"]) {
    if (!String(candidate[key] || "").trim()) throw new Error(`${key} is required`);
  }
  if (!/^\d{4}-\d{2}-\d{2}$/.test(candidate.date)) throw new Error("date must be YYYY-MM-DD");
  if (!VALID_CATEGORIES.has(candidate.category)) throw new Error(`unsupported category: ${candidate.category}`);
  if (!VALID_IMPACTS.has(candidate.impact)) throw new Error(`unsupported impact: ${candidate.impact}`);
  if (!Array.isArray(candidate.sourceLinks) || candidate.sourceLinks.length === 0) {
    throw new Error("sourceLinks are required");
  }
  for (const link of [...(candidate.links || []), ...(candidate.sourceLinks || [])]) {
    validateLink(link);
  }
  for (const sdk of candidate.sdkVersions || []) {
    validateSDKVersion(sdk);
  }
  return payload;
}

export function isDiscordWebhookURL(value) {
  const href = String(value || "").trim();
  if (!href) return false;
  try {
    const url = new URL(href);
    return (url.hostname === "discord.com" || url.hostname === "discordapp.com") &&
      url.pathname.startsWith("/api/webhooks/");
  } catch {
    return false;
  }
}

export function extractAnthropicCandidateContent(body) {
  const text = (body?.content || [])
    .filter((part) => part?.type === "text" && String(part.text || "").trim())
    .map((part) => String(part.text).trim())
    .join("\n")
    .trim();
  if (!text) throw new Error("Anthropic response did not include text content");
  return text;
}

export function parseAIJSONContent(content) {
  const text = String(content || "").trim();
  const fenced = text.match(/^```(?:json)?\s*([\s\S]*?)\s*```$/i);
  return JSON.parse(fenced ? fenced[1].trim() : text);
}

export function normalizeCandidatePayload(payload, { commits = [], repo = "" } = {}) {
  const payloads = normalizeCandidatePayloads(payload, { commits, repo });
  if (payloads.length === 0) return payload;
  return payloads[0];
}

export function normalizeCandidatePayloads(payload, { commits = [], repo = "" } = {}) {
  if (!payload?.hasCandidate) return [];
  const rawCandidates = Array.isArray(payload.candidates)
    ? payload.candidates
    : payload.candidate
      ? [payload.candidate]
      : [];
  return rawCandidates
    .filter((candidate) => candidate && typeof candidate === "object")
    .map((candidate) => normalizedPayloadForCandidate(payload, candidate, {
      commits,
      repo,
      useAllCommitFallback: rawCandidates.length === 1,
    }));
}

function normalizedPayloadForCandidate(payload, rawCandidate, { commits, repo, useAllCommitFallback }) {
  const { candidate: _candidate, candidates: _candidates, ...rest } = payload;
  const scopedCommitLinks = verifiedCommitLinksForCandidate(rawCandidate, commits, repo);
  const fallbackSourceLinks = useAllCommitFallback
    ? verifiedCommitLinks(commits, repo)
    : null;
  const normalizedSourceLinks = scopedCommitLinks ||
    fallbackSourceLinks ||
    normalizeLinks(rawCandidate.sourceLinks) ||
    [];
  const candidate = {
    ...rawCandidate,
    links: normalizeLinks(rawCandidate.links),
    sourceLinks: normalizedSourceLinks,
  };
  return { ...rest, hasCandidate: true, candidate };
}

export function candidateSourceHash(payload) {
  if (!payload?.hasCandidate || !payload.candidate) return normalizeSourceHash([]);
  const candidate = payload.candidate;
  const sourceParts = (candidate.sourceLinks || [])
    .map((link) => `source:${String(link.href || "").trim()}`)
    .filter((part) => part !== "source:");
  const sdkParts = (candidate.sdkVersions || [])
    .map((sdk) => `sdk:${sdk.ecosystem}:${sdk.packageName}:${sdk.version}`)
    .filter(Boolean);
  return normalizeSourceHash([...sourceParts, ...sdkParts]);
}

export function selectReviewPayloads(entries, { limit = 2 } = {}) {
  const max = Number.isInteger(limit) && limit > 0 ? limit : 2;
  return (entries || [])
    .map((entry, index) => ({ ...entry, index, score: reviewScore(entry) }))
    .filter((entry) => entry.payload?.hasCandidate && entry.payload.candidate)
    .sort((a, b) => b.score - a.score || a.index - b.index)
    .slice(0, max)
    .map(({ index: _index, score: _score, ...entry }) => entry);
}

function reviewScore(entry) {
  const candidate = entry?.payload?.candidate || {};
  const savedBoost = entry?.status === "saved" ? 10000 : 0;
  const breakingBoost = candidate.isBreaking ? 1000 : 0;
  const impactBoost = {
    new: 400,
    improved: 300,
    changed: 200,
    fixed: 100,
  }[candidate.impact] || 0;
  const categoryBoost = {
    sdk: 80,
    api: 70,
    reliability: 60,
    platform: 50,
    dashboard: 40,
    dx: 30,
  }[candidate.category] || 0;
  const sdkBoost = candidate.sdkVersions?.length ? 60 : 0;
  const confidenceBoost = {
    high: 30,
    medium: 20,
    low: 10,
  }[candidate.confidence] || 0;
  const sourceBoost = Math.min(candidate.sourceLinks?.length || 0, 5);
  return savedBoost + breakingBoost + impactBoost + categoryBoost + sdkBoost + confidenceBoost + sourceBoost;
}

function normalizeLinks(links) {
  return (links || [])
    .map(coerceLink)
    .filter(Boolean);
}

function coerceLink(link) {
  if (typeof link === "string") {
    const href = link.trim();
    if (!href) return null;
    return { label: labelForHref(href), href };
  }
  if (!link || typeof link !== "object") return null;
  const label = String(link.label || "").trim();
  const href = String(link.href || "").trim();
  if (!label || !href) return null;
  return { ...link, label, href };
}

function verifiedCommitLinks(commits, repo) {
  const ownerRepo = String(repo || "").trim();
  if (!ownerRepo || !Array.isArray(commits) || commits.length === 0) return null;
  return commits
    .map((commit) => String(commit?.sha || "").trim())
    .filter(Boolean)
    .map((sha) => ({
      label: `Commit ${sha.slice(0, 7)}`,
      href: `https://github.com/${ownerRepo}/commit/${sha}`,
    }));
}

function verifiedCommitLinksForCandidate(candidate, commits, repo) {
  const sourceShas = Array.isArray(candidate?.sourceCommitShas)
    ? candidate.sourceCommitShas
    : Array.isArray(candidate?.sourceCommits)
      ? candidate.sourceCommits
      : [];
  const wanted = sourceShas
    .map((sha) => String(sha || "").trim())
    .filter(Boolean);
  if (wanted.length === 0) return null;
  const scopedCommits = (commits || []).filter((commit) => {
    const sha = String(commit?.sha || "").trim();
    return sha && wanted.some((wantedSha) => sha === wantedSha || sha.startsWith(wantedSha) || wantedSha.startsWith(sha));
  });
  if (scopedCommits.length === 0) return null;
  return verifiedCommitLinks(scopedCommits, repo);
}

function labelForHref(href) {
  const clean = href.replace(/[#?].*$/, "").replace(/\/+$/, "");
  const segment = clean.split("/").filter(Boolean).pop();
  return segment || href;
}

function validateLink(link) {
  if (!link || !String(link.label || "").trim() || !String(link.href || "").trim()) {
    throw new Error("links require label and href");
  }
  const href = String(link.href).trim();
  if (href.startsWith("/")) return;
  const url = new URL(href);
  if (!["https:", "http:"].includes(url.protocol)) throw new Error(`unsupported link protocol: ${href}`);
}

function validateSDKVersion(sdk) {
  if (!VALID_ECOSYSTEMS.has(sdk.ecosystem)) throw new Error(`unsupported SDK ecosystem: ${sdk.ecosystem}`);
  if (sdk.packageName === "@unipost/sdk-js") {
    throw new Error("JavaScript SDK package is @unipost/sdk, not @unipost/sdk-js");
  }
  if (sdk.packageName === "@unipost/agentpost") {
    throw new Error("AgentPost is not an SDK release");
  }
  for (const key of ["packageName", "version", "href"]) {
    if (!String(sdk[key] || "").trim()) throw new Error(`sdkVersions.${key} is required`);
  }
  validateLink({ label: sdk.packageName, href: sdk.href });
}

export function renderDiscordCandidateMessage(payload, actions) {
  validateCandidatePayload(payload);
  if (!payload.hasCandidate) {
    return `UniPost changelog candidate\n\nNo candidate: ${payload.reason || "No changelog-worthy shipped work."}`;
  }
  const c = payload.candidate;
  const sources = c.sourceLinks.map((link) => `- [${link.label}](${link.href})`).join("\n");
  const sdkVersions = (c.sdkVersions || [])
    .map((sdk) => `- ${sdk.ecosystem}: ${sdk.packageName} ${sdk.version}`)
    .join("\n");
  return [
    `UniPost changelog candidate for ${c.date}`,
    "",
    `Candidate: ${c.title}`,
    `Area: ${c.category}`,
    `Impact: ${c.impact}`,
    `Confidence: ${c.confidence || "unknown"}`,
    "",
    "Summary:",
    c.summary,
    "",
    "Why user-visible:",
    c.whyUserVisible,
    sdkVersions ? ["", "SDK versions:", sdkVersions].join("\n") : "",
    "",
    "Verified sources:",
    sources,
    "",
    "Actions:",
    `[Publish](${actions.publish}) | [Save for later](${actions.save}) | [Discard](${actions.discard})`,
  ].filter((line) => line !== "").join("\n");
}

export function applyCandidateToReleasesSource(source, candidate) {
  if (!candidate?.id) throw new Error("candidate id is required");
  if (source.includes(`id: "${candidate.id}"`) || source.includes(`id: '${candidate.id}'`)) {
    throw new Error(`release ${candidate.id} already exists`);
  }
  const marker = "export const changelogReleases: ChangelogRelease[] = [";
  const index = source.indexOf(marker);
  if (index === -1) throw new Error("changelogReleases marker not found");
  const insertAt = index + marker.length;
  return `${source.slice(0, insertAt)}\n${renderReleaseObject(candidate, "  ")},${source.slice(insertAt)}`;
}

function renderReleaseObject(candidate, indent = "") {
  const lines = [];
  const push = (line) => lines.push(`${indent}${line}`);
  push("{");
  push(`  id: ${quote(candidate.id)},`);
  push(`  date: ${quote(candidate.date)},`);
  if (candidate.displayDate) push(`  displayDate: ${quote(candidate.displayDate)},`);
  push(`  title: ${quote(candidate.title)},`);
  push(`  summary: ${quote(candidate.summary)},`);
  push(`  category: ${quote(candidate.category)},`);
  push(`  impact: ${quote(candidate.impact)},`);
  push(`  isBreaking: ${candidate.isBreaking ? "true" : "false"},`);
  if (candidate.sdkVersions?.length) {
    push("  sdkVersions: [");
    for (const sdk of candidate.sdkVersions) {
      push("    {");
      push(`      ecosystem: ${quote(sdk.ecosystem)},`);
      push(`      packageName: ${quote(sdk.packageName)},`);
      push(`      version: ${quote(sdk.version)},`);
      push(`      href: ${quote(sdk.href)},`);
      if (sdk.installCommand) push(`      installCommand: ${quote(sdk.installCommand)},`);
      push("    },");
    }
    push("  ],");
  }
  push(`  links: ${renderLinks(candidate.links || [])},`);
  push(`  sourceLinks: ${renderLinks(candidate.sourceLinks || [])},`);
  push("}");
  return lines.join("\n");
}

function renderLinks(links) {
  if (!links.length) return "[]";
  return `[\n${links.map((link) => `    { label: ${quote(link.label)}, href: ${quote(link.href)} }`).join(",\n")},\n  ]`;
}

function quote(value) {
  return JSON.stringify(String(value));
}

export function registryURLForSDK(sdk) {
  switch (sdk.ecosystem) {
    case "npm":
      return `https://registry.npmjs.org/${encodeURIComponent(sdk.packageName).replace(/^%40/, "@")}/${sdk.version}`;
    case "pip":
      return `https://pypi.org/pypi/${encodeURIComponent(sdk.packageName)}/${sdk.version}/json`;
    case "go":
      return `https://proxy.golang.org/${sdk.packageName.toLowerCase()}/@v/v${sdk.version}.info`;
    case "maven": {
      const [group, artifact] = sdk.packageName.split(":");
      return `https://repo1.maven.org/maven2/${group.replaceAll(".", "/")}/${artifact}/${sdk.version}/${artifact}-${sdk.version}.pom`;
    }
    default:
      throw new Error(`unsupported SDK ecosystem: ${sdk.ecosystem}`);
  }
}

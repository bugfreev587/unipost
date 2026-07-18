#!/usr/bin/env node

import { appendFile, mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { pathToFileURL } from "node:url";

const hostPattern =
  /^unipost-dev-pr-\d+-\d+-\d+\.vercel\.app$/;

export function extractShareableURL(payload, expectedHost) {
  const candidate =
    typeof payload === "string"
      ? payload
      : payload?.protectionBypassUrl ??
        payload?.shareableUrl ??
        payload?.url ??
        payload?.value;

  let url;
  try {
    url = new URL(candidate);
  } catch {
    if (
      typeof candidate !== "string" ||
      !/^[A-Za-z0-9._~-]{16,512}$/.test(candidate)
    ) {
      throw new Error("Vercel did not return a valid Vercel shareable URL");
    }
    url = new URL(`https://${expectedHost}/`);
    url.searchParams.set("_vercel_share", candidate);
  }

  const isAliasShare =
    url.protocol === "https:" &&
    url.hostname === expectedHost &&
    Boolean(url.searchParams.get("_vercel_share"));
  const isShortShare =
    url.protocol === "https:" &&
    url.hostname === "vercel.sh" &&
    url.pathname.startsWith("/s/");

  if (!isAliasShare && !isShortShare) {
    throw new Error("Vercel did not return a valid Vercel shareable URL");
  }

  return url.toString();
}

async function responseError(response) {
  let detail = "";
  try {
    const payload = await response.json();
    detail = payload?.error?.message ?? payload?.error?.code ?? "";
  } catch {
    // The HTTP status remains sufficient failure evidence.
  }
  return `Vercel shareable-link request failed: ${response.status} ${response.statusText}${
    detail ? ` (${detail})` : ""
  }`;
}

export async function createShareableLink({
  host,
  teamId,
  token,
  ttlSeconds = 86_400,
  fetchImpl = fetch,
}) {
  if (!hostPattern.test(host)) {
    throw new Error(`Invalid isolated Preview host: ${host}`);
  }
  if (!teamId || !token) {
    throw new Error("VERCEL_ORG_ID and VERCEL_TOKEN are required");
  }

  const endpoint = new URL(
    `https://api.vercel.com/aliases/${encodeURIComponent(host)}/protection-bypass`,
  );
  endpoint.searchParams.set("teamId", teamId);

  const response = await fetchImpl(endpoint, {
    method: "PATCH",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ ttl: ttlSeconds }),
  });
  if (!response.ok) {
    throw new Error(await responseError(response));
  }

  return extractShareableURL(await response.json(), host);
}

async function main() {
  const host = process.env.PREVIEW_HOST;
  const outputFile = process.env.GITHUB_OUTPUT;
  const evidenceFile =
    process.env.VERCEL_SHARE_EVIDENCE ??
    "artifacts/preview/vercel-share-link.json";

  if (!outputFile) {
    throw new Error("GITHUB_OUTPUT is required");
  }

  const shareableURL = await createShareableLink({
    host,
    teamId: process.env.VERCEL_ORG_ID,
    token: process.env.VERCEL_TOKEN,
  });

  console.log(`::add-mask::${shareableURL}`);
  await appendFile(outputFile, `shareable_url=${shareableURL}\n`);
  await mkdir(dirname(resolve(evidenceFile)), { recursive: true });
  await writeFile(
    evidenceFile,
    `${JSON.stringify(
      {
        host,
        ttlSeconds: 86_400,
        createdAt: new Date().toISOString(),
      },
      null,
      2,
    )}\n`,
  );
  console.log(`Created a temporary Vercel shareable link for ${host}`);
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}

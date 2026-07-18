#!/usr/bin/env node

import { pathToFileURL } from "node:url";

export function selectPreviewAliases(aliases, pullRequestNumber) {
  const legacyAlias = `unipost-dev-pr-${pullRequestNumber}.vercel.app`;
  const isolatedAliasPattern = new RegExp(
    `^unipost-dev-pr-${pullRequestNumber}-\\d+-\\d+\\.vercel\\.app$`,
  );
  return aliases
    .map((item) => item?.alias)
    .filter(
      (alias) =>
        typeof alias === "string" &&
        (alias === legacyAlias || isolatedAliasPattern.test(alias)),
    );
}

async function checkedJSON(response, action) {
  if (!response.ok) {
    let detail = "";
    try {
      const payload = await response.json();
      detail = payload?.error?.message ?? payload?.error?.code ?? "";
    } catch {
      // The HTTP status remains sufficient failure evidence.
    }
    throw new Error(
      `${action} failed: ${response.status} ${response.statusText}${
        detail ? ` (${detail})` : ""
      }`,
    );
  }
  return response.json();
}

export async function cleanupPreviewAliases({
  pullRequestNumber,
  projectId,
  teamId,
  token,
  fetchImpl = fetch,
}) {
  if (!/^\d+$/.test(pullRequestNumber ?? "")) {
    throw new Error("PR_NUMBER must be numeric");
  }
  if (!projectId || !teamId || !token) {
    throw new Error(
      "VERCEL_PROJECT_ID, VERCEL_ORG_ID, and VERCEL_TOKEN are required",
    );
  }

  const aliases = [];
  let until;
  for (let page = 0; page < 20; page += 1) {
    const endpoint = new URL("https://api.vercel.com/v4/aliases");
    endpoint.searchParams.set("projectId", projectId);
    endpoint.searchParams.set("teamId", teamId);
    endpoint.searchParams.set("limit", "100");
    if (until) {
      endpoint.searchParams.set("until", String(until));
    }

    const payload = await checkedJSON(
      await fetchImpl(endpoint, {
        headers: { Authorization: `Bearer ${token}` },
      }),
      "Listing Vercel aliases",
    );
    aliases.push(...(payload.aliases ?? []));
    until = payload.pagination?.next;
    if (!until) {
      break;
    }
  }

  const matches = selectPreviewAliases(aliases, pullRequestNumber);
  for (const alias of matches) {
    const endpoint = new URL(
      `https://api.vercel.com/v2/aliases/${encodeURIComponent(alias)}`,
    );
    endpoint.searchParams.set("teamId", teamId);
    await checkedJSON(
      await fetchImpl(endpoint, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      }),
      `Deleting Vercel alias ${alias}`,
    );
    console.log(`Deleted Vercel alias ${alias}`);
  }

  console.log(`Cleaned ${matches.length} Vercel Preview alias(es)`);
  return matches;
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  cleanupPreviewAliases({
    pullRequestNumber: process.env.PR_NUMBER,
    projectId: process.env.VERCEL_PROJECT_ID,
    teamId: process.env.VERCEL_ORG_ID,
    token: process.env.VERCEL_TOKEN,
  }).catch((error) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}

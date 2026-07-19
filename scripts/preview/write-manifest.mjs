import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

const permanentHosts = new Set([
  "api.unipost.dev",
  "dev-api.unipost.dev",
  "staging-api.unipost.dev",
  "unipost-dev.up.railway.app",
  "unipost-production.up.railway.app",
  "unipost-staging.up.railway.app",
]);

export function createPreviewManifest({ sha, branch, apiURL }) {
  if (!/^[a-f0-9]{40}$/i.test(sha ?? "")) {
    throw new Error("Preview manifest requires a full 40-character commit SHA");
  }
  if (!/^(?:dev-|hotfix-)[a-z0-9][a-z0-9-]*$/i.test(branch ?? "")) {
    throw new Error(
      "Preview manifest branch must use the dev-<task-slug> or hotfix-<task-slug> form",
    );
  }

  let parsedAPI;
  try {
    parsedAPI = new URL(apiURL);
  } catch {
    throw new Error("Preview manifest requires an ephemeral Railway HTTPS API URL");
  }
  if (
    parsedAPI.protocol !== "https:"
    || !parsedAPI.hostname.endsWith(".up.railway.app")
    || permanentHosts.has(parsedAPI.hostname)
  ) {
    throw new Error("Preview manifest requires an ephemeral Railway HTTPS API URL");
  }

  return {
    sha: sha.toLowerCase(),
    branch,
    apiURL: parsedAPI.origin,
  };
}

function parseArgs(argv) {
  const values = {};
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (!argument.startsWith("--")) throw new Error(`Unexpected argument: ${argument}`);
    const [key, inlineValue] = argument.slice(2).split("=", 2);
    const value = inlineValue ?? argv[index + 1];
    if (inlineValue === undefined) index += 1;
    if (!value || value.startsWith("--")) throw new Error(`Missing value for --${key}`);
    values[key] = value;
  }
  return values;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const output = args.output || "dashboard/public/__unipost-preview.json";
  const manifest = createPreviewManifest({
    sha: args.sha,
    branch: args.branch,
    apiURL: args["api-url"],
  });
  await mkdir(path.dirname(output), { recursive: true });
  await writeFile(output, `${JSON.stringify(manifest, null, 2)}\n`, { mode: 0o644 });
  console.log(`Wrote preview manifest for ${manifest.branch} at ${manifest.sha}`);
}

const invokedPath = process.argv[1] ? pathToFileURL(path.resolve(process.argv[1])).href : "";
if (import.meta.url === invokedPath) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}

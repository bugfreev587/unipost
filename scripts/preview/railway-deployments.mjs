import { appendFile, mkdir, writeFile } from "node:fs/promises";
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

const terminalStates = new Set(["error", "failure"]);
const railwayTerminalStates = new Set(["CRASHED", "FAILED", "REMOVED", "SKIPPED"]);

export class PreviewPendingError extends Error {
  constructor(message) {
    super(message);
    this.name = "PreviewPendingError";
  }
}

export class PreviewTerminalError extends Error {
  constructor(message) {
    super(message);
    this.name = "PreviewTerminalError";
  }
}

function railwayPreviewURL(rawURL) {
  if (!rawURL) return null;

  let url;
  try {
    url = new URL(rawURL);
  } catch {
    return null;
  }

  if (url.protocol !== "https:") return null;
  if (!url.hostname.endsWith(".up.railway.app")) return null;
  if (permanentHosts.has(url.hostname)) return null;
  return `${url.origin}${url.pathname === "/" ? "" : url.pathname}`.replace(/\/+$/, "");
}

export function selectRailwayEnvironment(deployments, expectedSHA) {
  if (!/^[a-f0-9]{40}$/i.test(expectedSHA)) {
    throw new PreviewTerminalError("Expected a full 40-character pull request head SHA");
  }

  const exact = deployments.filter((deployment) => deployment.sha === expectedSHA);
  if (exact.length === 0) {
    throw new PreviewPendingError("No Railway deployment matches the exact head SHA yet");
  }

  const candidates = [];
  const terminal = [];
  for (const deployment of exact) {
    const environmentId = deployment.payload?.environmentId;
    if (typeof environmentId !== "string" || environmentId.length === 0) continue;

    for (const status of deployment.statuses ?? []) {
      if (terminalStates.has(status.state)) {
        terminal.push({ deployment, environmentId, status });
      } else {
        candidates.push({ deployment, environmentId, status });
      }
    }
  }

  if (terminal.length > 0 && candidates.length === 0) {
    const states = [...new Set(terminal.map(({ status }) => status.state))].join(", ");
    throw new PreviewTerminalError(`Railway PR environment deployment reached terminal state: ${states}`);
  }

  const byEnvironment = new Map();
  for (const candidate of candidates) {
    const existing = byEnvironment.get(candidate.environmentId);
    const candidateTime = Date.parse(candidate.status.created_at ?? 0) || 0;
    const existingTime = Date.parse(existing?.status.created_at ?? 0) || 0;
    if (!existing || candidateTime >= existingTime) {
      byEnvironment.set(candidate.environmentId, candidate);
    }
  }

  if (byEnvironment.size === 0) {
    throw new PreviewPendingError(
      "No Railway PR environment deployment with an environmentId yet",
    );
  }
  if (byEnvironment.size > 1) {
    throw new PreviewTerminalError("Found multiple Railway PR environments for the exact head SHA");
  }

  const [{ deployment, environmentId, status }] = byEnvironment.values();
  return {
    environmentId,
    deploymentId: deployment.id,
    environment: deployment.environment,
    githubState: status.state,
    sha: expectedSHA,
  };
}

export function selectRailwayPreviewService(environment) {
  if (!environment || !/^unipost-pr-\d+$/.test(environment.name ?? "")) {
    throw new PreviewTerminalError("Railway environment is not an ephemeral UniPost PR environment");
  }

  const services = (environment.serviceInstances?.edges ?? [])
    .map(({ node }) => node)
    .filter((service) => service?.serviceName === "preview-api");

  if (services.length === 0) {
    throw new PreviewPendingError("Railway preview-api service has not appeared yet");
  }
  if (services.length > 1) {
    throw new PreviewTerminalError("Railway PR environment contains multiple preview-api services");
  }

  return services[0];
}

export function selectRailwayPreviewAPI(environment, expectedSHA) {
  const service = selectRailwayPreviewService(environment);
  const deployment = service.latestDeployment;
  if (!deployment) {
    throw new PreviewPendingError("Railway preview-api deployment has not appeared yet");
  }
  if (railwayTerminalStates.has(deployment.status)) {
    throw new PreviewTerminalError(
      `Railway preview-api deployment reached terminal state: ${deployment.status}`,
    );
  }
  if (!["SUCCESS", "SLEEPING"].includes(deployment.status)) {
    throw new PreviewPendingError(
      `Railway preview-api deployment is ${deployment.status ?? "not ready"}`,
    );
  }
  if (deployment.meta?.commitHash !== expectedSHA) {
    throw new PreviewPendingError("Railway preview-api deployment does not match the exact head SHA");
  }

  const domains = (service.domains?.serviceDomains ?? [])
    .map(({ domain }) => railwayPreviewURL(`https://${domain}`))
    .filter(Boolean);
  const uniqueDomains = [...new Set(domains)];
  if (uniqueDomains.length === 0) {
    throw new PreviewPendingError("Railway preview-api has no ephemeral public domain yet");
  }
  if (uniqueDomains.length > 1) {
    throw new PreviewTerminalError("Railway preview-api has multiple ephemeral public domains");
  }

  return {
    apiURL: uniqueDomains[0],
    railwayEnvironmentId: environment.id,
    railwayEnvironmentName: environment.name,
  };
}

function parseArgs(argv) {
  const values = {};
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (!argument.startsWith("--")) {
      throw new Error(`Unexpected argument: ${argument}`);
    }
    const [rawKey, inlineValue] = argument.slice(2).split("=", 2);
    const value = inlineValue ?? argv[index + 1];
    if (inlineValue === undefined) index += 1;
    if (!value || value.startsWith("--")) {
      throw new Error(`Missing value for --${rawKey}`);
    }
    values[rawKey] = value;
  }
  return values;
}

async function githubJSON(url, token) {
  const response = await fetch(url, {
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${token}`,
      "X-GitHub-Api-Version": "2022-11-28",
    },
  });
  if (!response.ok) {
    throw new PreviewTerminalError(`GitHub deployments API returned ${response.status}`);
  }
  return response.json();
}

async function loadDeployments(repo, sha, token) {
  const deployments = await githubJSON(
    `https://api.github.com/repos/${repo}/deployments?sha=${sha}&per_page=100`,
    token,
  );

  return Promise.all(deployments.map(async (deployment) => ({
    id: deployment.id,
    sha: deployment.sha,
    environment: deployment.environment,
    payload: deployment.payload,
    statuses: await githubJSON(deployment.statuses_url, token),
  })));
}

async function railwayGraphQL(query, variables, token) {
  const response = await fetch("https://backboard.railway.com/graphql/v2", {
    method: "POST",
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      query,
      variables,
    }),
  });

  if (!response.ok) {
    throw new PreviewTerminalError(`Railway API returned ${response.status}`);
  }
  const payload = await response.json();
  if (payload.errors?.length) {
    const message = payload.errors.map(({ message }) => message).join("; ");
    throw new PreviewTerminalError(`Railway API error: ${message}`);
  }
  return payload.data;
}

async function loadRailwayEnvironment(environmentId, token) {
  const data = await railwayGraphQL(
    `query environment($id: String!) {
        environment(id: $id) {
          id
          name
          serviceInstances {
            edges {
              node {
                serviceId
                serviceName
                latestDeployment {
                  id
                  status
                  meta
                }
                domains {
                  serviceDomains {
                    domain
                  }
                }
              }
            }
          }
        }
      }`,
    { id: environmentId },
    token,
  );
  if (!data?.environment) {
    throw new PreviewPendingError(`Railway environment ${environmentId} is not queryable yet`);
  }
  return data.environment;
}

async function triggerRailwayPreview({
  projectId,
  environmentId,
  serviceId,
  token,
}) {
  const data = await railwayGraphQL(
    `mutation environmentTriggersDeploy($input: EnvironmentTriggersDeployInput!) {
      environmentTriggersDeploy(input: $input)
    }`,
    {
      input: {
        projectId,
        environmentId,
        serviceId,
      },
    },
    token,
  );
  if (data?.environmentTriggersDeploy !== true) {
    throw new PreviewTerminalError("Railway API did not accept the preview-api deployment trigger");
  }
}

async function healthIsReady(apiURL) {
  try {
    const response = await fetch(`${apiURL}/health`, {
      headers: { Accept: "application/json" },
      redirect: "error",
    });
    return response.ok;
  } catch {
    return false;
  }
}

async function waitForRailwayPreview({
  repo,
  sha,
  projectId,
  githubToken,
  railwayToken,
  timeoutMs = 25 * 60 * 1000,
  pollMs = 10_000,
}) {
  const deadline = Date.now() + timeoutMs;
  let lastPending = "Railway PR API deployment has not appeared";
  let deploymentTriggered = false;

  while (Date.now() < deadline) {
    const deployments = await loadDeployments(repo, sha, githubToken);
    try {
      const githubDeployment = selectRailwayEnvironment(deployments, sha);
      const environment = await loadRailwayEnvironment(
        githubDeployment.environmentId,
        railwayToken,
      );
      const service = selectRailwayPreviewService(environment);
      if (service.latestDeployment?.meta?.commitHash !== sha) {
        const githubIsRunning = ["in_progress", "pending", "queued"].includes(
          githubDeployment.githubState,
        );
        if (!githubIsRunning && !deploymentTriggered) {
          await triggerRailwayPreview({
            projectId,
            environmentId: environment.id,
            serviceId: service.serviceId,
            token: railwayToken,
          });
          deploymentTriggered = true;
          console.log(`Triggered Railway preview-api for exact SHA ${sha}`);
        }
        throw new PreviewPendingError(
          "Railway preview-api deployment does not match the exact head SHA yet",
        );
      }
      const railwayPreview = selectRailwayPreviewAPI(environment, sha);
      const preview = { ...githubDeployment, ...railwayPreview };
      if (await healthIsReady(preview.apiURL)) return preview;
      lastPending = `Railway PR API health is not ready at ${preview.apiURL}`;
    } catch (error) {
      if (error instanceof PreviewTerminalError) throw error;
      if (!(error instanceof PreviewPendingError)) throw error;
      lastPending = error.message;
    }

    console.log(`${lastPending}; polling again`);
    await new Promise((resolve) => setTimeout(resolve, pollMs));
  }

  throw new PreviewTerminalError(`Timed out waiting for Railway preview: ${lastPending}`);
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  const repo = args.repo;
  const sha = args.sha;
  const projectId = args["project-id"] || process.env.RAILWAY_PROJECT_ID;
  const githubToken = args.token || process.env.GITHUB_TOKEN;
  const railwayToken = args["railway-token"] || process.env.RAILWAY_API_TOKEN;
  const output = args.output || process.env.GITHUB_OUTPUT;
  const manifest = args.manifest || "artifacts/preview/railway.json";

  if (!repo || !sha || !projectId || !githubToken || !railwayToken || !output) {
    throw new Error(
      "--repo, --sha, --project-id, --token, --railway-token, and --output are required",
    );
  }

  const preview = await waitForRailwayPreview({
    repo,
    sha,
    projectId,
    githubToken,
    railwayToken,
  });
  await mkdir(path.dirname(manifest), { recursive: true });
  await writeFile(manifest, `${JSON.stringify(preview, null, 2)}\n`, { mode: 0o600 });
  await appendFile(output, [
    `api_url=${preview.apiURL}`,
    `railway_deployment_id=${preview.deploymentId}`,
    `railway_environment=${preview.railwayEnvironmentName}`,
    "",
  ].join("\n"));
  console.log(`Railway preview is ready for ${sha} at ${preview.apiURL}`);
}

const invokedPath = process.argv[1] ? pathToFileURL(path.resolve(process.argv[1])).href : "";
if (import.meta.url === invokedPath) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}

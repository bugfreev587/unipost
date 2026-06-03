import { createHash } from "node:crypto";
import { execFile as execFileCallback } from "node:child_process";
import { createReadStream } from "node:fs";
import { mkdir, readFile, stat, writeFile } from "node:fs/promises";
import { homedir, platform } from "node:os";
import { basename, extname, join } from "node:path";
import { promisify } from "node:util";

const execFile = promisify(execFileCallback);

const CLI_VERSION = "0.1.1";
const CLI_RUNNER = "npx -y @unipost/cli";
const DEFAULT_BASE_URL = "https://api.unipost.dev";
const DOCS_QUICKSTART_URL = "https://unipost.dev/docs/quickstart";
const DOCS_CLI_URL = "https://unipost.dev/docs/cli";
const KEYCHAIN_SERVICE = "dev.unipost.cli";
const AGENT_CATALOG_VERSION = "2026-06-03.phase5";
const MCP_ENDPOINT = "https://mcp.unipost.dev/mcp";
const AGENT_PACKAGE_FILES = [
  "agent-packages/codex/SKILL.md",
  "agent-packages/claude-code/CLAUDE.md",
];
const TERMINAL_POST_STATUSES = new Set(["published", "failed", "partial", "canceled"]);
const TERMINAL_MEDIA_STATUSES = new Set(["ready", "failed"]);

const EXIT = {
  success: 0,
  generic: 1,
  invalidArgs: 2,
  missingInput: 3,
  auth: 4,
  authz: 5,
  validation: 6,
  upstream: 7,
  network: 8,
  unsafe: 9,
  timeout: 10,
};

const ERROR_EXIT_BY_CODE = new Map([
  ["unauthorized", EXIT.auth],
  ["forbidden", EXIT.authz],
  ["validation_error", EXIT.validation],
  ["invalid_request", EXIT.validation],
  ["bad_request", EXIT.validation],
  ["not_found", EXIT.validation],
  ["account_already_connected", EXIT.validation],
  ["account_disconnected", EXIT.validation],
  ["not_supported", EXIT.validation],
  ["plan_post_quota_exceeded", EXIT.validation],
  ["request_rate_limited", EXIT.upstream],
  ["rate_limited", EXIT.upstream],
  ["setup_token_expired", EXIT.auth],
  ["setup_token_invalid", EXIT.auth],
  ["setup_token_used", EXIT.auth],
  ["enqueue_rate_limited", EXIT.upstream],
  ["queue_depth_exceeded", EXIT.upstream],
  ["upstream_error", EXIT.upstream],
  ["internal_error", EXIT.upstream],
]);

const GLOBAL_FLAGS_WITH_VALUES = new Set([
  "--output",
  "--field",
  "--base-url",
  "--api-key",
  "--setup-token",
  "--profile",
  "--account",
  "--client",
  "--intent",
  "--idempotency-key",
  "--agent-name",
  "--schedule-at",
  "--at",
  "--limit",
  "--cursor",
  "--status",
  "--result",
  "--from",
  "--to",
  "--format",
  "--content-type",
  "--lang",
  "--name",
  "--platform",
  "--external-user-id",
  "--external-user-email",
  "--return-url",
  "--caption",
  "--timeout",
  "--from-file",
  "--plan",
]);

const GLOBAL_BOOLEAN_FLAGS = new Set([
  "--json",
  "--non-interactive",
  "--yes",
  "--quiet",
  "--verbose",
  "--no-color",
  "--dry-run",
  "--all",
  "--no-telemetry",
  "--open",
  "--insecure",
  "--force",
  "--reauth",
  "--replace-key",
  "--allow-quickstart-creds",
  "--no-quickstart-creds",
]);

class CliError extends Error {
  constructor({
    code,
    normalizedCode,
    message,
    hint,
    docsUrl,
    exitCode,
    requestId,
    status,
    cause,
  }) {
    super(message, { cause });
    this.code = code || normalizedCode || "error";
    this.normalizedCode = normalizedCode || code || "error";
    this.hint = hint;
    this.docsUrl = docsUrl;
    this.exitCode = exitCode || EXIT.generic;
    this.requestId = requestId;
    this.status = status;
  }
}

export async function main(argv, io = {}) {
  let context;
  try {
    context = createContext(argv, io);
    await applyConfigDefaults(context);
    const result = await dispatch(context);
    writeResult(context, result);
    return result.exitCode || EXIT.success;
  } catch (error) {
    const cliError = normalizeError(error);
    writeError(context || fallbackContext(argv, io), cliError);
    return cliError.exitCode;
  }
}

function createContext(argv, io) {
  const env = io.env || {};
  const parsed = parseArgs(argv);
  const output = parsed.options.json ? "json" : parsed.options.output || "table";
  const baseUrl = normalizeBaseUrl(parsed.options.baseUrl || env.UNIPOST_BASE_URL || DEFAULT_BASE_URL);
  const baseUrlSource = parsed.options.baseUrl ? "flag" : env.UNIPOST_BASE_URL ? "env" : "default";
  const telemetry = resolveTelemetry(parsed.options, env);
  const credentialSource = parsed.options.apiKey ? "flag" : env.UNIPOST_API_KEY ? "env" : "none";

  return {
    argv,
    commandParts: parsed.commandParts,
    options: {
      ...parsed.options,
      output,
      baseUrl,
      baseUrlSource,
      apiKey: parsed.options.apiKey || env.UNIPOST_API_KEY || "",
      credentialSource,
      noColor: Boolean(parsed.options.noColor || env.NO_COLOR),
      telemetry,
    },
    env,
    stdout: io.stdout || process.stdout,
    stderr: io.stderr || process.stderr,
    fetchImpl: io.fetchImpl || globalThis.fetch,
    secureStore: io.secureStore || createDefaultSecureStore(),
  };
}

function fallbackContext(argv, io) {
  const env = io.env || {};
  const baseUrl = normalizeBaseUrl(env.UNIPOST_BASE_URL || DEFAULT_BASE_URL);
  return {
    argv,
    commandParts: [],
    options: {
      output: "table",
      baseUrl,
      baseUrlSource: env.UNIPOST_BASE_URL ? "env" : "default",
      telemetry: resolveTelemetry({}, env),
    },
    env,
    stdout: io.stdout || process.stdout,
    stderr: io.stderr || process.stderr,
    fetchImpl: io.fetchImpl || globalThis.fetch,
    secureStore: io.secureStore || createDefaultSecureStore(),
  };
}

function createDefaultSecureStore() {
  if (platform() !== "darwin") {
    return {
      async set() {
        throw keychainUnavailableError();
      },
      async get() {
        throw keychainUnavailableError();
      },
      async delete() {
        throw keychainUnavailableError();
      },
    };
  }
  return {
    async set({ service, account, value }) {
      await execFile("/usr/bin/security", [
        "add-generic-password",
        "-s", service,
        "-a", account,
        "-w", value,
        "-U",
      ]);
    },
    async get({ service, account }) {
      try {
        const { stdout } = await execFile("/usr/bin/security", [
          "find-generic-password",
          "-s", service,
          "-a", account,
          "-w",
        ]);
        return String(stdout || "").trim();
      } catch (error) {
        if (String(error?.stderr || error?.message || "").includes("could not be found")) {
          return "";
        }
        throw error;
      }
    },
    async delete({ service, account }) {
      try {
        await execFile("/usr/bin/security", [
          "delete-generic-password",
          "-s", service,
          "-a", account,
        ]);
      } catch (error) {
        if (!String(error?.stderr || error?.message || "").includes("could not be found")) {
          throw error;
        }
      }
    },
  };
}

function keychainUnavailableError() {
  return new CliError({
    code: "keychain_unavailable",
    normalizedCode: "keychain_unavailable",
    message: "OS keychain storage is unavailable on this platform.",
    hint: "Use UNIPOST_API_KEY for command-level fallback auth, or run setup-token login on macOS keychain.",
    docsUrl: DOCS_CLI_URL,
    exitCode: EXIT.auth,
  });
}

function parseArgs(argv) {
  const options = {};
  const commandParts = [];

  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--help" || arg === "-h") {
      options.help = true;
      continue;
    }
    if (arg === "--version" || arg === "-v") {
      options.version = true;
      continue;
    }
    if (GLOBAL_BOOLEAN_FLAGS.has(arg)) {
      setBooleanOption(options, arg);
      continue;
    }
    if (GLOBAL_FLAGS_WITH_VALUES.has(arg)) {
      const value = argv[index + 1];
      if (!value || value.startsWith("--")) {
        throw new CliError({
          code: "missing_required_input",
          normalizedCode: "missing_required_input",
          message: `${arg} requires a value.`,
          hint: `Pass a value after ${arg}.`,
          exitCode: EXIT.missingInput,
        });
      }
      setValueOption(options, arg, value);
      index += 1;
      continue;
    }
    if (arg.startsWith("--")) {
      throw new CliError({
        code: "invalid_argument",
        normalizedCode: "invalid_argument",
        message: `Unknown option: ${arg}`,
        hint: "Run unipost --help to see supported flags.",
        docsUrl: DOCS_CLI_URL,
        exitCode: EXIT.invalidArgs,
      });
    }
    commandParts.push(arg);
  }

  return { options, commandParts };
}

function setBooleanOption(options, flag) {
  const key = toCamelCase(flag.slice(2));
  options[key] = true;
  if (flag === "--json") {
    options.json = true;
  }
}

function setValueOption(options, flag, value) {
  const key = toCamelCase(flag.slice(2));
  if (flag === "--output") {
    if (!["table", "json", "yaml"].includes(value)) {
      throw new CliError({
        code: "invalid_argument",
        normalizedCode: "invalid_argument",
        message: "--output must be one of: table, json, yaml.",
        exitCode: EXIT.invalidArgs,
      });
    }
    options.output = value;
    return;
  }
  if (flag === "--limit") {
    const limit = Number(value);
    if (!Number.isInteger(limit) || limit <= 0) {
      throw new CliError({
        code: "invalid_argument",
        normalizedCode: "invalid_argument",
        message: "--limit must be a positive integer.",
        exitCode: EXIT.invalidArgs,
      });
    }
    options.limit = limit;
    return;
  }
  if (flag === "--timeout") {
    const timeout = Number(value);
    if (!Number.isFinite(timeout) || timeout < 0) {
      throw new CliError({
        code: "invalid_argument",
        normalizedCode: "invalid_argument",
        message: "--timeout must be a non-negative number of seconds.",
        exitCode: EXIT.invalidArgs,
      });
    }
    options.timeout = timeout;
    return;
  }
  options[key] = value;
}

function toCamelCase(value) {
  return value.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
}

async function dispatch(context) {
  const [command, subcommand, third] = context.commandParts;

  if (context.options.version) {
    return textResult(`${CLI_VERSION}\n`);
  }
  if (context.options.help || !command) {
    return textResult(helpText());
  }
  if (command === "auth" && subcommand === "status") {
    return authStatus(context);
  }
  if (command === "auth" && subcommand === "login") {
    return authLogin(context);
  }
  if (command === "auth" && subcommand === "logout") {
    return authLogout(context);
  }
  if (command === "auth" && subcommand === "list") {
    return authList(context);
  }
  if (command === "auth" && subcommand === "use") {
    return authUse(context, third);
  }
  if (command === "config") {
    return configCommand(context, subcommand, third);
  }
  if (command === "init") {
    return init(context);
  }
  if (command === "quickstart") {
    return quickstart(context);
  }
  if (command === "profiles") {
    return profiles(context, subcommand, third);
  }
  if (command === "connect") {
    return connect(context, subcommand, third);
  }
  if (command === "accounts") {
    return accounts(context, subcommand, third);
  }
  if (command === "posts") {
    return posts(context, subcommand, third);
  }
  if (command === "media") {
    return media(context, subcommand, third);
  }
  if (command === "analytics") {
    return analytics(context, subcommand, third);
  }
  if (command === "examples") {
    return examples(context, subcommand);
  }
  if (command === "agent") {
    return agent(context, subcommand, third);
  }
  if (command === "doctor") {
    return doctor(context);
  }
  if (command === "completion") {
    return completion(context, subcommand);
  }

  throw new CliError({
    code: "invalid_command",
    normalizedCode: "invalid_command",
    message: `Unknown command: ${context.commandParts.join(" ")}`,
    hint: "Run unipost --help to see supported commands.",
    docsUrl: DOCS_CLI_URL,
    exitCode: EXIT.invalidArgs,
  });
}

async function authStatus(context) {
  requireApiKey(context, "auth status");

  const response = await requestJson(context, "/v1/workspace", { auth: true });
  const workspace = unwrapData(response.body);

  return envelopeResult({
    data: {
      authenticated: true,
      credential_source: context.options.credentialSource,
      workspace,
    },
    meta: {
      request_id: response.requestId,
      rate_limit: response.rateLimit,
    },
    human: `Authenticated with workspace ${workspace?.id || "unknown"}.\n`,
  });
}

async function authLogin(context) {
  if (context.options.setupToken) {
    return loginWithSetupToken(context);
  }
  if (!context.options.apiKey) {
    throw new CliError({
      code: "missing_required_input",
      normalizedCode: "missing_required_input",
      message: "auth login requires --api-key or --setup-token.",
      hint: "Use auth login --setup-token <token> from the dashboard agent setup flow, or pass --api-key for metadata-only fallback.",
      docsUrl: DOCS_CLI_URL,
      exitCode: EXIT.missingInput,
    });
  }

  const { workspace, response } = await fetchWorkspace(context);
  const credential = {
    type: "api_key",
    storage: "metadata_only",
    workspace_id: workspace?.id || "",
    workspace_name: workspace?.name || "",
    credential_source: "auth login --api-key",
    key_fingerprint: fingerprintSecret(context.options.apiKey),
    authenticated_at: new Date().toISOString(),
  };
  const config = await patchConfig(context, {
    credential,
    default_workspace_id: workspace?.id || "",
  });

  return envelopeResult({
    data: {
      authenticated: true,
      workspace,
      credential: config.credential,
      stored_secret: false,
      config_path: configPath(context),
      next_actions: [
        "Use auth login --setup-token for keychain-backed CLI auth, or continue passing UNIPOST_API_KEY for command-level fallback.",
      ],
    },
    warnings: [{
      code: "credential_metadata_only",
      message: "auth login records redacted credential metadata only; it does not store the API key locally.",
    }],
    meta: {
      request_id: response.requestId,
      rate_limit: response.rateLimit,
    },
    human: `Logged in to workspace ${workspace?.id || "unknown"} metadata only. Set UNIPOST_API_KEY for authenticated commands.\n`,
  });
}

async function loginWithSetupToken(context) {
  const client = context.options.client || "codex";
  const exchange = await exchangeSetupToken(context, client);
  const apiKey = exchange.api_key || exchange.apiKey;
  if (!apiKey?.key) {
    throw new CliError({
      code: "invalid_response",
      normalizedCode: "invalid_response",
      message: "Setup-token exchange did not return an API key.",
      exitCode: EXIT.upstream,
    });
  }

  const previousApiKey = context.options.apiKey;
  const previousCredentialSource = context.options.credentialSource;
  context.options.apiKey = apiKey.key;
  context.options.credentialSource = "setup_token";
  try {
    const { workspace, response } = await fetchWorkspace(context);
    const credential = await storeKeychainCredential(context, {
      apiKey,
      workspace,
      client: exchange.client || client,
    });

    return envelopeResult({
      data: {
        authenticated: true,
        workspace,
        credential,
        stored_secret: true,
        config_path: configPath(context),
        next_actions: [
          `Run ${CLI_RUNNER} auth status --json to confirm keychain-backed auth.`,
          `Run ${CLI_RUNNER} agent bootstrap --json before agent workflows.`,
        ],
      },
      meta: {
        request_id: response.requestId,
        rate_limit: response.rateLimit,
      },
      human: `Logged in to workspace ${workspace?.id || "unknown"} with keychain-backed CLI auth.\n`,
    });
  } catch (error) {
    context.options.apiKey = previousApiKey;
    context.options.credentialSource = previousCredentialSource;
    throw error;
  }
}

async function exchangeSetupToken(context, client) {
  const response = await requestJson(context, "/v1/cli/setup-tokens/exchange", {
    method: "POST",
    auth: false,
    body: {
      setup_token: context.options.setupToken,
      client,
    },
  });
  return unwrapData(response.body);
}

async function storeKeychainCredential(context, { apiKey, workspace, client }) {
  const workspaceID = workspace?.id || "";
  const keyID = apiKey.id || "";
  const account = `workspace:${workspaceID}:key:${keyID || fingerprintSecret(apiKey.key)}`;
  await context.secureStore.set({
    service: KEYCHAIN_SERVICE,
    account,
    value: apiKey.key,
  });
  const credential = {
    type: "api_key",
    storage: "keychain",
    workspace_id: workspaceID,
    workspace_name: workspace?.name || "",
    client,
    key_id: keyID,
    key_name: apiKey.name || "",
    key_prefix: apiKey.prefix || "",
    key_fingerprint: fingerprintSecret(apiKey.key),
    keychain_service: KEYCHAIN_SERVICE,
    keychain_account: account,
    credential_source: "setup_token",
    authenticated_at: new Date().toISOString(),
  };
  const config = await patchConfig(context, {
    credential,
    default_workspace_id: workspaceID,
  });
  return config.credential;
}

async function authLogout(context) {
  const current = await readConfig(context);
  const hadCredential = Object.prototype.hasOwnProperty.call(current, "credential");
  if (current.credential?.storage === "keychain" && current.credential.keychain_account) {
    await context.secureStore.delete({
      service: current.credential.keychain_service || KEYCHAIN_SERVICE,
      account: current.credential.keychain_account,
    });
  }
  const { credential: _credential, ...rest } = current;
  const config = await writeConfig(context, {
    ...rest,
    updated_at: new Date().toISOString(),
  });

  return envelopeResult({
    data: {
      removed_credential: hadCredential,
      remote_revoked: false,
      config_path: configPath(context),
      config,
    },
    warnings: [{
      code: "remote_revoke_not_requested",
      message: "auth logout clears local credentials only; revoke the named API key from the UniPost dashboard if it should stop working remotely.",
    }],
    human: hadCredential ? "Removed local credential metadata.\n" : "No local credential metadata found.\n",
  });
}

async function configCommand(context, subcommand, key) {
  if (subcommand === "path") {
    const path = configPath(context);
    return envelopeResult({
      data: { config_path: path },
      human: `${path}\n`,
    });
  }

  if (subcommand === "show") {
    const config = sanitizeConfig(await readConfig(context));
    return envelopeResult({
      data: {
        config,
        config_path: configPath(context),
      },
      human: `${JSON.stringify(config, null, 2)}\n`,
    });
  }

  if (subcommand === "set") {
    const value = context.commandParts.slice(3).join(" ");
    const configKey = requireValue(key, "config_key", "config set requires a key.");
    const configValue = requireValue(value, "config_value", "config set requires a value.");
    const patch = configPatch(configKey, configValue);
    const config = sanitizeConfig(await patchConfig(context, patch));
    return envelopeResult({
      data: {
        key: configKey,
        value: config[configKey],
        config,
        config_path: configPath(context),
      },
      human: `Set ${configKey}.\n`,
    });
  }

  return invalidSubcommand("config", subcommand);
}

function configPatch(key, value) {
  if (key === "base_url") {
    const url = normalizeBaseUrl(value);
    validateConfigBaseURL(url);
    return { base_url: url };
  }
  if (key === "default_profile_id") {
    return { default_profile_id: value };
  }
  if (key === "default_workspace_id") {
    return { default_workspace_id: value };
  }
  throw new CliError({
    code: "invalid_argument",
    normalizedCode: "invalid_argument",
    message: `Unsupported config key: ${key}`,
    hint: "Supported config keys: base_url, default_profile_id, default_workspace_id.",
    docsUrl: DOCS_CLI_URL,
    exitCode: EXIT.invalidArgs,
  });
}

function validateConfigBaseURL(value) {
  try {
    const url = new URL(value);
    if (!["http:", "https:"].includes(url.protocol)) {
      throw new Error("Unsupported protocol");
    }
  } catch {
    throw new CliError({
      code: "invalid_argument",
      normalizedCode: "invalid_argument",
      message: "base_url must be a valid http or https URL.",
      exitCode: EXIT.invalidArgs,
    });
  }
}

async function doctor(context) {
  const checks = [
    {
      id: "cli_version",
      status: "pass",
      message: `UniPost CLI ${CLI_VERSION}`,
    },
  ];
  let workspace = null;
  let lastRequestId = "";
  let lastRateLimit = {};
  let hardExit = EXIT.success;

  const health = await runCheck(async () => requestJson(context, "/health", { auth: false }));
  if (health.ok) {
    checks.push({
      id: "api_reachability",
      status: "pass",
      message: "API is reachable.",
    });
    lastRequestId = health.value.requestId || lastRequestId;
  } else {
    checks.push({
      id: "api_reachability",
      status: "fail",
      message: health.error.message,
    });
    hardExit = chooseHardExit(hardExit, health.error.exitCode);
  }

  if (!context.options.apiKey) {
    checks.push({
      id: "auth",
      status: "fail",
      message: "No API key found.",
      hint: "Set UNIPOST_API_KEY or pass --api-key.",
    });
    checks.push({
      id: "workspace",
      status: "fail",
      message: "Workspace check skipped because auth failed.",
    });
    checks.push({
      id: "rate_limit_headers",
      status: "warn",
      message: "No authenticated response was available.",
    });
    checks.push({
      id: "request_id",
      status: lastRequestId ? "pass" : "warn",
      message: lastRequestId ? "API returned a request id." : "No request id observed.",
    });
    hardExit = chooseHardExit(hardExit, EXIT.auth);
  } else {
    const auth = await runCheck(async () => requestJson(context, "/v1/workspace", { auth: true }));
    if (auth.ok) {
      workspace = auth.value.body?.data ?? auth.value.body;
      lastRequestId = auth.value.requestId || lastRequestId;
      lastRateLimit = auth.value.rateLimit;
      checks.push({
        id: "auth",
        status: "pass",
        message: "API key authenticated.",
      });
      checks.push({
        id: "workspace",
        status: "pass",
        message: `Workspace ${workspace?.id || "unknown"} is accessible.`,
      });
    } else {
      checks.push({
        id: "auth",
        status: "fail",
        message: auth.error.message,
        hint: auth.error.hint,
      });
      checks.push({
        id: "workspace",
        status: "fail",
        message: "Workspace check failed.",
      });
      hardExit = chooseHardExit(hardExit, auth.error.exitCode);
    }
    checks.push({
      id: "rate_limit_headers",
      status: Object.keys(lastRateLimit).length > 0 ? "pass" : "warn",
      message: Object.keys(lastRateLimit).length > 0 ? "Rate-limit headers observed." : "No rate-limit headers observed.",
    });
    checks.push({
      id: "request_id",
      status: lastRequestId ? "pass" : "warn",
      message: lastRequestId ? "API returned a request id." : "No request id observed.",
    });
  }

  return envelopeResult({
    data: {
      checks,
      workspace,
      telemetry: context.options.telemetry,
    },
    meta: {
      request_id: lastRequestId,
      rate_limit: lastRateLimit,
    },
    human: renderDoctor(checks, workspace, context.options.telemetry),
    exitCode: hardExit,
  });
}

async function authList(context) {
  if (!context.options.apiKey) {
    return envelopeResult({
      data: {
        credentials: [],
        active_workspace_id: "",
      },
      human: "No local or environment credential is active.\n",
      exitCode: EXIT.auth,
    });
  }
  const { workspace, response } = await fetchWorkspace(context);
  const config = await readConfig(context);
  return envelopeResult({
    data: {
      credentials: [{
        workspace_id: workspace?.id || "",
        workspace_name: workspace?.name || "",
        credential_source: context.options.credentialSource,
        active: true,
      }],
      active_workspace_id: config.default_workspace_id || workspace?.id || "",
    },
    meta: {
      request_id: response.requestId,
      rate_limit: response.rateLimit,
    },
    human: `Active workspace: ${workspace?.id || "unknown"} (${context.options.credentialSource}).\n`,
  });
}

async function authUse(context, workspaceID) {
  const id = requireValue(workspaceID, "workspace_id", "auth use requires a workspace ID.");
  const config = await patchConfig(context, { default_workspace_id: id });
  return envelopeResult({
    data: {
      default_workspace_id: config.default_workspace_id,
      config_path: configPath(context),
    },
    human: `Default workspace set to ${id}.\n`,
  });
}

async function init(context) {
  if (!context.options.apiKey && context.options.setupToken) {
    await loginWithSetupToken(context);
  }
  if (!context.options.apiKey) {
    return envelopeResult({
      data: {
        authenticated: false,
        credential_source: "none",
        setup_token_supported: true,
        next_actions: [
          "Open UniPost Dashboard API Keys and choose Connect with Claude Code / Codex.",
          `Run ${CLI_RUNNER} init --setup-token <token> --json once, or set UNIPOST_API_KEY for fallback auth.`,
        ],
      },
      warnings: [{
        code: "auth_setup_required",
        message: "Setup-token auth is available, but no setup token or stored credential was provided.",
      }],
      human: "No UniPost API key found. Generate a setup token from the dashboard and rerun init.\n",
      exitCode: EXIT.auth,
    });
  }

  const [{ workspace, response: workspaceResponse }, { profiles, pagination }] = await Promise.all([
    fetchWorkspace(context),
    fetchProfiles(context),
  ]);
  let config = await readConfig(context);
  const updates = {
    default_workspace_id: workspace?.id || config.default_workspace_id,
  };
  if (!config.default_profile_id && profiles.length === 1) {
    updates.default_profile_id = profiles[0].id;
  }
  config = await patchConfig(context, updates);

  const nextActions = [];
  if (profiles.length === 0) {
    nextActions.push("Run unipost profiles create --name \"Brand\".");
  } else if (!config.default_profile_id) {
    nextActions.push("Run unipost profiles use <profile_id>.");
  }
  nextActions.push("Run unipost quickstart to continue with Connect and draft creation.");

  return envelopeResult({
    data: {
      authenticated: true,
      credential_source: context.options.credentialSource,
      workspace,
      profiles,
      default_profile_id: config.default_profile_id || "",
      config_path: configPath(context),
      next_actions: nextActions,
    },
    meta: {
      request_id: workspaceResponse.requestId,
      pagination,
      rate_limit: workspaceResponse.rateLimit,
    },
    human: renderInit(workspace, profiles, config, nextActions),
  });
}

async function quickstart(context) {
  requireApiKey(context, "quickstart");
  const { workspace, response: workspaceResponse } = await fetchWorkspace(context);
  let { profiles } = await fetchProfiles(context);
  let profile = await selectProfile(context, profiles, { allowCreate: Boolean(context.options.name) });
  if (!profile && context.options.name) {
    profile = await createProfile(context, context.options.name);
    profiles = [profile, ...profiles];
  }
  if (profile?.id) {
    await patchConfig(context, {
      default_workspace_id: workspace?.id || "",
      default_profile_id: profile.id,
    });
  }
  const { accounts: accountList } = await fetchAccounts(context);
  const steps = [];
  if (!profile) {
    steps.push("Create a profile with unipost profiles create --name \"Brand\".");
  }
  if (profile && accountList.length === 0) {
    steps.push(`Connect an account with unipost connect create --platform linkedin --profile ${profile.id}.`);
  }
  if (accountList.length > 0) {
    const account = context.options.account
      ? accountList.find((item) => item.id === context.options.account)
      : accountList[0];
    if (account) {
      steps.push(`Validate a post with unipost posts validate --account ${account.id} --caption "Hello from UniPost".`);
      steps.push(`Create a draft with unipost posts draft --account ${account.id} --caption "Hello from UniPost".`);
    }
  }

  return envelopeResult({
    data: {
      workspace,
      profile: profile || null,
      profiles,
      accounts: accountList,
      live_publish_created: false,
      next_actions: steps,
    },
    meta: {
      request_id: workspaceResponse.requestId,
    },
    human: renderQuickstart(workspace, profile, accountList, steps),
  });
}

async function profiles(context, subcommand, id) {
  if (subcommand === "list" || !subcommand) {
    requireApiKey(context, "profiles list");
    const { profiles: profileList, pagination, response } = await fetchProfiles(context);
    return envelopeResult({
      data: { profiles: profileList },
      meta: {
        request_id: response.requestId,
        pagination,
        rate_limit: response.rateLimit,
      },
      human: renderProfiles(profileList),
    });
  }
  if (subcommand === "create") {
    requireApiKey(context, "profiles create");
    const name = requireValue(context.options.name, "--name <name>", "profiles create requires a name.");
    const profile = await createProfile(context, name);
    return envelopeResult({
      data: { profile },
      human: `Created profile ${profile.id} (${profile.name}).\n`,
    });
  }
  if (subcommand === "use") {
    const profileID = requireValue(id, "profile_id", "profiles use requires a profile ID.");
    const config = await patchConfig(context, { default_profile_id: profileID });
    return envelopeResult({
      data: {
        default_profile_id: config.default_profile_id,
        config_path: configPath(context),
      },
      human: `Default profile set to ${profileID}.\n`,
    });
  }
  if (subcommand === "get") {
    requireApiKey(context, "profiles get");
    const profileID = requireValue(id, "profile_id", "profiles get requires a profile ID.");
    const response = await requestJson(context, `/v1/profiles/${encodeURIComponent(profileID)}`, { auth: true });
    const profile = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { profile },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `${profile.id} ${profile.name || ""}\n`,
    });
  }
  return invalidSubcommand("profiles", subcommand);
}

async function createProfile(context, name) {
  const response = await requestJson(context, "/v1/profiles", {
    auth: true,
    method: "POST",
    body: { name: String(name).trim() },
  });
  return normalizeStatuses(unwrapData(response.body));
}

async function connect(context, subcommand, id) {
  if (subcommand === "create") {
    requireApiKey(context, "connect create");
    const platform = requireValue(context.options.platform, "--platform <platform>", "connect create requires a platform.");
    const profileID = await resolveProfileID(context);
    const body = {
      platform: platform.toLowerCase(),
      profile_id: profileID,
      external_user_id: context.options.externalUserId || "cli-local-user",
      ...(context.options.externalUserEmail ? { external_user_email: context.options.externalUserEmail } : {}),
      ...(context.options.returnUrl ? { return_url: context.options.returnUrl } : {}),
      allow_quickstart_creds: context.options.noQuickstartCreds ? false : true,
    };
    const response = await requestJson(context, "/v1/connect/sessions", {
      auth: true,
      method: "POST",
      body,
    });
    const session = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { session },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: renderConnectSession(session),
    });
  }
  if (subcommand === "get") {
    requireApiKey(context, "connect get");
    const sessionID = requireValue(id, "session_id", "connect get requires a session ID.");
    const response = await requestJson(context, `/v1/connect/sessions/${encodeURIComponent(sessionID)}`, { auth: true });
    const session = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { session },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: renderConnectSession(session),
    });
  }
  if (subcommand === "wait") {
    requireApiKey(context, "connect wait");
    const sessionID = requireValue(id, "session_id", "connect wait requires a session ID.");
    return waitForConnectSession(context, sessionID);
  }
  return invalidSubcommand("connect", subcommand);
}

async function waitForConnectSession(context, sessionID) {
  const timeoutSeconds = context.options.timeout ?? 300;
  const deadline = Date.now() + timeoutSeconds * 1000;
  let attempt = 0;
  let lastResponse = null;

  while (Date.now() <= deadline) {
    attempt += 1;
    const response = await requestJson(context, `/v1/connect/sessions/${encodeURIComponent(sessionID)}`, { auth: true });
    lastResponse = response;
    const session = normalizeStatuses(unwrapData(response.body));
    if (["completed", "expired", "canceled"].includes(session.status)) {
      return envelopeResult({
        data: { session, attempts: attempt },
        meta: { request_id: response.requestId, rate_limit: response.rateLimit },
        human: renderConnectSession(session),
      });
    }
    await sleep(pollDelayMs(context, attempt, response.response.headers));
  }

  throw new CliError({
    code: "timeout",
    normalizedCode: "timeout",
    message: `Timed out waiting for connect session ${sessionID}.`,
    hint: "Increase --timeout or retry connect wait later.",
    exitCode: EXIT.timeout,
    requestId: lastResponse?.requestId,
    status: lastResponse?.response?.status,
  });
}

async function accounts(context, subcommand, id) {
  if (subcommand === "list" || !subcommand) {
    requireApiKey(context, "accounts list");
    const { accounts: accountList, pagination, response } = await fetchAccounts(context);
    const filtered = filterAccounts(accountList, context.options);
    return envelopeResult({
      data: { accounts: filtered },
      meta: {
        request_id: response.requestId,
        pagination,
        rate_limit: response.rateLimit,
      },
      human: renderAccounts(filtered),
    });
  }
  if (subcommand === "get") {
    requireApiKey(context, "accounts get");
    const accountID = requireValue(id, "account_id", "accounts get requires an account ID.");
    const { accounts: accountList, response } = await fetchAccounts(context);
    const account = accountList.find((item) => item.id === accountID);
    if (!account) {
      throw new CliError({
        code: "not_found",
        normalizedCode: "not_found",
        message: `Account ${accountID} was not found in accounts list.`,
        exitCode: EXIT.validation,
      });
    }
    return envelopeResult({
      data: { account },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: renderAccounts([account]),
    });
  }
  if (subcommand === "health" || subcommand === "capabilities" || subcommand === "metrics") {
    requireApiKey(context, `accounts ${subcommand}`);
    const accountID = requireAccountID(context, id, `accounts ${subcommand}`);
    const response = await requestJson(context, `/v1/accounts/${encodeURIComponent(accountID)}/${subcommand}`, { auth: true });
    const payload = normalizeStatuses(unwrapData(response.body));
    const dataKey = subcommand === "capabilities" ? "capabilities" : subcommand;
    return envelopeResult({
      data: { [dataKey]: payload },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: renderAccountDiagnostic(subcommand, accountID, payload),
    });
  }
  return invalidSubcommand("accounts", subcommand);
}

async function posts(context, subcommand, id) {
  if (subcommand === "list" || !subcommand) {
    requireApiKey(context, "posts list");
    const response = await requestJson(context, apiPath("/v1/posts", {
      status: context.options.status,
      limit: context.options.limit,
      cursor: context.options.cursor,
    }), { auth: true });
    const postList = normalizeStatuses(unwrapData(response.body) || []);
    return envelopeResult({
      data: { posts: postList },
      meta: {
        request_id: response.requestId,
        pagination: paginationFromBody(response.body),
        rate_limit: response.rateLimit,
      },
      human: renderPosts(postList),
    });
  }
  if (subcommand === "get") {
    requireApiKey(context, "posts get");
    const postID = requireValue(id, "post_id", "posts get requires a post ID.");
    const response = await requestJson(context, `/v1/posts/${encodeURIComponent(postID)}`, { auth: true });
    const post = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { post },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: renderPost(post),
    });
  }
  if (subcommand === "analytics") {
    requireApiKey(context, "posts analytics");
    const postID = requireValue(id, "post_id", "posts analytics requires a post ID.");
    const response = await requestJson(context, `/v1/posts/${encodeURIComponent(postID)}/analytics`, { auth: true });
    const analyticsData = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { analytics: analyticsData },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `Analytics for ${postID}.\n`,
    });
  }
  if (subcommand === "validate") {
    requireApiKey(context, "posts validate");
    const payload = await postPayloadFromOptions(context);
    const response = await requestJson(context, "/v1/posts/validate", {
      auth: true,
      method: "POST",
      body: payload,
    });
    const validation = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { validation },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: validation?.valid ? "Post is valid.\n" : "Post validation returned issues.\n",
    });
  }
  if (subcommand === "draft") {
    requireApiKey(context, "posts draft");
    const payload = { ...(await postPayloadFromOptions(context)), status: "draft" };
    const response = await requestJson(context, "/v1/posts", {
      auth: true,
      method: "POST",
      body: payload,
    });
    const post = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { post },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `Created draft ${post.id || "post"}.\n`,
    });
  }
  if (subcommand === "create") {
    requireApiKey(context, "posts create");
    const payload = await postPayloadFromOptions(context);
    if (context.options.dryRun) {
      const response = await requestJson(context, "/v1/posts/validate", {
        auth: true,
        method: "POST",
        body: payload,
      });
      const validation = normalizeStatuses(unwrapData(response.body));
      return envelopeResult({
        data: { dry_run: true, payload, validation },
        meta: { request_id: response.requestId, rate_limit: response.rateLimit },
        human: validation?.valid ? "Post dry run is valid.\n" : "Post dry run returned issues.\n",
      });
    }
    if (payload.status !== "draft") {
      requirePublishConfirmation(context, payload, "posts create");
    }
    const response = await requestJson(context, "/v1/posts", {
      auth: true,
      method: "POST",
      body: payload,
    });
    const post = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { post },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `Created post ${post.id || "post"}.\n`,
    });
  }
  if (subcommand === "schedule") {
    requireApiKey(context, "posts schedule");
    const payload = await postPayloadFromOptions(context);
    if (!payload.scheduled_at) {
      throw new CliError({
        code: "missing_required_input",
        normalizedCode: "missing_required_input",
        message: "posts schedule requires --at or scheduled_at in --from-file.",
        hint: "Pass --at <RFC3339 timestamp>.",
        exitCode: EXIT.missingInput,
      });
    }
    requirePublishConfirmation(context, payload, "posts schedule");
    const response = await requestJson(context, "/v1/posts", {
      auth: true,
      method: "POST",
      body: payload,
    });
    const post = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { post },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `Scheduled post ${post.id || "post"}.\n`,
    });
  }
  if (subcommand === "publish-draft") {
    requireApiKey(context, "posts publish-draft");
    const postID = requireValue(id, "post_id", "posts publish-draft requires a post ID.");
    requirePublishConfirmation(context, {}, "posts publish-draft");
    const response = await requestJson(context, `/v1/posts/${encodeURIComponent(postID)}/publish`, {
      auth: true,
      method: "POST",
    });
    const post = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { post },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `Queued draft ${postID} for publishing.\n`,
    });
  }
  if (subcommand === "wait") {
    requireApiKey(context, "posts wait");
    const postID = requireValue(id, "post_id", "posts wait requires a post ID.");
    return waitForPost(context, postID);
  }
  if (subcommand === "cancel") {
    requireApiKey(context, "posts cancel");
    const postID = requireValue(id, "post_id", "posts cancel requires a post ID.");
    requireDestructiveConfirmation(context, "posts cancel");
    const response = await requestJson(context, `/v1/posts/${encodeURIComponent(postID)}/cancel`, {
      auth: true,
      method: "POST",
    });
    const post = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { post },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `Canceled post ${post.id || postID}.\n`,
    });
  }
  if (subcommand === "retry") {
    requireApiKey(context, "posts retry");
    const postID = requireValue(id, "post_id", "posts retry requires a post ID.");
    const resultID = requireValue(context.options.result, "--result <result_id>", "posts retry requires --result.");
    requireDestructiveConfirmation(context, "posts retry");
    const response = await requestJson(context, `/v1/posts/${encodeURIComponent(postID)}/results/${encodeURIComponent(resultID)}/retry`, {
      auth: true,
      method: "POST",
    });
    const retry = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { retry },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `Retried result ${resultID} for post ${postID}.\n`,
    });
  }
  return invalidSubcommand("posts", subcommand);
}

async function postPayloadFromOptions(context) {
  let payload = {};
  if (context.options.fromFile) {
    payload = await readJsonFile(context, context.options.fromFile);
  } else {
    const caption = requireValue(context.options.caption, "--caption <text>", "posts command requires a caption.");
    const accountIDs = splitIDs(requireValue(context.options.account, "--account <account_id>", "posts command requires at least one account."));
    payload = {
      caption,
      account_ids: accountIDs,
    };
  }
  if (context.options.account) {
    payload.account_ids = splitIDs(context.options.account);
  }
  if (context.options.caption) {
    payload.caption = context.options.caption;
  }
  const scheduledAt = context.options.scheduleAt || context.options.at;
  if (scheduledAt) {
    payload.scheduled_at = scheduledAt;
  }
  return payload;
}

async function readJsonFile(context, filePath) {
  try {
    const raw = await readFile(filePath, "utf8");
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      throw new Error("expected a JSON object");
    }
    return parsed;
  } catch (error) {
    throw new CliError({
      code: "invalid_request",
      normalizedCode: "invalid_request",
      message: `Failed to read JSON from ${filePath}: ${error.message}`,
      hint: "Pass --from-file with a JSON object matching the UniPost post request shape.",
      exitCode: EXIT.validation,
      cause: error,
    });
  }
}

function requirePublishConfirmation(context, payload, action) {
  if (!context.options.yes) {
    throw new CliError({
      code: "unsafe_action_blocked",
      normalizedCode: "unsafe_action_blocked",
      message: `${action} can publish to external social networks and requires --yes.`,
      hint: "Ask the user to approve the publish action, then rerun with --yes.",
      exitCode: EXIT.unsafe,
    });
  }
  if (!context.options.idempotencyKey && !payload?.idempotency_key) {
    throw new CliError({
      code: "missing_required_input",
      normalizedCode: "missing_required_input",
      message: `${action} requires --idempotency-key for publish-capable writes.`,
      hint: "Pass --idempotency-key <stable-key> after explicit user approval.",
      exitCode: EXIT.missingInput,
    });
  }
}

function requireDestructiveConfirmation(context, action) {
  if (context.options.yes) {
    return;
  }
  throw new CliError({
    code: "unsafe_action_blocked",
    normalizedCode: "unsafe_action_blocked",
    message: `${action} requires --yes because it changes an existing post or delivery result.`,
    hint: "Ask the user to approve the exact post/result ID, then rerun with --yes.",
    exitCode: EXIT.unsafe,
  });
}

async function waitForPost(context, postID) {
  const timeoutSeconds = context.options.timeout ?? 120;
  const deadline = Date.now() + timeoutSeconds * 1000;
  let attempt = 0;
  let lastResponse = null;

  while (Date.now() <= deadline) {
    attempt += 1;
    const response = await requestJson(context, `/v1/posts/${encodeURIComponent(postID)}`, { auth: true });
    lastResponse = response;
    const post = normalizeStatuses(unwrapData(response.body));
    if (TERMINAL_POST_STATUSES.has(post?.status)) {
      return envelopeResult({
        data: { post, attempts: attempt },
        meta: { request_id: response.requestId, rate_limit: response.rateLimit },
        human: renderPost(post),
      });
    }
    await sleep(pollDelayMs(context, attempt, response.response.headers));
  }

  throw new CliError({
    code: "timeout",
    normalizedCode: "timeout",
    message: `Timed out waiting for post ${postID}.`,
    hint: "Increase --timeout or check the post later with unipost posts get.",
    exitCode: EXIT.timeout,
    requestId: lastResponse?.requestId,
    status: lastResponse?.response?.status,
  });
}

async function uploadMedia(context, filePath) {
  const fileInfo = await mediaFileInfo(filePath);
  const contentType = context.options.contentType || inferContentType(filePath);
  if (!contentType) {
    throw new CliError({
      code: "missing_required_input",
      normalizedCode: "missing_required_input",
      message: "media upload could not infer content type from the file extension.",
      hint: "Pass --content-type <mime>, such as --content-type video/mp4.",
      exitCode: EXIT.missingInput,
    });
  }

  const createResponse = await requestJson(context, "/v1/media", {
    auth: true,
    method: "POST",
    body: {
      filename: basename(filePath),
      content_type: contentType,
      size_bytes: fileInfo.sizeBytes,
      content_hash: fileInfo.contentHash,
    },
  });
  const reserved = normalizeStatuses(unwrapData(createResponse.body));

  if (!reserved?.id) {
    throw new CliError({
      code: "invalid_response",
      normalizedCode: "invalid_response",
      message: "Media reserve response did not include a media ID.",
      hint: "Capture request_id and contact support.",
      exitCode: EXIT.upstream,
      requestId: createResponse.requestId,
      status: createResponse.response.status,
    });
  }

  if (mediaReady(reserved) || reserved.status === "failed") {
    return mediaEnvelope(context, reserved, 0, createResponse);
  }

  const uploadURL = reserved.upload_url;
  if (!uploadURL) {
    throw new CliError({
      code: "invalid_response",
      normalizedCode: "invalid_response",
      message: "Media reserve response did not include an upload URL.",
      hint: "Retry media upload or contact support with the request_id.",
      exitCode: EXIT.upstream,
      requestId: createResponse.requestId,
      status: createResponse.response.status,
    });
  }

  await uploadBytes(context, uploadURL, filePath, contentType);
  return waitForMedia(context, reserved.id);
}

async function mediaFileInfo(filePath) {
  try {
    const fileStat = await stat(filePath);
    if (!fileStat.isFile()) {
      throw new Error("path is not a regular file");
    }
    return {
      sizeBytes: fileStat.size,
      contentHash: await hashFile(filePath),
    };
  } catch (error) {
    throw new CliError({
      code: "invalid_request",
      normalizedCode: "invalid_request",
      message: `Failed to read media file ${filePath}: ${error.message}`,
      hint: "Pass a readable local image or video path.",
      exitCode: EXIT.validation,
      cause: error,
    });
  }
}

async function hashFile(filePath) {
  const hash = createHash("sha256");
  await new Promise((resolve, reject) => {
    const stream = createReadStream(filePath);
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("error", reject);
    stream.on("end", resolve);
  });
  return hash.digest("hex");
}

async function uploadBytes(context, uploadURL, filePath, contentType) {
  let response;
  try {
    response = await context.fetchImpl(uploadURL, {
      method: "PUT",
      headers: {
        "Content-Type": contentType,
      },
      body: createReadStream(filePath),
      duplex: "half",
    });
  } catch (error) {
    throw new CliError({
      code: "network_error",
      normalizedCode: "network_error",
      message: `Media upload failed: ${error.message}`,
      hint: "Retry media upload; the reserve URL may have expired.",
      exitCode: EXIT.network,
      cause: error,
    });
  }

  if (!response.ok) {
    throw new CliError({
      code: "upstream_error",
      normalizedCode: "upstream_error",
      message: `Media upload target returned HTTP ${response.status}.`,
      hint: "Retry media upload; if it repeats, capture request_id and contact support.",
      exitCode: EXIT.upstream,
      status: response.status,
    });
  }
}

async function waitForMedia(context, mediaID) {
  const timeoutSeconds = context.options.timeout ?? 120;
  const deadline = Date.now() + timeoutSeconds * 1000;
  let attempt = 0;
  let lastResponse = null;

  while (Date.now() <= deadline) {
    attempt += 1;
    const response = await requestJson(context, `/v1/media/${encodeURIComponent(mediaID)}`, { auth: true });
    lastResponse = response;
    const mediaItem = normalizeStatuses(unwrapData(response.body));
    if (TERMINAL_MEDIA_STATUSES.has(mediaItem?.status)) {
      return mediaEnvelope(context, mediaItem, attempt, response);
    }
    await sleep(pollDelayMs(context, attempt, response.response.headers));
  }

  throw new CliError({
    code: "timeout",
    normalizedCode: "timeout",
    message: `Timed out waiting for media ${mediaID}.`,
    hint: "Increase --timeout or check the media later with unipost media get.",
    exitCode: EXIT.timeout,
    requestId: lastResponse?.requestId,
    status: lastResponse?.response?.status,
  });
}

function mediaEnvelope(context, mediaItem, attempts, response) {
  return envelopeResult({
    data: {
      media: mediaItem,
      ready: mediaReady(mediaItem),
      ...(attempts ? { attempts } : {}),
      next_publish_hint: mediaReady(mediaItem) ? `Use media_id ${mediaItem.id} in posts create --from-file or post.json media_ids.` : "",
    },
    meta: { request_id: response.requestId, rate_limit: response.rateLimit },
    human: renderMedia(mediaItem),
  });
}

function mediaReady(mediaItem) {
  return mediaItem?.status === "ready";
}

function inferContentType(filePath) {
  const ext = extname(filePath).toLowerCase();
  return {
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".png": "image/png",
    ".webp": "image/webp",
    ".gif": "image/gif",
    ".heic": "image/heic",
    ".mp4": "video/mp4",
    ".mov": "video/quicktime",
    ".qt": "video/quicktime",
    ".webm": "video/webm",
    ".m4v": "video/x-m4v",
  }[ext] || "";
}

async function media(context, subcommand, id) {
  if (subcommand === "upload") {
    requireApiKey(context, "media upload");
    const filePath = requireValue(id, "file_path", "media upload requires a local file path.");
    return uploadMedia(context, filePath);
  }
  if (subcommand === "get") {
    requireApiKey(context, "media get");
    const mediaID = requireValue(id, "media_id", "media get requires a media ID.");
    const response = await requestJson(context, `/v1/media/${encodeURIComponent(mediaID)}`, { auth: true });
    const mediaItem = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { media: mediaItem },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `${mediaItem.id || mediaID} ${mediaItem.status || ""}\n`,
    });
  }
  if (subcommand === "wait") {
    requireApiKey(context, "media wait");
    const mediaID = requireValue(id, "media_id", "media wait requires a media ID.");
    return waitForMedia(context, mediaID);
  }
  return invalidSubcommand("media", subcommand);
}

async function analytics(context, subcommand, id) {
  const command = subcommand || "summary";
  if (command === "summary") {
    requireApiKey(context, "analytics summary");
    const response = await requestJson(context, apiPath("/v1/analytics/summary", dateRangeQuery(context)), { auth: true });
    const summary = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { summary },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: "Analytics summary loaded.\n",
    });
  }
  if (command === "posts") {
    requireApiKey(context, "analytics posts");
    const response = await requestJson(context, apiPath("/v1/analytics/posts", {
      ...dateRangeQuery(context),
      limit: context.options.limit,
      cursor: context.options.cursor,
    }), { auth: true });
    const postsData = normalizeStatuses(unwrapData(response.body) || []);
    return envelopeResult({
      data: { posts: postsData },
      meta: {
        request_id: response.requestId,
        pagination: paginationFromBody(response.body),
        rate_limit: response.rateLimit,
      },
      human: `Loaded analytics for ${postsData.length || 0} posts.\n`,
    });
  }
  if (command === "platforms") {
    requireApiKey(context, "analytics platforms");
    const response = await requestJson(context, "/v1/analytics/platforms", { auth: true });
    const platforms = normalizeStatuses(unwrapData(response.body) || []);
    return envelopeResult({
      data: { platforms },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `Loaded ${platforms.length || 0} analytics platforms.\n`,
    });
  }
  if (command === "platform") {
    requireApiKey(context, "analytics platform");
    const platform = requireValue(id, "platform", "analytics platform requires a platform name.");
    const response = await requestJson(context, apiPath(`/v1/analytics/platforms/${encodeURIComponent(platform)}`, dateRangeQuery(context)), { auth: true });
    const platformData = normalizeStatuses(unwrapData(response.body));
    return envelopeResult({
      data: { platform: platformData },
      meta: { request_id: response.requestId, rate_limit: response.rateLimit },
      human: `Loaded analytics for ${platform}.\n`,
    });
  }
  return invalidSubcommand("analytics", subcommand);
}

function dateRangeQuery(context) {
  return {
    from: context.options.from,
    to: context.options.to,
  };
}

async function examples(context, subcommand) {
  if (subcommand === "mcp.claude-code") {
    const content = claudeCodeMcpExample();
    return envelopeResult({
      data: {
        example: "mcp.claude-code",
        client: "claude-code",
        content,
        auth_test_command: `${CLI_RUNNER} agent mcp-test --json`,
      },
      human: `${content}\n`,
    });
  }
  if (subcommand !== "posts.create") {
    return invalidSubcommand("examples", subcommand);
  }
  const language = (context.options.lang || "curl").toLowerCase();
  if (!["curl", "node"].includes(language)) {
    throw new CliError({
      code: "invalid_argument",
      normalizedCode: "invalid_argument",
      message: "examples posts.create supports --lang curl or --lang node in Phase 2.",
      exitCode: EXIT.invalidArgs,
    });
  }
  const caption = context.options.caption || "Hello from UniPost";
  const accountIDs = splitIDs(context.options.account || "sa_your_account_id");
  const payload = {
    caption,
    account_ids: accountIDs,
  };
  const code = language === "node"
    ? nodeFetchPostExample(context, payload)
    : curlPostExample(context, payload);
  return envelopeResult({
    data: {
      language,
      code,
      sdk_dependency_required: false,
    },
    human: `${code}\n`,
  });
}

async function agent(context, subcommand, third) {
  if (subcommand === "plan") {
    const intent = requireValue(context.options.intent || third, "--intent <intent>", "agent plan requires --intent.");
    return agentPlan(context, intent);
  }
  if (subcommand === "plan-publish") {
    return agentPlan(context, "plan_publish_post");
  }
  if (subcommand === "execute") {
    return agentExecute(context);
  }
  if (subcommand === "capabilities") {
    return agentCapabilities(context);
  }
  if (subcommand === "context") {
    requireApiKey(context, "agent context");
    const data = await agentContextData(context);
    return envelopeResult({
      data,
      human: `Workspace ${data.workspace?.id || "unknown"}: ${data.profiles.length} profiles, ${data.accounts.length} accounts.\n`,
    });
  }
  if (subcommand === "bootstrap") {
    return agentBootstrap(context);
  }
  if (subcommand === "guide") {
    const client = context.options.client || third || "codex";
    const guide = agentGuide(client);
    return envelopeResult({
      data: guide,
      human: `${guide.recommended_prompt}\n`,
    });
  }
  if (subcommand === "mcp-config") {
    const client = context.options.client || third || "claude-code";
    const config = agentMcpConfig(context, client);
    return envelopeResult({
      data: config,
      human: `${config.content}\n`,
    });
  }
  if (subcommand === "mcp-test") {
    return agentMcpTest(context);
  }
  if (subcommand === "install") {
    const client = context.options.client || third || "codex";
    const install = agentInstallInstructions(client);
    return envelopeResult({
      data: install,
      human: `${install.instructions}\n`,
    });
  }
  return invalidSubcommand("agent", subcommand);
}

async function agentPlan(context, intent) {
  const normalizedIntent = String(intent || "").trim();
  const input = await agentPlanInput(context);

  if (normalizedIntent === "diagnose_setup") {
    return envelopeResult({
      data: {
        catalog_version: AGENT_CATALOG_VERSION,
        intent: normalizedIntent,
        safety_level: "read_only",
        missing_inputs: [],
        required_user_confirmations: [],
        safe_to_execute_without_user: true,
        actions: [{
          canonical_action: "agent.bootstrap",
          command: `${CLI_RUNNER} agent bootstrap`,
          args: { "--json": true },
          safety_level: "read_only",
        }],
      },
      human: "Agent plan: run bootstrap diagnostics.\n",
    });
  }

  if (normalizedIntent === "diagnose_account") {
    const accountID = input.account_id || context.options.account || "";
    const missing = missingInputs({ account_id: accountID });
    return envelopeResult({
      data: {
        catalog_version: AGENT_CATALOG_VERSION,
        intent: normalizedIntent,
        safety_level: "read_only",
        input: compactObject({ account_id: accountID }),
        missing_inputs: missing,
        required_user_confirmations: [],
        safe_to_execute_without_user: missing.length === 0,
        actions: missing.length === 0 ? [
          {
            canonical_action: "accounts.health",
            command: "unipost accounts health",
            args: compactObject({ "--account": accountID, "--json": true }),
            safety_level: "read_only",
            safe_to_execute_without_user: true,
          },
          {
            canonical_action: "accounts.capabilities",
            command: "unipost accounts capabilities",
            args: compactObject({ "--account": accountID, "--json": true }),
            safety_level: "read_only",
            safe_to_execute_without_user: true,
          },
          {
            canonical_action: "accounts.metrics",
            command: "unipost accounts metrics",
            args: compactObject({ "--account": accountID, "--json": true }),
            safety_level: "read_only",
            safe_to_execute_without_user: true,
          },
        ] : [],
      },
      human: missing.length === 0 ? "Agent plan: read account diagnostics.\n" : "Agent plan has missing inputs.\n",
    });
  }

  if (normalizedIntent === "create_draft_post") {
    const accountIDs = agentAccountIDs(input, context);
    const caption = agentCaption(input, context);
    const missing = missingInputs({ account_ids: accountIDs, caption });
    return envelopeResult({
      data: {
        catalog_version: AGENT_CATALOG_VERSION,
        intent: normalizedIntent,
        safety_level: "draft_write",
        input: compactObject({ account_ids: accountIDs, caption }),
        missing_inputs: missing,
        required_user_confirmations: [],
        safe_to_execute_without_user: missing.length === 0,
        actions: missing.length === 0 ? [
          {
            canonical_action: "posts.validate",
            command: "unipost posts validate",
            args: postArgs(accountIDs, caption, { json: true }),
            safety_level: "read_only",
          },
          {
            canonical_action: "posts.draft",
            command: "unipost posts draft",
            args: postArgs(accountIDs, caption, { json: true }),
            safety_level: "draft_write",
          },
        ] : [],
      },
      human: missing.length === 0 ? "Agent plan: validate then create a draft.\n" : "Agent plan has missing inputs.\n",
    });
  }

  if (normalizedIntent === "plan_publish_post") {
    const accountIDs = agentAccountIDs(input, context);
    const caption = agentCaption(input, context);
    const scheduledAt = input.scheduled_at || context.options.scheduleAt || context.options.at || "";
    const missing = missingInputs({ account_ids: accountIDs, caption });
    const validateArgs = agentPostArgs(context, accountIDs, caption, {
      json: true,
      scheduledAt,
    });
    const dryRunArgs = agentPostArgs(context, accountIDs, caption, {
      json: true,
      scheduledAt,
      dryRun: true,
    });
    const createArgs = agentPostArgs(context, accountIDs, caption, {
      json: true,
      yes: "<required_after_user_confirmation>",
      idempotencyKey: input.idempotency_key || "<required_stable_key>",
      scheduledAt,
    });
    return envelopeResult({
      data: {
        catalog_version: AGENT_CATALOG_VERSION,
        intent: normalizedIntent,
        safety_level: "live_write",
        input: compactObject({ account_ids: accountIDs, caption, scheduled_at: scheduledAt }),
        missing_inputs: missing,
        required_user_confirmations: ["approve_live_publish"],
        safe_to_execute_without_user: false,
        actions: missing.length === 0 ? [
          {
            canonical_action: "posts.validate",
            command: "unipost posts validate",
            args: validateArgs,
            safety_level: "read_only",
            safe_to_execute_without_user: true,
          },
          {
            canonical_action: "posts.create_dry_run",
            command: "unipost posts create",
            args: dryRunArgs,
            safety_level: "read_only",
            safe_to_execute_without_user: true,
          },
          {
            canonical_action: "posts.create",
            command: "unipost posts create",
            args: createArgs,
            safety_level: "live_write",
            required_user_confirmations: ["approve_live_publish"],
            safe_to_execute_without_user: false,
          },
        ] : [],
      },
      human: missing.length === 0 ? "Agent plan: validate, then publish only after explicit confirmation.\n" : "Agent plan has missing inputs.\n",
    });
  }

  if (normalizedIntent === "connect_account") {
    const platform = input.platform || context.options.platform || "";
    const missing = missingInputs({ platform });
    return envelopeResult({
      data: {
        catalog_version: AGENT_CATALOG_VERSION,
        intent: normalizedIntent,
        safety_level: "setup_write",
        input: compactObject({ platform }),
        missing_inputs: missing,
        required_user_confirmations: [],
        safe_to_execute_without_user: missing.length === 0,
        actions: missing.length === 0 ? [{
          canonical_action: "connect.create",
          command: "unipost connect create",
          args: compactObject({ "--platform": platform, "--json": true }),
          safety_level: "setup_write",
        }] : [],
      },
      human: missing.length === 0 ? "Agent plan: create a connect session.\n" : "Agent plan has missing inputs.\n",
    });
  }

  if (normalizedIntent === "upload_media") {
    const filePath = input.file_path || "";
    const contentType = input.content_type || context.options.contentType || "";
    const missing = missingInputs({ file_path: filePath });
    return envelopeResult({
      data: {
        catalog_version: AGENT_CATALOG_VERSION,
        intent: normalizedIntent,
        safety_level: "setup_write",
        input: compactObject({ file_path: filePath, content_type: contentType }),
        missing_inputs: missing,
        required_user_confirmations: ["approve_local_file_upload"],
        safe_to_execute_without_user: false,
        actions: missing.length === 0 ? [
          {
            canonical_action: "media.upload",
            command: "unipost media upload",
            args: compactObject({ file_path: filePath, "--content-type": contentType, "--json": true }),
            safety_level: "setup_write",
            required_user_confirmations: ["approve_local_file_upload"],
            safe_to_execute_without_user: false,
          },
          {
            canonical_action: "media.wait",
            command: "unipost media wait",
            args: { media_id: "<media_id_from_upload>", "--json": true },
            safety_level: "read_only",
            safe_to_execute_without_user: true,
          },
        ] : [],
      },
      human: missing.length === 0 ? "Agent plan: upload local media after user confirmation, then wait for readiness.\n" : "Agent plan has missing inputs.\n",
    });
  }

  throw new CliError({
    code: "invalid_argument",
    normalizedCode: "invalid_argument",
    message: `Unsupported agent intent: ${normalizedIntent}`,
    hint: `Run ${CLI_RUNNER} agent capabilities --json to list supported intents.`,
    docsUrl: DOCS_CLI_URL,
    exitCode: EXIT.invalidArgs,
  });
}

async function agentPlanInput(context) {
  const fromFile = context.options.fromFile ? await readJsonFile(context, context.options.fromFile) : {};
  return compactObject({
    ...fromFile,
    ...(context.options.account ? { account_ids: splitIDs(context.options.account) } : {}),
    ...(context.options.caption ? { caption: context.options.caption } : {}),
    ...(context.options.platform ? { platform: context.options.platform } : {}),
    ...(context.options.scheduleAt ? { scheduled_at: context.options.scheduleAt } : {}),
    ...(context.options.at ? { scheduled_at: context.options.at } : {}),
    ...(context.options.idempotencyKey ? { idempotency_key: context.options.idempotencyKey } : {}),
  });
}

function agentAccountIDs(input) {
  if (Array.isArray(input.account_ids) && input.account_ids.length > 0) {
    return input.account_ids.map(String).filter(Boolean);
  }
  if (Array.isArray(input.platform_posts)) {
    return input.platform_posts.map((post) => post?.account_id).filter(Boolean);
  }
  return [];
}

function agentCaption(input) {
  if (typeof input.caption === "string" && input.caption.trim()) {
    return input.caption.trim();
  }
  if (Array.isArray(input.platform_posts)) {
    const first = input.platform_posts.find((post) => typeof post?.caption === "string" && post.caption.trim());
    return first?.caption?.trim() || "";
  }
  return "";
}

function missingInputs(values) {
  return Object.entries(values)
    .filter(([, value]) => Array.isArray(value) ? value.length === 0 : !value)
    .map(([key]) => key);
}

function postArgs(accountIDs, caption, options = {}) {
  return compactObject({
    "--account": accountIDs.join(","),
    "--caption": caption,
    ...(options.scheduledAt ? { "--schedule-at": options.scheduledAt } : {}),
    ...(options.dryRun ? { "--dry-run": true } : {}),
    ...(options.yes ? { "--yes": options.yes } : {}),
    ...(options.idempotencyKey ? { "--idempotency-key": options.idempotencyKey } : {}),
    ...(options.json ? { "--json": true } : {}),
  });
}

function agentPostArgs(context, accountIDs, caption, options = {}) {
  if (!context.options.fromFile) {
    return postArgs(accountIDs, caption, options);
  }
  return compactObject({
    "--from-file": context.options.fromFile,
    ...(options.scheduledAt ? { "--schedule-at": options.scheduledAt } : {}),
    ...(options.dryRun ? { "--dry-run": true } : {}),
    ...(options.yes ? { "--yes": options.yes } : {}),
    ...(options.idempotencyKey ? { "--idempotency-key": options.idempotencyKey } : {}),
    ...(options.json ? { "--json": true } : {}),
  });
}

function compactObject(value) {
  return Object.fromEntries(Object.entries(value).filter(([, item]) => {
    if (item === undefined || item === null || item === "") {
      return false;
    }
    if (Array.isArray(item) && item.length === 0) {
      return false;
    }
    return true;
  }));
}

function agentCapabilities() {
  return envelopeResult({
    data: {
      catalog_version: AGENT_CATALOG_VERSION,
      status_enums: {
        post: ["draft", "scheduled", "publishing", "published", "partial", "failed", "canceled"],
        connect_session: ["pending", "completed", "expired", "canceled"],
        media: ["pending", "processing", "ready", "failed"],
      },
      commands: [
        "init",
        "quickstart",
        "config path",
        "config show",
        "config set",
        "auth login",
        "auth logout",
        "auth status",
        "auth list",
        "auth use",
        "profiles list",
        "profiles create",
        "profiles use",
        "connect create",
        "connect get",
        "connect wait",
        "accounts list",
        "accounts get",
        "accounts health",
        "accounts capabilities",
        "accounts metrics",
        "posts list",
        "posts get",
        "posts analytics",
        "posts validate",
        "posts draft",
        "posts create",
        "posts schedule",
        "posts publish-draft",
        "posts wait",
        "posts cancel",
        "posts retry",
        "media upload",
        "media get",
        "media wait",
        "analytics summary",
        "analytics posts",
        "analytics platforms",
        "analytics platform",
        "examples posts.create",
        "examples mcp.claude-code",
        "agent plan",
        "agent plan-publish",
        "agent execute",
        "agent bootstrap",
        "agent capabilities",
        "agent context",
        "agent guide",
        "agent mcp-config",
        "agent mcp-test",
        "agent install",
      ],
      intents: [
        {
          name: "diagnose_setup",
          description: "Check auth, workspace, profile and account readiness for an agent.",
          command: `${CLI_RUNNER} agent bootstrap --json`,
          canonical_action: "agent.bootstrap",
          safety_level: "read_only",
          required_inputs: [],
          optional_inputs: ["client"],
          input_schema: {
            type: "object",
            properties: {
              client: { type: "string", enum: ["codex", "claude-code", "cursor", "windsurf"] },
            },
            additionalProperties: false,
          },
        },
        {
          name: "diagnose_account",
          description: "Read account health, capabilities and metrics for support diagnostics.",
          command: `${CLI_RUNNER} accounts health --account <account_id> --json`,
          canonical_action: "accounts.diagnose",
          safety_level: "read_only",
          required_inputs: ["account_id"],
          optional_inputs: [],
          input_schema: {
            type: "object",
            required: ["account_id"],
            properties: {
              account_id: { type: "string", minLength: 1 },
            },
            additionalProperties: false,
          },
          canonical_actions: ["accounts.health", "accounts.capabilities", "accounts.metrics"],
        },
        {
          name: "create_draft_post",
          description: "Validate copy and create a UniPost draft without publishing externally.",
          command: `${CLI_RUNNER} posts draft --account <account_id> --caption <text> --json`,
          canonical_action: "posts.draft",
          safety_level: "draft_write",
          required_inputs: ["account_ids", "caption"],
          optional_inputs: [],
          input_schema: {
            type: "object",
            required: ["account_ids", "caption"],
            properties: {
              account_ids: { type: "array", items: { type: "string" }, minItems: 1 },
              caption: { type: "string", minLength: 1 },
            },
            additionalProperties: false,
          },
          preflight: `Run ${CLI_RUNNER} posts validate with the same account_ids and caption before creating a draft.`,
        },
        {
          name: "plan_publish_post",
          description: "Plan a live or scheduled publish flow without executing the write.",
          command: `${CLI_RUNNER} agent plan --intent plan_publish_post --from-file post.json --json`,
          canonical_action: "agent.plan_publish_post",
          safety_level: "live_write_plan",
          required_inputs: ["account_ids", "caption"],
          optional_inputs: ["scheduled_at", "media_ids", "platform_posts", "idempotency_key"],
          input_schema: {
            type: "object",
            properties: {
              account_ids: { type: "array", items: { type: "string" }, minItems: 1 },
              caption: { type: "string", minLength: 1 },
              scheduled_at: { type: "string" },
              media_ids: { type: "array", items: { type: "string" } },
              platform_posts: { type: "array", items: { type: "object" } },
              idempotency_key: { type: "string" },
            },
            additionalProperties: true,
          },
          required_user_confirmations: ["approve_live_publish"],
          canonical_actions: ["posts.validate", "posts.create_dry_run", "posts.create"],
          guardrails: [
            "posts create and posts schedule require --yes plus --idempotency-key before publish-capable writes.",
            "Use posts create --from-file --dry-run before requesting publish approval.",
          ],
        },
        {
          name: "connect_account",
          description: "Create a hosted OAuth connect session for a social platform.",
          command: "unipost connect create --platform <platform> --json",
          canonical_action: "connect.create",
          safety_level: "setup_write",
          required_inputs: ["platform"],
          optional_inputs: ["profile_id", "return_url", "external_user_id", "external_user_email"],
          input_schema: {
            type: "object",
            required: ["platform"],
            properties: {
              platform: { type: "string", minLength: 1 },
              profile_id: { type: "string" },
              return_url: { type: "string" },
              external_user_id: { type: "string" },
              external_user_email: { type: "string" },
            },
            additionalProperties: false,
          },
        },
        {
          name: "upload_media",
          description: "Reserve and upload a local image or video, then wait until the media ID is publish-ready.",
          command: "unipost media upload <path> --json",
          canonical_action: "media.upload",
          safety_level: "setup_write",
          required_inputs: ["file_path"],
          optional_inputs: ["content_type"],
          input_schema: {
            type: "object",
            required: ["file_path"],
            properties: {
              file_path: { type: "string", minLength: 1 },
              content_type: { type: "string" },
            },
            additionalProperties: false,
          },
          canonical_actions: ["media.upload", "media.wait"],
        },
        {
          name: "generate_post_example",
          description: "Generate dependency-free cURL or native Node fetch examples.",
          command: "unipost examples posts.create --lang <curl|node> --json",
          canonical_action: "examples.posts.create",
          safety_level: "read_only",
          required_inputs: [],
          optional_inputs: ["language", "account_ids", "caption"],
          input_schema: {
            type: "object",
            properties: {
              language: { type: "string", enum: ["curl", "node"] },
              account_ids: { type: "array", items: { type: "string" } },
              caption: { type: "string" },
            },
            additionalProperties: false,
          },
        },
      ],
    },
    human: "Agent capabilities are available as JSON.\n",
  });
}

async function agentContextData(context) {
  const config = await readConfig(context);
  const [{ workspace, response: workspaceResponse }, { profiles }, { accounts: accountList }] = await Promise.all([
    fetchWorkspace(context),
    fetchProfiles(context),
    fetchAccounts(context),
  ]);
  return {
    workspace,
    profiles,
    accounts: accountList,
    defaults: {
      workspace_id: config.default_workspace_id || workspace?.id || "",
      profile_id: config.default_profile_id || "",
    },
    grounding: {
      profile_count: profiles.length,
      account_count: accountList.length,
      has_default_profile: Boolean(config.default_profile_id),
    },
    request_id: workspaceResponse.requestId,
  };
}

async function agentBootstrap(context) {
  const client = context.options.client || "codex";
  if (!context.options.apiKey && context.options.setupToken) {
    await loginWithSetupToken(context);
  }
  if (!context.options.apiKey) {
    return envelopeResult({
      data: {
        client,
        authenticated: false,
        ready_for_draft: false,
        setup_token_supported: true,
        next_actions: [
          "Open UniPost Dashboard API Keys and choose Connect with Claude Code / Codex.",
          `Run ${CLI_RUNNER} agent bootstrap --setup-token <token> --client <client> --json once.`,
          "Or set UNIPOST_API_KEY for command-level fallback auth.",
          `Run ${CLI_RUNNER} init after the API key is available.`,
        ],
        recommended_prompt: `Ask the user to generate a UniPost CLI setup token in Dashboard, then run \`${CLI_RUNNER} agent bootstrap --setup-token <token> --json\` once.`,
      },
      warnings: [{
        code: "auth_setup_required",
        message: "Setup-token auth is available, but no setup token or stored credential was provided.",
      }],
      human: "UniPost auth is missing. Generate a setup token from the dashboard before agent use.\n",
      exitCode: EXIT.auth,
    });
  }

  const data = await agentContextData(context);
  const readyForDraft = data.profiles.length > 0 && data.accounts.length > 0;
  const nextActions = [];
  if (data.profiles.length === 0) {
    nextActions.push("Run unipost profiles create --name \"Brand\".");
  }
  if (data.profiles.length > 0 && data.accounts.length === 0) {
    nextActions.push("Run unipost connect create --platform linkedin --json and have the user complete OAuth.");
  }
  if (readyForDraft) {
    nextActions.push("Run unipost posts validate before unipost posts draft.");
  }

  return envelopeResult({
    data: {
      client,
      authenticated: true,
      ready_for_draft: readyForDraft,
      ...data,
      next_actions: nextActions,
      recommended_prompt: agentGuide(client).recommended_prompt,
    },
    human: readyForDraft
      ? "UniPost agent bootstrap is ready for validate/draft workflows.\n"
      : "UniPost agent bootstrap found setup gaps.\n",
  });
}

function agentGuide(client) {
  const normalizedClient = String(client || "codex").toLowerCase();
  return {
    client: normalizedClient,
    recommended_prompt: [
      `Before using UniPost, run \`${CLI_RUNNER} agent bootstrap --json\`.`,
      `For publish requests, call \`${CLI_RUNNER} agent plan --intent plan_publish_post --json\` first.`,
      `Run \`${CLI_RUNNER} posts validate --json\` or the dry-run command before any publish-capable write.`,
      `Use \`${CLI_RUNNER} posts create --from-file post.json --dry-run --json\` before asking for live-publish approval.`,
      "Never live-publish unless the user explicitly approves the exact action and the command includes --yes plus --idempotency-key.",
    ].join(" "),
    stable_contracts: [
      "Branch on normalized_code, exit code, and documented status enum values.",
      "Treat canceled as the only CLI-facing spelling for canceled resources.",
    ],
  };
}

function agentMcpConfig(context, client) {
  const normalizedClient = String(client || "claude-code").toLowerCase();
  const bearer = "Bearer ${UNIPOST_API_KEY}";
  if (normalizedClient === "claude-code") {
    const content = [
      "claude mcp add unipost \\",
      "  -t http \\",
      "  --header \"Authorization:Bearer ${UNIPOST_API_KEY}\" \\",
      `  -- "${MCP_ENDPOINT}"`,
    ].join("\n");
    return {
      client: "claude-code",
      transport: "streamable_http",
      endpoint: MCP_ENDPOINT,
      content,
      auth_test_command: `${CLI_RUNNER} agent mcp-test --json`,
    };
  }
  if (normalizedClient === "codex") {
    const content = [
      "[mcp_servers.unipost]",
      `url = "${MCP_ENDPOINT}"`,
      'headers = { Authorization = "Bearer ${UNIPOST_API_KEY}" }',
    ].join("\n");
    return {
      client: "codex",
      transport: "streamable_http",
      endpoint: MCP_ENDPOINT,
      content,
      auth_test_command: `${CLI_RUNNER} agent mcp-test --json`,
    };
  }
  const config = {
    mcpServers: {
      unipost: {
        url: MCP_ENDPOINT,
        headers: {
          Authorization: bearer,
        },
      },
    },
  };
  return {
    client: normalizedClient,
    transport: "streamable_http",
    endpoint: MCP_ENDPOINT,
    config,
    content: JSON.stringify(config, null, 2),
    auth_test_command: `${CLI_RUNNER} agent mcp-test --json`,
  };
}

function claudeCodeMcpExample() {
  return [
    "export UNIPOST_API_KEY=up_live_...",
    `${CLI_RUNNER} agent mcp-test --json`,
    "",
    "claude mcp add unipost \\",
    "  -t http \\",
    "  --header \"Authorization:Bearer ${UNIPOST_API_KEY}\" \\",
    `  -- "${MCP_ENDPOINT}"`,
    "",
    "Then ask Claude Code to run UniPost through the MCP tools and keep live publish behind explicit approval.",
  ].join("\n");
}

async function agentMcpTest(context) {
  requireApiKey(context, "agent mcp-test");
  const { workspace, response } = await fetchWorkspace(context);
  return envelopeResult({
    data: {
      authenticated: true,
      workspace,
      catalog_version: AGENT_CATALOG_VERSION,
      mcp: {
        endpoint: MCP_ENDPOINT,
        transport: "streamable_http",
        auth_header: "Authorization: Bearer ${UNIPOST_API_KEY}",
        mirrors_intents: agentIntentNames(),
      },
    },
    meta: { request_id: response.requestId, rate_limit: response.rateLimit },
    human: `MCP auth ready for workspace ${workspace?.id || "unknown"}.\n`,
  });
}

function agentInstallInstructions(client) {
  const normalizedClient = String(client || "codex").toLowerCase();
  const file = normalizedClient === "claude-code"
    ? "agent-packages/claude-code/CLAUDE.md"
    : "agent-packages/codex/SKILL.md";
  return {
    client: normalizedClient,
    mode: "instructions",
    automatic_install: false,
    files: AGENT_PACKAGE_FILES.map((path) => ({
      path,
      description: path.endsWith("SKILL.md") ? "Codex skill instructions" : "Claude Code project instructions",
    })),
    selected_file: file,
    instructions: [
      `Use ${file} as the first-party UniPost instruction package for ${normalizedClient}.`,
      `Run \`${CLI_RUNNER} agent bootstrap --json\`, \`${CLI_RUNNER} agent capabilities --json\`, and \`${CLI_RUNNER} agent guide --client <client>\` before modifying user projects.`,
      `Use \`${CLI_RUNNER} agent mcp-test --json\` before configuring MCP.`,
      "Do not execute live publish plans; use explicit publish commands only after user approval with --yes and --idempotency-key.",
    ].join("\n"),
  };
}

async function agentExecute(context) {
  requireApiKey(context, "agent execute");
  const planPath = requireValue(context.options.plan, "--plan <plan.json>", "agent execute requires --plan.");
  const plan = await readJsonFile(context, planPath);
  const planData = validateAgentPlanEnvelope(plan);
  const actions = planData.actions;
  if (actions.length === 0) {
    throw new CliError({
      code: "missing_required_input",
      normalizedCode: "missing_required_input",
      message: "agent execute found no structured actions in the plan.",
      hint: `Create a plan with ${CLI_RUNNER} agent plan --intent ... --json.`,
      exitCode: EXIT.missingInput,
    });
  }
  validateAgentExecuteActions(actions);
  if (hasOutstandingAgentConfirmation(planData)) {
    throw new CliError({
      code: "agent_execute_confirmation_required",
      normalizedCode: "agent_execute_confirmation_required",
      message: "agent execute cannot run a plan with missing inputs or user confirmations.",
      hint: "Resolve missing inputs and use explicit CLI commands for any user-approved action.",
      exitCode: EXIT.unsafe,
    });
  }

  const executedActions = [];
  const results = [];
  for (const action of actions) {
    const result = await executeStructuredAgentAction(context, action);
    executedActions.push(action.canonical_action);
    results.push({
      canonical_action: action.canonical_action,
      safety_level: agentExecuteRegistry()[action.canonical_action].safety_level,
      data: result.data,
      warnings: result.warnings || [],
      meta: result.meta || {},
    });
  }

  return envelopeResult({
    data: {
      plan: basename(planPath),
      executed_actions: executedActions,
      results,
    },
    human: `Executed ${executedActions.length} safe agent actions.\n`,
  });
}

function validateAgentPlanEnvelope(plan) {
  if (!plan || typeof plan !== "object" || Array.isArray(plan) || plan.ok !== true || !plan.data || typeof plan.data !== "object" || Array.isArray(plan.data)) {
    throw new CliError({
      code: "invalid_agent_plan",
      normalizedCode: "invalid_agent_plan",
      message: `agent execute requires a JSON envelope produced by ${CLI_RUNNER} agent plan --json.`,
      hint: `Create a fresh plan with ${CLI_RUNNER} agent plan --intent ... --json and pass it with --plan.`,
      exitCode: EXIT.validation,
    });
  }

  const payload = plan.data;
  if (payload.catalog_version !== AGENT_CATALOG_VERSION) {
    throw new CliError({
      code: "stale_agent_plan",
      normalizedCode: "stale_agent_plan",
      message: `agent execute requires catalog_version ${AGENT_CATALOG_VERSION}.`,
      hint: "Regenerate the plan with the current UniPost CLI before executing it.",
      exitCode: EXIT.validation,
    });
  }
  if (!agentIntentNames().includes(payload.intent) || !Array.isArray(payload.actions)) {
    throw new CliError({
      code: "invalid_agent_plan",
      normalizedCode: "invalid_agent_plan",
      message: "agent execute found an invalid agent plan payload.",
      hint: `Run ${CLI_RUNNER} agent capabilities --json and regenerate the plan with ${CLI_RUNNER} agent plan --json.`,
      exitCode: EXIT.validation,
    });
  }
  return payload;
}

function hasOutstandingAgentConfirmation(planData) {
  return planData.safe_to_execute_without_user !== true
    || (Array.isArray(planData.missing_inputs) && planData.missing_inputs.length > 0)
    || (Array.isArray(planData.required_user_confirmations) && planData.required_user_confirmations.length > 0);
}

function validateAgentExecuteActions(actions) {
  for (const action of actions) {
    const spec = agentExecuteRegistry()[action?.canonical_action];
    if (!spec) {
      throw new CliError({
        code: "unsupported_agent_execute_action",
        normalizedCode: "unsupported_agent_execute_action",
        message: `agent execute does not support action ${action?.canonical_action || "unknown"}.`,
        hint: `Run ${CLI_RUNNER} agent capabilities --json and use explicit CLI commands for unsupported actions.`,
        exitCode: EXIT.validation,
      });
    }
    if (spec.safety_level === "live_write") {
      throw new CliError({
        code: "requires_explicit_publish_command",
        normalizedCode: "requires_explicit_publish_command",
        message: "agent execute cannot execute live publish actions from a plan.",
        hint: "Ask the user for explicit approval, then use posts create/publish-draft with --yes and --idempotency-key.",
        exitCode: EXIT.unsafe,
      });
    }
    if (!["read_only", "validate_only", "draft_write"].includes(spec.safety_level)) {
      throw new CliError({
        code: "agent_execute_confirmation_required",
        normalizedCode: "agent_execute_confirmation_required",
        message: `agent execute cannot run ${spec.safety_level} actions.`,
        hint: "Use the explicit CLI command after the user approves the exact action.",
        exitCode: EXIT.unsafe,
      });
    }
  }
}

async function executeStructuredAgentAction(context, action) {
  const registry = agentExecuteRegistry();
  const spec = registry[action?.canonical_action];
  return spec.run(context, actionOptionsFromArgs(action.args || {}));
}

function agentExecuteRegistry() {
  return {
    "posts.validate": {
      safety_level: "validate_only",
      run: (context, options) => posts(derivedContext(context, ["posts", "validate"], options), "validate"),
    },
    "posts.draft": {
      safety_level: "draft_write",
      run: (context, options) => posts(derivedContext(context, ["posts", "draft"], options), "draft"),
    },
    "accounts.health": {
      safety_level: "read_only",
      run: (context, options) => accounts(derivedContext(context, ["accounts", "health"], options), "health"),
    },
    "accounts.capabilities": {
      safety_level: "read_only",
      run: (context, options) => accounts(derivedContext(context, ["accounts", "capabilities"], options), "capabilities"),
    },
    "accounts.metrics": {
      safety_level: "read_only",
      run: (context, options) => accounts(derivedContext(context, ["accounts", "metrics"], options), "metrics"),
    },
    "media.wait": {
      safety_level: "read_only",
      run: (context, options) => media(derivedContext(context, ["media", "wait"], options), "wait", options.media_id),
    },
    "posts.create": {
      safety_level: "live_write",
      run: null,
    },
    "posts.schedule": {
      safety_level: "live_write",
      run: null,
    },
    "posts.publish_draft": {
      safety_level: "live_write",
      run: null,
    },
  };
}

function derivedContext(context, commandParts, options) {
  return {
    ...context,
    commandParts,
    options: {
      ...context.options,
      ...options,
      output: "json",
      json: true,
    },
  };
}

function actionOptionsFromArgs(args) {
  const options = {};
  for (const [key, value] of Object.entries(args || {})) {
    if (key.startsWith("--")) {
      options[toCamelCase(key.slice(2))] = value;
      if (key === "--json") {
        options.json = Boolean(value);
      }
    } else if (key === "account_id") {
      options.account = value;
    } else if (key === "account_ids") {
      options.account = Array.isArray(value) ? value.join(",") : value;
    } else if (key === "media_id") {
      options.media_id = value;
    } else {
      options[key] = value;
    }
  }
  return options;
}

function agentIntentNames() {
  return [
    "diagnose_setup",
    "diagnose_account",
    "create_draft_post",
    "plan_publish_post",
    "connect_account",
    "upload_media",
    "generate_post_example",
  ];
}

async function selectProfile(context, profiles, options = {}) {
  if (context.options.profile) {
    const explicit = profiles.find((profile) => profile.id === context.options.profile);
    if (explicit) {
      return explicit;
    }
    return { id: context.options.profile };
  }
  const config = await readConfig(context);
  if (config.default_profile_id) {
    const configured = profiles.find((profile) => profile.id === config.default_profile_id);
    return configured || { id: config.default_profile_id };
  }
  if (profiles.length === 1) {
    return profiles[0];
  }
  if (profiles.length === 0 && options.allowCreate) {
    return null;
  }
  return null;
}

async function resolveProfileID(context) {
  if (context.options.profile) {
    return context.options.profile;
  }
  const config = await readConfig(context);
  if (config.default_profile_id) {
    return config.default_profile_id;
  }
  const { profiles } = await fetchProfiles(context);
  if (profiles.length === 1) {
    return profiles[0].id;
  }
  if (profiles.length === 0) {
    throw new CliError({
      code: "missing_required_input",
      normalizedCode: "missing_required_input",
      message: "No profile exists for this workspace.",
      hint: "Run unipost profiles create --name \"Brand\" or pass --profile after creating one.",
      exitCode: EXIT.missingInput,
    });
  }
  throw new CliError({
    code: "missing_required_input",
    normalizedCode: "missing_required_input",
    message: "Multiple profiles exist; choose one for this command.",
    hint: "Pass --profile <profile_id> or run unipost profiles use <profile_id>.",
    exitCode: EXIT.missingInput,
  });
}

function filterAccounts(accountList, options) {
  return accountList.filter((account) => {
    if (options.platform && account.platform !== options.platform) {
      return false;
    }
    if (options.profile && account.profile_id !== options.profile) {
      return false;
    }
    return true;
  });
}

function pollDelayMs(context, attempt, headers) {
  const testMs = Number(context.env.UNIPOST_TEST_POLL_MS);
  if (Number.isFinite(testMs) && testMs >= 0) {
    return testMs;
  }
  return retryDelayMs(headers.get("retry-after"), attempt);
}

function curlPostExample(context, payload) {
  const body = JSON.stringify(payload);
  return [
    "curl -sS \\",
    `  -H 'Authorization: Bearer $UNIPOST_API_KEY' \\`,
    "  -H 'Content-Type: application/json' \\",
    `  -d '${body}' \\`,
    `  '${context.options.baseUrl}/v1/posts'`,
  ].join("\n");
}

function nodeFetchPostExample(context, payload) {
  return `const response = await fetch("${context.options.baseUrl}/v1/posts", {
  method: "POST",
  headers: {
    Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify(${JSON.stringify(payload, null, 2)}),
});

const body = await response.json();
console.log(body);`;
}

function invalidSubcommand(command, subcommand) {
  throw new CliError({
    code: "invalid_command",
    normalizedCode: "invalid_command",
    message: `Unknown command: ${command}${subcommand ? ` ${subcommand}` : ""}`,
    hint: "Run unipost --help to see supported commands.",
    docsUrl: DOCS_CLI_URL,
    exitCode: EXIT.invalidArgs,
  });
}

function requireApiKey(context, command) {
  if (context.options.apiKey) {
    return;
  }
  throw new CliError({
    code: "unauthorized",
    normalizedCode: "unauthorized",
    message: "API key is missing.",
    hint: "Run auth login --setup-token <token>, set UNIPOST_API_KEY, or pass --api-key.",
    docsUrl: DOCS_QUICKSTART_URL,
    exitCode: EXIT.auth,
    requestId: "",
    status: 401,
    command,
  });
}

async function requestJson(context, path, options = {}) {
  const auth = options.auth !== false;
  const method = options.method || "GET";
  const url = new URL(path, `${context.options.baseUrl}/`);
  const headers = {
    Accept: "application/json",
    "User-Agent": `unipost-cli/${CLI_VERSION}`,
    "X-UniPost-CLI-Version": CLI_VERSION,
    "X-UniPost-CLI-Source": "cli",
    "X-UniPost-CLI-Command": context.commandParts.join(" "),
  };

  if (auth) {
    headers.Authorization = `Bearer ${context.options.apiKey}`;
  }
  if (context.options.agentName) {
    headers["X-UniPost-Agent-Name"] = context.options.agentName;
  }
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (context.options.idempotencyKey) {
    headers["Idempotency-Key"] = context.options.idempotencyKey;
  }

  const retryable = method === "GET" || method === "HEAD";
  const maxAttempts = retryable ? 3 : 1;
  let lastError = null;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    let response;
    try {
      response = await context.fetchImpl(url, {
        method,
        headers,
        ...(options.body !== undefined ? { body: JSON.stringify(options.body) } : {}),
      });
    } catch (error) {
      lastError = new CliError({
        code: "network_error",
        normalizedCode: "network_error",
        message: `Network request failed: ${error.message}`,
        hint: "Check connectivity, proxy settings, and the --base-url value.",
        exitCode: EXIT.network,
        cause: error,
      });
      if (attempt < maxAttempts) {
        await sleep(backoffMs(attempt));
        continue;
      }
      throw lastError;
    }

    const body = await readJsonBody(response);
    const requestId = body?.request_id || response.headers.get("x-request-id") || "";
    const rateLimit = rateLimitFromHeaders(response.headers);

    if (response.ok) {
      return { response, body, requestId, rateLimit };
    }

    if (retryable && attempt < maxAttempts && shouldRetry(response.status)) {
      await sleep(retryDelayMs(response.headers.get("retry-after"), attempt));
      continue;
    }

    throw errorFromApiResponse(response, body, requestId);
  }

  throw lastError || new CliError({
    code: "network_error",
    normalizedCode: "network_error",
    message: "Network request failed.",
    exitCode: EXIT.network,
  });
}

function unwrapData(body) {
  return body && Object.prototype.hasOwnProperty.call(body, "data") ? body.data : body;
}

function paginationFromBody(body) {
  const meta = body?.meta || {};
  const pagination = {};
  for (const key of ["total", "limit", "has_more", "next_cursor"]) {
    if (meta[key] !== undefined && meta[key] !== null && meta[key] !== "") {
      pagination[key] = meta[key];
    }
  }
  if (body?.next_cursor) {
    pagination.next_cursor = body.next_cursor;
  }
  return pagination;
}

function apiPath(path, params = {}) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && String(value) !== "") {
      search.set(key, String(value));
    }
  }
  const query = search.toString();
  return query ? `${path}?${query}` : path;
}

function configDir(context) {
  return context.env.UNIPOST_CONFIG_DIR || join(homedir(), ".unipost");
}

function configPath(context) {
  return join(configDir(context), "config.json");
}

async function applyConfigDefaults(context) {
  if (context.options.version || context.options.help || context.commandParts[0] === "config") {
    return;
  }
  const config = await readConfig(context);
  if (context.options.baseUrlSource === "default" && config.base_url) {
    validateConfigBaseURL(config.base_url);
    context.options.baseUrl = normalizeBaseUrl(config.base_url);
    context.options.baseUrlSource = "config";
  }
  if (shouldLoadStoredCredential(context) && config.credential?.storage === "keychain") {
    const value = await context.secureStore.get({
      service: config.credential.keychain_service || KEYCHAIN_SERVICE,
      account: config.credential.keychain_account,
    });
    if (value) {
      context.options.apiKey = value;
      context.options.credentialSource = "keychain";
    }
  }
}

function shouldLoadStoredCredential(context) {
  if (context.options.apiKey || context.options.setupToken) {
    return false;
  }
  const command = context.commandParts.join(" ");
  return command !== "auth login" && command !== "auth logout";
}

async function readConfig(context) {
  try {
    return JSON.parse(await readFile(configPath(context), "utf8"));
  } catch (error) {
    if (error?.code === "ENOENT") {
      return {};
    }
    throw new CliError({
      code: "config_error",
      normalizedCode: "config_error",
      message: `Failed to read UniPost config: ${error.message}`,
      exitCode: EXIT.generic,
      cause: error,
    });
  }
}

async function writeConfig(context, nextConfig) {
  const dir = configDir(context);
  await mkdir(dir, { recursive: true, mode: 0o700 });
  const safeConfig = stripSecrets(nextConfig);
  await writeFile(configPath(context), `${JSON.stringify(safeConfig, null, 2)}\n`, { mode: 0o600 });
  return safeConfig;
}

async function patchConfig(context, patch) {
  const current = await readConfig(context);
  return writeConfig(context, {
    ...current,
    ...patch,
    updated_at: new Date().toISOString(),
  });
}

function sanitizeConfig(value) {
  return stripSecrets(value || {});
}

function fingerprintSecret(value) {
  return createHash("sha256").update(String(value || "")).digest("hex").slice(0, 16);
}

function stripSecrets(value) {
  if (Array.isArray(value)) {
    return value.map(stripSecrets);
  }
  if (value && typeof value === "object") {
    const out = {};
    for (const [key, item] of Object.entries(value)) {
      if (/(api[_-]?key|token|secret|password)/i.test(key)) {
        continue;
      }
      out[key] = stripSecrets(item);
    }
    return out;
  }
  return value;
}

function normalizeStatus(status) {
  if (status === "cancelled") {
    return "canceled";
  }
  if (status === "uploaded") {
    return "ready";
  }
  return status;
}

function normalizeStatuses(value) {
  if (Array.isArray(value)) {
    return value.map(normalizeStatuses);
  }
  if (value && typeof value === "object") {
    const out = {};
    for (const [key, item] of Object.entries(value)) {
      if (key === "status" && typeof item === "string") {
        out[key] = normalizeStatus(item);
      } else {
        out[key] = normalizeStatuses(item);
      }
    }
    return out;
  }
  return value;
}

function requireValue(value, flag, message) {
  if (value !== undefined && value !== null && String(value).trim() !== "") {
    return String(value).trim();
  }
  throw new CliError({
    code: "missing_required_input",
    normalizedCode: "missing_required_input",
    message,
    hint: `Pass ${flag}.`,
    exitCode: EXIT.missingInput,
  });
}

function requireAccountID(context, value, command) {
  return requireValue(value || context.options.account, "--account <account_id>", `${command} requires an account ID.`);
}

function splitIDs(value) {
  return String(value || "").split(",").map((id) => id.trim()).filter(Boolean);
}

async function fetchWorkspace(context) {
  const response = await requestJson(context, "/v1/workspace", { auth: true });
  return {
    workspace: normalizeStatuses(unwrapData(response.body)),
    response,
  };
}

async function fetchProfiles(context) {
  const response = await requestJson(context, "/v1/profiles", { auth: true });
  return {
    profiles: normalizeStatuses(unwrapData(response.body) || []),
    pagination: paginationFromBody(response.body),
    response,
  };
}

async function fetchAccounts(context) {
  const response = await requestJson(context, "/v1/accounts", { auth: true });
  return {
    accounts: normalizeStatuses(unwrapData(response.body) || []),
    pagination: paginationFromBody(response.body),
    response,
  };
}

async function readJsonBody(response) {
  const text = await response.text();
  if (!text) {
    return {};
  }
  try {
    return JSON.parse(text);
  } catch {
    return { raw: text };
  }
}

function errorFromApiResponse(response, body, requestId) {
  const backend = body?.error || {};
  const normalizedCode = normalizeCode(backend.normalized_code || backend.code || statusCodeToCode(response.status));
  const exitCode = ERROR_EXIT_BY_CODE.get(normalizedCode) || exitCodeFromStatus(response.status);
  return new CliError({
    code: normalizedCode,
    normalizedCode,
    message: backend.message || `UniPost API returned HTTP ${response.status}.`,
    hint: hintForCode(normalizedCode),
    docsUrl: DOCS_CLI_URL,
    exitCode,
    requestId,
    status: response.status,
  });
}

function normalizeError(error) {
  if (error instanceof CliError) {
    return error;
  }
  return new CliError({
    code: "generic_error",
    normalizedCode: "generic_error",
    message: error?.message || "Unexpected error.",
    exitCode: EXIT.generic,
    cause: error,
  });
}

function normalizeCode(code) {
  return String(code || "error").trim().toLowerCase();
}

function statusCodeToCode(status) {
  if (status === 401) return "unauthorized";
  if (status === 403) return "forbidden";
  if (status === 404) return "not_found";
  if (status === 429) return "request_rate_limited";
  if (status >= 500) return "internal_error";
  return "invalid_request";
}

function exitCodeFromStatus(status) {
  if (status === 401) return EXIT.auth;
  if (status === 403) return EXIT.authz;
  if (status >= 500 || status === 429) return EXIT.upstream;
  return EXIT.validation;
}

function hintForCode(code) {
  if (code === "unauthorized") {
    return "Check UNIPOST_API_KEY, local auth, or run unipost auth status.";
  }
  if (code === "forbidden") {
    return "The current credential lacks permission for this action.";
  }
  if (code === "request_rate_limited" || code === "rate_limited" || code === "enqueue_rate_limited" || code === "queue_depth_exceeded") {
    return "Respect rate-limit headers and retry later with backoff.";
  }
  if (code === "internal_error") {
    return "Capture request_id and retry or contact support.";
  }
  return "Check command input and retry.";
}

function shouldRetry(status) {
  return status === 429 || status >= 500;
}

function retryDelayMs(retryAfter, attempt) {
  if (retryAfter) {
    const seconds = Number(retryAfter);
    if (Number.isFinite(seconds) && seconds >= 0) {
      return Math.min(seconds * 1000, 5000);
    }
    const date = Date.parse(retryAfter);
    if (!Number.isNaN(date)) {
      return Math.min(Math.max(date - Date.now(), 0), 5000);
    }
  }
  return backoffMs(attempt);
}

function backoffMs(attempt) {
  return Math.min(100 * 2 ** (attempt - 1), 1000);
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function rateLimitFromHeaders(headers) {
  const values = {
    limit: headers.get("x-unipost-ratelimit-limit"),
    remaining: headers.get("x-unipost-ratelimit-remaining"),
    reset: headers.get("x-unipost-ratelimit-reset"),
    retry_after: headers.get("retry-after"),
  };
  return Object.fromEntries(Object.entries(values).filter(([, value]) => value));
}

function runCheck(fn) {
  return fn().then((value) => ({ ok: true, value })).catch((error) => ({ ok: false, error: normalizeError(error) }));
}

function chooseHardExit(current, next) {
  if (current === EXIT.success) {
    return next || EXIT.generic;
  }
  return current;
}

function completion(context, shell) {
  const requestedShell = shell || "bash";
  if (!["bash", "zsh", "fish"].includes(requestedShell)) {
    throw new CliError({
      code: "invalid_argument",
      normalizedCode: "invalid_argument",
      message: "completion requires one of: bash, zsh, fish.",
      exitCode: EXIT.invalidArgs,
    });
  }
  if (requestedShell === "zsh") {
    return textResult(zshCompletion());
  }
  if (requestedShell === "fish") {
    return textResult(fishCompletion());
  }
  return textResult(bashCompletion());
}

function zshCompletion() {
  return `#compdef unipost

_unipost() {
  local -a commands
  commands=(
    'config path:Print the local UniPost config path'
    'config show:Print redacted local UniPost config'
    'config set:Set a safe local UniPost config value'
    'auth login:Log in with a setup token or validate API-key metadata'
    'auth logout:Remove local credential metadata'
    'auth status:Verify the configured UniPost credential'
    'auth list:List locally discoverable UniPost credentials'
    'init:Initialize local UniPost CLI config'
    'quickstart:Run the safe first-run workflow'
    'profiles list:List profiles'
    'profiles create:Create a profile'
    'connect create:Create an account Connect session'
    'accounts list:List connected accounts'
    'accounts health:Get account health diagnostics'
    'accounts capabilities:Get account platform capabilities'
    'accounts metrics:Get account metrics'
    'posts list:List posts'
    'posts get:Get a post'
    'posts analytics:Get post analytics'
    'posts validate:Validate a post payload'
    'posts draft:Create a server-side draft'
    'posts create:Create a live or scheduled post'
    'posts schedule:Schedule a post'
    'posts wait:Wait for a terminal post status'
    'posts cancel:Cancel a draft or scheduled post'
    'posts retry:Retry a failed result'
    'media upload:Upload local media'
    'media get:Get media status'
    'media wait:Wait for media readiness'
    'analytics summary:Get analytics summary'
    'analytics posts:Get post analytics rows'
    'analytics platforms:Get analytics platforms'
    'analytics platform:Get analytics for one platform'
    'examples posts.create:Generate REST examples'
    'examples mcp.claude-code:Generate Claude Code MCP setup example'
    'agent plan:Build a safe execution plan'
    'agent plan-publish:Plan a publish flow'
    'agent execute:Execute a structured safe plan'
    'agent bootstrap:Diagnose agent setup'
    'agent capabilities:Print agent command catalog'
    'agent context:Print grounded workspace context'
    'agent guide:Print client-specific agent guidance'
    'agent mcp-config:Generate MCP client config'
    'agent mcp-test:Validate MCP auth readiness'
    'agent install:Print agent instruction package setup'
    'doctor:Run local and API diagnostics'
    'completion:Generate shell completion'
  )
  _arguments \\
    '--json[Output the stable JSON envelope]' \\
    '--output[Output format]:format:(table json yaml)' \\
    '--field[Print one field from the envelope]:field:' \\
    '--base-url[Override API base URL]:url:' \\
    '--api-key[Pass a UniPost API key]:key:' \\
    '--client[Calling client]:client:(codex claude-code cursor windsurf)' \\
    '--limit[Page size for list commands]:limit:' \\
    '--cursor[Page cursor for list commands]:cursor:' \\
    '--status[Filter posts by status]:status:' \\
    '--result[Post result id]:result:' \\
    '--from[Start date]:from:' \\
    '--to[End date]:to:' \\
    '--at[Schedule timestamp]:at:' \\
    '--schedule-at[Schedule timestamp]:schedule-at:' \\
    '--from-file[Read request JSON]:file:' \\
    '--plan[Read structured agent plan JSON]:file:' \\
    '--content-type[Override media MIME type]:mime:' \\
    '--idempotency-key[Idempotency key]:key:' \\
    '--agent-name[Calling agent name]:name:' \\
    '--yes[Confirm publish-capable or destructive writes]' \\
    '--all[Follow pagination until exhausted]' \\
    '--non-interactive[Never prompt]' \\
    '--no-color[Disable ANSI color]' \\
    '--no-telemetry[Disable telemetry for this run]' \\
    '*::command:->command'
}

_unipost "$@"
`;
}

function bashCompletion() {
  return `# bash completion for unipost
_unipost_completion() {
  local words="init quickstart config path config show config set auth login auth logout auth status auth list auth use profiles list profiles get profiles create profiles use connect create connect get connect wait accounts list accounts get accounts health accounts capabilities accounts metrics posts list posts get posts analytics posts validate posts draft posts create posts schedule posts publish-draft posts wait posts cancel posts retry media upload media get media wait analytics summary analytics posts analytics platforms analytics platform examples posts.create examples mcp.claude-code agent plan agent plan-publish agent execute agent bootstrap agent capabilities agent context agent guide agent mcp-config agent mcp-test agent install doctor completion --json --output --field --base-url --api-key --setup-token --client --name --profile --platform --account --caption --status --result --from --to --at --schedule-at --from-file --plan --content-type --idempotency-key --agent-name --yes --limit --cursor --all --non-interactive --no-color --no-telemetry"
  COMPREPLY=($(compgen -W "$words" -- "\${COMP_WORDS[COMP_CWORD]}"))
}
complete -F _unipost_completion unipost
`;
}

function fishCompletion() {
  return `complete -c unipost -f
complete -c unipost -a "config path" -d "Print local config path"
complete -c unipost -a "config show" -d "Print redacted local config"
complete -c unipost -a "config set" -d "Set a safe local config value"
complete -c unipost -a "auth login" -d "Log in with setup-token keychain auth or API-key metadata"
complete -c unipost -a "auth logout" -d "Remove local credential metadata"
complete -c unipost -a "auth status" -d "Verify the configured UniPost credential"
complete -c unipost -a "profiles list" -d "List profiles"
complete -c unipost -a "connect create" -d "Create a Connect session"
complete -c unipost -a "accounts list" -d "List connected accounts"
complete -c unipost -a "accounts health" -d "Get account health diagnostics"
complete -c unipost -a "accounts capabilities" -d "Get account platform capabilities"
complete -c unipost -a "accounts metrics" -d "Get account metrics"
complete -c unipost -a "posts list" -d "List posts"
complete -c unipost -a "posts get" -d "Get a post"
complete -c unipost -a "posts validate" -d "Validate a post"
complete -c unipost -a "posts draft" -d "Create a draft"
complete -c unipost -a "posts create" -d "Create a post"
complete -c unipost -a "posts schedule" -d "Schedule a post"
complete -c unipost -a "posts wait" -d "Wait for post status"
complete -c unipost -a "posts cancel" -d "Cancel a post"
complete -c unipost -a "posts retry" -d "Retry a failed result"
complete -c unipost -a "media upload" -d "Upload local media"
complete -c unipost -a "media get" -d "Get media status"
complete -c unipost -a "media wait" -d "Wait for media readiness"
complete -c unipost -a "analytics summary" -d "Get analytics summary"
complete -c unipost -a "analytics platforms" -d "Get analytics platforms"
complete -c unipost -a "examples posts.create" -d "Generate REST examples"
complete -c unipost -a "examples mcp.claude-code" -d "Generate Claude Code MCP setup example"
complete -c unipost -a "agent plan" -d "Build a safe execution plan"
complete -c unipost -a "agent execute" -d "Execute a structured safe plan"
complete -c unipost -a "agent bootstrap" -d "Diagnose agent setup"
complete -c unipost -a "agent capabilities" -d "Print agent command catalog"
complete -c unipost -a "agent context" -d "Print grounded workspace context"
complete -c unipost -a "agent guide" -d "Print client-specific agent guidance"
complete -c unipost -a "agent mcp-config" -d "Generate MCP client config"
complete -c unipost -a "agent mcp-test" -d "Validate MCP auth readiness"
complete -c unipost -a "agent install" -d "Print agent instruction package setup"
complete -c unipost -a doctor -d "Run local and API diagnostics"
complete -c unipost -a completion -d "Generate shell completion"
complete -c unipost -l json -d "Output the stable JSON envelope"
complete -c unipost -l client -xa "codex claude-code cursor windsurf"
complete -c unipost -l limit -d "Page size for list commands"
complete -c unipost -l cursor -d "Page cursor for list commands"
complete -c unipost -l all -d "Follow pagination until exhausted"
complete -c unipost -l status -d "Filter posts by status"
complete -c unipost -l result -d "Post result ID"
complete -c unipost -l from -d "Start date"
complete -c unipost -l to -d "End date"
complete -c unipost -l at -d "Schedule timestamp"
complete -c unipost -l schedule-at -d "Schedule timestamp"
complete -c unipost -l from-file -d "Read request JSON"
complete -c unipost -l plan -d "Read structured agent plan JSON"
complete -c unipost -l content-type -d "Override media MIME type"
complete -c unipost -l idempotency-key -d "Idempotency key"
complete -c unipost -l agent-name -d "Calling agent name"
complete -c unipost -l yes -d "Confirm write"
`;
}

function envelopeResult({ data, warnings = [], meta = {}, human = "", exitCode = EXIT.success }) {
  return {
    kind: "envelope",
    data,
    warnings,
    meta,
    human,
    exitCode,
  };
}

function textResult(text) {
  return { kind: "text", text, exitCode: EXIT.success };
}

function writeResult(context, result) {
  if (result.kind === "text") {
    context.stdout.write(result.text);
    return;
  }
  if (context.options.output === "json" || context.options.field) {
    writeEnvelope(context, successEnvelope(context, result));
    return;
  }
  if (context.options.output === "yaml") {
    context.stdout.write(toYaml(successEnvelope(context, result)));
    return;
  }
  context.stdout.write(result.human || `${JSON.stringify(result.data, null, 2)}\n`);
}

function writeError(context, error) {
  const envelope = errorEnvelope(context, error);
  if (context.options?.output === "json" || context.options?.field) {
    writeEnvelope(context, envelope);
    return;
  }
  if (context.options?.output === "yaml") {
    context.stdout.write(toYaml(envelope));
    return;
  }
  context.stderr.write(`${error.message}\n`);
  if (error.hint) {
    context.stderr.write(`${error.hint}\n`);
  }
}

function writeEnvelope(context, envelope) {
  if (context.options.field) {
    const value = selectField(envelope, context.options.field);
    if (value === undefined) {
      return;
    }
    context.stdout.write(typeof value === "string" ? `${value}\n` : `${JSON.stringify(value)}\n`);
    return;
  }
  context.stdout.write(`${JSON.stringify(envelope, null, 2)}\n`);
}

function successEnvelope(context, result) {
  return {
    ok: true,
    data: result.data,
    warnings: result.warnings || [],
    meta: baseMeta(context, result.meta),
  };
}

function errorEnvelope(context, error) {
  return {
    ok: false,
    error: {
      code: error.code,
      normalized_code: error.normalizedCode,
      message: error.message,
      ...(error.hint ? { hint: error.hint } : {}),
      ...(error.docsUrl ? { docs_url: error.docsUrl } : {}),
    },
    warnings: [],
    meta: baseMeta(context, {
      request_id: error.requestId,
      status: error.status,
    }),
  };
}

function baseMeta(context, meta = {}) {
  const command = context.commandParts.join(" ") || (context.options.version ? "--version" : "help");
  return {
    ...(meta.request_id ? { request_id: meta.request_id } : {}),
    base_url: context.options.baseUrl,
    cli_version: CLI_VERSION,
    command,
    source: "cli",
    telemetry: context.options.telemetry,
    ...(meta.pagination && Object.keys(meta.pagination).length > 0 ? { pagination: meta.pagination } : {}),
    ...(meta.rate_limit && Object.keys(meta.rate_limit).length > 0 ? { rate_limit: meta.rate_limit } : {}),
    ...(meta.status ? { status: meta.status } : {}),
  };
}

function selectField(value, path) {
  const normalized = path.replace(/^\$?\./, "");
  if (!normalized) {
    return value;
  }
  return normalized.split(".").reduce((current, part) => {
    if (current == null) {
      return undefined;
    }
    return current[part];
  }, value);
}

function renderInit(workspace, profiles, config, nextActions) {
  const lines = [
    "UniPost init",
    `Workspace: ${workspace?.id || "unknown"}`,
    `Profiles: ${profiles.length}`,
  ];
  if (config.default_profile_id) {
    lines.push(`Default profile: ${config.default_profile_id}`);
  }
  for (const action of nextActions) {
    lines.push(`Next: ${action}`);
  }
  return `${lines.join("\n")}\n`;
}

function renderQuickstart(workspace, profile, accountList, steps) {
  const lines = [
    "UniPost quickstart",
    `Workspace: ${workspace?.id || "unknown"}`,
    `Profile: ${profile?.id || "not selected"}`,
    `Accounts: ${accountList.length}`,
  ];
  for (const step of steps) {
    lines.push(`Next: ${step}`);
  }
  return `${lines.join("\n")}\n`;
}

function renderProfiles(profileList) {
  if (profileList.length === 0) {
    return "No profiles found.\n";
  }
  return `${profileList.map((profile) => {
    const count = profile.account_count ?? 0;
    return `${profile.id}\t${profile.name || ""}\t${count} accounts`;
  }).join("\n")}\n`;
}

function renderConnectSession(session) {
  const lines = [
    `${session.id || "connect_session"}\t${session.platform || ""}\t${session.status || ""}`,
  ];
  if (session.url) {
    lines.push(session.url);
  }
  if (session.completed_social_account_id) {
    lines.push(`Account: ${session.completed_social_account_id}`);
  }
  return `${lines.join("\n")}\n`;
}

function renderAccounts(accountList) {
  if (accountList.length === 0) {
    return "No accounts found.\n";
  }
  return `${accountList.map((account) => {
    const name = account.account_name || account.accountName || "";
    return `${account.id}\t${account.platform || ""}\t${name}\t${account.status || ""}`;
  }).join("\n")}\n`;
}

function renderAccountDiagnostic(kind, accountID, payload) {
  if (kind === "health") {
    return `${accountID}\t${payload.platform || ""}\t${payload.status || ""}${payload.last_error?.code ? `\t${payload.last_error.code}` : ""}\n`;
  }
  if (kind === "capabilities") {
    return `${accountID}\t${payload.platform || ""}\tcapabilities loaded\n`;
  }
  if (kind === "metrics") {
    return `${accountID}\t${payload.platform || ""}\tfollowers=${payload.follower_count ?? ""}\tposts=${payload.post_count ?? ""}\n`;
  }
  return `${accountID}\t${kind}\n`;
}

function renderPosts(postList) {
  if (postList.length === 0) {
    return "No posts found.\n";
  }
  return `${postList.map((post) => `${post.id}\t${post.status || ""}\t${post.caption || ""}`.trim()).join("\n")}\n`;
}

function renderPost(post) {
  if (!post) {
    return "Post not found.\n";
  }
  return `${post.id || "post"}\t${post.status || ""}${post.caption ? `\t${post.caption}` : ""}\n`;
}

function renderMedia(mediaItem) {
  if (!mediaItem) {
    return "Media not found.\n";
  }
  const lines = [
    `${mediaItem.id || "media"}\t${mediaItem.status || ""}\t${mediaItem.content_type || ""}\t${mediaItem.size_bytes ?? ""}`.trim(),
  ];
  if (mediaReady(mediaItem)) {
    lines.push(`Next: add "${mediaItem.id}" to media_ids in post.json, then run unipost posts create --from-file post.json --dry-run --json`);
  }
  return `${lines.join("\n")}\n`;
}

function renderDoctor(checks, workspace, telemetry) {
  const lines = ["UniPost doctor\n"];
  for (const check of checks) {
    lines.push(`${check.status.toUpperCase()} ${check.id}: ${check.message}`);
    if (check.hint) {
      lines.push(`  ${check.hint}`);
    }
  }
  if (workspace?.id) {
    lines.push(`Workspace: ${workspace.id}`);
  }
  lines.push(`Telemetry: ${telemetry.enabled ? "enabled" : "disabled"} (${telemetry.reason})`);
  return `${lines.join("\n")}\n`;
}

function helpText() {
  return `UniPost CLI ${CLI_VERSION}

Usage:
  unipost --version
  unipost init [--json]
  unipost quickstart [--json] [--name <profile-name>]
  unipost config path|show [--json]
  unipost config set base_url <url> [--json]
  unipost config set default_profile_id <profile_id> [--json]
  unipost auth login --setup-token <token> [--client <client>] [--json]
  unipost auth login --api-key <key> [--json]
  unipost auth logout [--json]
  unipost auth status [--json] [--api-key <key>] [--base-url <url>]
  unipost profiles list|create|use [--json]
  unipost connect create|get|wait [--json]
  unipost accounts list|get|health|capabilities|metrics [--json]
  unipost posts list|get|analytics [--json]
  unipost posts validate|draft --account <id> --caption <text> [--json]
  unipost posts create --account <id> --caption <text> --yes --idempotency-key <key>
  unipost posts create --from-file post.json --dry-run [--json]
  unipost posts schedule --account <id> --caption <text> --at <timestamp> --yes --idempotency-key <key>
  unipost posts wait|cancel|retry <post_id> [--json]
  unipost media upload <file_path> [--content-type <mime>] [--json]
  unipost media get|wait <media_id> [--json]
  unipost analytics summary|posts|platforms|platform [--json]
  unipost examples posts.create --lang <curl|node>
  unipost examples mcp.claude-code
  unipost agent plan --intent <intent> [--json]
  unipost agent execute --plan plan.json [--json]
  unipost agent bootstrap [--setup-token <token>] [--client <client>] [--json]
  unipost agent capabilities|context|guide [--json]
  unipost agent mcp-config|mcp-test|install [--client <client>] [--json]
  unipost doctor [--json] [--api-key <key>] [--base-url <url>]
  unipost completion <bash|zsh|fish>

Global flags:
  --json, --output <table|json|yaml>, --field <field>, --non-interactive
  --base-url <url>, --api-key <key>, --setup-token <token>, --limit <n>, --cursor <cursor>, --all
  --client <codex|claude-code|cursor|windsurf>, --profile <id>, --account <id>
  --platform <name>, --caption <text>, --status <status>, --result <id>
  --from <date>, --to <date>, --at <timestamp>, --schedule-at <timestamp>
  --from-file <path>, --plan <path>, --content-type <mime>, --yes
  --idempotency-key <key>, --agent-name <name>
  --no-color, --no-telemetry

Auth note:
  auth login --setup-token exchanges a Dashboard-generated short-lived token for
  a named API key and stores the secret in OS keychain. auth login --api-key
  remains metadata-only and never writes the plaintext key to config.
`;
}

function resolveTelemetry(options, env) {
  if (options.noTelemetry || env.UNIPOST_TELEMETRY === "0" || env.UNIPOST_TELEMETRY === "false") {
    return { enabled: false, reason: "disabled" };
  }
  if (env.UNIPOST_TELEMETRY === "1" || env.UNIPOST_TELEMETRY === "true") {
    return { enabled: true, reason: "env_enabled" };
  }
  return { enabled: false, reason: "default_off" };
}

function normalizeBaseUrl(value) {
  const trimmed = String(value || DEFAULT_BASE_URL).trim();
  return trimmed.replace(/\/+$/, "");
}

function toYaml(value, depth = 0) {
  const indent = "  ".repeat(depth);
  if (Array.isArray(value)) {
    return value.map((item) => `${indent}- ${formatYamlValue(item, depth + 1)}`).join("\n") + "\n";
  }
  if (value && typeof value === "object") {
    return Object.entries(value).map(([key, item]) => {
      if (item && typeof item === "object") {
        return `${indent}${key}:\n${toYaml(item, depth + 1).replace(/\n$/, "")}`;
      }
      return `${indent}${key}: ${formatScalar(item)}`;
    }).join("\n") + "\n";
  }
  return `${indent}${formatScalar(value)}\n`;
}

function formatYamlValue(value, depth) {
  if (value && typeof value === "object") {
    return `\n${toYaml(value, depth).replace(/\n$/, "")}`;
  }
  return formatScalar(value);
}

function formatScalar(value) {
  if (value === null) return "null";
  if (value === undefined) return "";
  if (typeof value === "string") return JSON.stringify(value);
  return String(value);
}

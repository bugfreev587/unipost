const CLI_VERSION = "0.1.0";
const DEFAULT_BASE_URL = "https://api.unipost.dev";
const DOCS_QUICKSTART_URL = "https://unipost.dev/docs/quickstart";
const DOCS_CLI_URL = "https://unipost.dev/docs/cli";

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
  "--limit",
  "--cursor",
  "--lang",
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
  const telemetry = resolveTelemetry(parsed.options, env);

  return {
    argv,
    commandParts: parsed.commandParts,
    options: {
      ...parsed.options,
      output,
      baseUrl,
      apiKey: parsed.options.apiKey || env.UNIPOST_API_KEY || "",
      noColor: Boolean(parsed.options.noColor || env.NO_COLOR),
      telemetry,
    },
    env,
    stdout: io.stdout || process.stdout,
    stderr: io.stderr || process.stderr,
    fetchImpl: io.fetchImpl || globalThis.fetch,
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
      telemetry: resolveTelemetry({}, env),
    },
    env,
    stdout: io.stdout || process.stdout,
    stderr: io.stderr || process.stderr,
    fetchImpl: io.fetchImpl || globalThis.fetch,
  };
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
  options[key] = value;
}

function toCamelCase(value) {
  return value.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
}

async function dispatch(context) {
  const [command, subcommand] = context.commandParts;

  if (context.options.version) {
    return textResult(`${CLI_VERSION}\n`);
  }
  if (context.options.help || !command) {
    return textResult(helpText());
  }
  if (command === "auth" && subcommand === "status") {
    return authStatus(context);
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
  const workspace = response.body?.data ?? response.body;

  return envelopeResult({
    data: {
      authenticated: true,
      credential_source: context.options.apiKey ? "flag" : "env",
      workspace,
    },
    meta: {
      request_id: response.requestId,
      rate_limit: response.rateLimit,
    },
    human: `Authenticated with workspace ${workspace?.id || "unknown"}.\n`,
  });
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

function requireApiKey(context, command) {
  if (context.options.apiKey) {
    return;
  }
  throw new CliError({
    code: "unauthorized",
    normalizedCode: "unauthorized",
    message: "API key is missing.",
    hint: "Set UNIPOST_API_KEY or pass --api-key. Browser/device login is planned for a later phase.",
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
  };

  if (auth) {
    headers.Authorization = `Bearer ${context.options.apiKey}`;
  }

  const retryable = method === "GET" || method === "HEAD";
  const maxAttempts = retryable ? 3 : 1;
  let lastError = null;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    let response;
    try {
      response = await context.fetchImpl(url, { method, headers });
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
    'auth status:Verify the configured UniPost credential'
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
  local words="auth status doctor completion --json --output --field --base-url --api-key --client --limit --cursor --all --non-interactive --no-color --no-telemetry"
  COMPREPLY=($(compgen -W "$words" -- "\${COMP_WORDS[COMP_CWORD]}"))
}
complete -F _unipost_completion unipost
`;
}

function fishCompletion() {
  return `complete -c unipost -f
complete -c unipost -a "auth status" -d "Verify the configured UniPost credential"
complete -c unipost -a doctor -d "Run local and API diagnostics"
complete -c unipost -a completion -d "Generate shell completion"
complete -c unipost -l json -d "Output the stable JSON envelope"
complete -c unipost -l client -xa "codex claude-code cursor windsurf"
complete -c unipost -l limit -d "Page size for list commands"
complete -c unipost -l cursor -d "Page cursor for list commands"
complete -c unipost -l all -d "Follow pagination until exhausted"
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
  unipost auth status [--json] [--api-key <key>] [--base-url <url>]
  unipost doctor [--json] [--api-key <key>] [--base-url <url>]
  unipost completion <bash|zsh|fish>

Global flags:
  --json, --output <table|json|yaml>, --field <field>, --non-interactive
  --base-url <url>, --api-key <key>, --limit <n>, --cursor <cursor>, --all
  --client <codex|claude-code|cursor|windsurf>, --no-color, --no-telemetry
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

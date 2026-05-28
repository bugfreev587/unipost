#!/usr/bin/env node
import { fetchReviewScript, DEFAULT_API_URL } from "./client.js";
import { runDoctor, printDoctor } from "./doctor.js";
import { createAgentReporter } from "./reporter.js";
import { runScript } from "./runner.js";

async function main(argv = process.argv.slice(2)) {
  const [command, ...rest] = argv;
  if (!command || command === "--help" || command === "-h") {
    printHelp();
    return 0;
  }
  if (command === "doctor") {
    printDoctor(runDoctor());
    return 0;
  }
  if (command === "run") {
    const checks = runDoctor();
    printDoctor(checks);
    const hardFailure = checks.find((check) => !check.ok && !check.warning);
    if (hardFailure) {
      throw new Error(`pre-flight check failed: ${hardFailure.id}`);
    }
    const args = parseArgs(rest);
    const token = args.token || process.env.UNIPOST_REVIEW_TOKEN;
    const sessionToken = args.sessionToken || process.env.UNIPOST_REVIEW_SESSION_TOKEN || "";
    const apiUrl = args.apiUrl || process.env.UNIPOST_API_URL || DEFAULT_API_URL;
    const script = await fetchReviewScript({ token, apiUrl });
    const reporter = args.dryRun ? null : createAgentReporter({ token, apiUrl });
    await runScript(script, {
      dryRun: args.dryRun,
      reporter,
      sessionToken,
      manualOAuthHandoff: args.manualOAuthHandoff,
      aiGuided: args.aiGuided,
      token,
      apiUrl,
      uploadFilePath: args.uploadFile || process.env.UNIPOST_REVIEW_UPLOAD_FILE || "",
    });
    return 0;
  }
  if (command === "resume") {
    throw new Error("resume is reserved for failed recording recovery and is not available in this build");
  }
  throw new Error(`unknown command: ${command}`);
}

function parseArgs(args) {
  const parsed = { dryRun: false };
  for (let i = 0; i < args.length; i += 1) {
    const arg = args[i];
    if (arg === "--token") parsed.token = args[++i];
    else if (arg === "--session-token") parsed.sessionToken = args[++i];
    else if (arg === "--api-url") parsed.apiUrl = args[++i];
    else if (arg === "--manual-oauth-handoff") parsed.manualOAuthHandoff = true;
    else if (arg === "--ai-guided") parsed.aiGuided = true;
    else if (arg === "--upload-file") parsed.uploadFile = args[++i];
    else if (arg === "--dry-run") parsed.dryRun = true;
    else throw new Error(`unknown option: ${arg}`);
  }
  return parsed;
}

function printHelp() {
  process.stdout.write(`UniPost Review Agent\n\nCommands:\n  run --token <token> --session-token <token> [--api-url <url>] [--manual-oauth-handoff] [--ai-guided] [--upload-file <path>] [--dry-run]\n  doctor\n  resume --job <job_id>\n`);
}

main().then((code) => {
  process.exitCode = code;
}).catch((err) => {
  process.stderr.write(`${err.message}\n`);
  process.exitCode = 1;
});

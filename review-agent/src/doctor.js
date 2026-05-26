export function runDoctor({ platform = process.platform, nodeVersion = process.versions.node } = {}) {
  const checks = [];
  const major = Number(String(nodeVersion).split(".")[0]);
  checks.push({
    id: "node",
    ok: Number.isFinite(major) && major >= 20,
    message: `Node.js ${nodeVersion}`,
    remediation: "Install Node.js 20 or newer.",
  });
  checks.push({
    id: "playwright",
    ok: true,
    message: "Playwright will be loaded when recording starts.",
    remediation: "If browser launch fails, run: npx playwright install chromium",
  });
  if (platform === "darwin") {
    checks.push({
      id: "macos-screen-recording",
      ok: false,
      warning: true,
      message: "macOS requires Screen Recording permission for the terminal app that runs this command.",
      remediation: "Open System Settings -> Privacy & Security -> Screen Recording, enable your terminal, then restart it.",
    });
  }
  return checks;
}

export function printDoctor(checks, out = process.stdout) {
  for (const check of checks) {
    const mark = check.ok ? "OK" : check.warning ? "WARN" : "FAIL";
    out.write(`${mark} ${check.id}: ${check.message}\n`);
    if (!check.ok && check.remediation) {
      out.write(`  ${check.remediation}\n`);
    }
  }
}

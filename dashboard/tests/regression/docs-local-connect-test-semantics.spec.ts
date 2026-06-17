import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";

test.describe("local Connect testing documentation", () => {
  test("places the page between Connect Sessions and Hosted Connect with local setup steps", async () => {
    const docsShellSource = await readFile(path.join(process.cwd(), "src/app/docs/_components/docs-shell.tsx"), "utf8");
    const pageSource = await readFile(path.join(process.cwd(), "src/app/docs/local-connect-test/page.tsx"), "utf8");

    const connectSessionsIndex = docsShellSource.indexOf('{ label: "Connect Sessions", href: "/docs/connect-sessions" }');
    const localTestingIndex = docsShellSource.indexOf('{ label: "Local Connect testing", href: "/docs/local-connect-test" }');
    const hostedConnectIndex = docsShellSource.indexOf('{ label: "Hosted Connect (White-label branding)", href: "/docs/white-label" }');

    expect(connectSessionsIndex).toBeGreaterThan(-1);
    expect(localTestingIndex).toBeGreaterThan(connectSessionsIndex);
    expect(hostedConnectIndex).toBeGreaterThan(localTestingIndex);
    expect(docsShellSource).toContain('current === "/docs/local-connect-test"');

    expect(pageSource).toContain('title="Test Connect locally"');
    expect(pageSource).toContain("/docs/downloads/create_connect_session_url.py");
    expect(pageSource).toContain("export UNIPOST_DOCS_ORIGIN='https://dev.unipost.dev'");
    expect(pageSource).toContain("curl -fL");
    expect(pageSource).toContain("python3 -m py_compile create_connect_session_url.py");
    expect(pageSource).toContain("Dashboard -> Developer -> API Keys");
    expect(pageSource).toContain("Dashboard -> Profile");
    expect(pageSource).toContain("GET /v1/profiles");
    expect(pageSource).toContain("export UNIPOST_API_KEY='up_live_xxx'");
    expect(pageSource).toContain("export PROFILE_ID='16202f3f-0c3c-4b92-afae-177f279c692a'");
    expect(pageSource).toContain("export PLATFORM='youtube'");
    expect(pageSource).toContain("--platform \"$PLATFORM\"");
    expect(pageSource).toContain("--profile-id \"$PROFILE_ID\"");
    expect(pageSource).toContain("Connection session URL:");
    expect(pageSource).toContain("Copy this URL into your browser");
    expect(pageSource).toContain(".lct-summary>div{border:1px solid var(--docs-border)");
    expect(pageSource).toContain("background:var(--docs-bg-elevated)");
    expect(pageSource).not.toContain("background:#ffffff");
  });
});

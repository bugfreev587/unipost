import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";

test.describe("white-label documentation semantics", () => {
  test("separates Hosted Connect branding from platform credentials", async () => {
    const whiteLabelSource = await readFile(path.join(process.cwd(), "src/app/docs/white-label/page.tsx"), "utf8");
    const credentialsSource = await readFile(path.join(process.cwd(), "src/app/docs/api/white-label/credentials/page.tsx"), "utf8");
    const connectSessionsSource = await readFile(path.join(process.cwd(), "src/app/docs/connect-sessions/page.tsx"), "utf8");

    expect(whiteLabelSource).toContain("Hosted Connect Profile");
    expect(whiteLabelSource).toContain("Platform Credentials");
    expect(whiteLabelSource).toContain("You can combine either Hosted Connect profile with either credential source.");
    expect(whiteLabelSource).not.toContain("White-label uses your uploaded platform credentials instead");
    expect(whiteLabelSource).not.toContain("Paid plans only");

    expect(credentialsSource).toContain("Platform Credentials are separate from Hosted Connect branding");
    expect(credentialsSource).not.toContain("Call this once per platform during white-label onboarding.");

    expect(connectSessionsSource).toContain("Shared UniPost OAuth app");
    expect(connectSessionsSource).toContain("Workspace platform credentials");
    expect(connectSessionsSource).not.toContain("white-label credentials");
  });
});

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const root = process.cwd();

describe("notification email preferences source contract", () => {
  it("exposes dashboard API helpers for category-level email preferences", () => {
    const api = readFileSync(join(root, "src/lib/api.ts"), "utf8");

    expect(api).toContain("EmailPreferenceCategory");
    expect(api).toContain("listEmailPreferences");
    expect(api).toContain("updateEmailPreference");
    expect(api).toContain("/v1/me/notifications/email-preferences");
  });

  it("keeps email preferences separate from the Slack/Discord subscription matrix", () => {
    const page = readFileSync(join(root, "src/app/(dashboard)/settings/notifications/page.tsx"), "utf8");

    expect(page).toContain("emailPreferences");
    expect(page).toContain("Email preferences");
    expect(page).toContain("matrixChannels");
    expect(page).toContain('channel.kind !== "email"');
    expect(page).toContain("Publishing failure alerts");
    expect(page).toContain("Account connection alerts");
  });

  it("shows admin email policy fields in the admin email page", () => {
    const api = readFileSync(join(root, "src/lib/api.ts"), "utf8");
    const page = readFileSync(join(root, "src/app/admin/email/page.tsx"), "utf8");

    for (const token of ["preference_category", "footer_policy", "preference_decision"]) {
      expect(api).toContain(token);
      expect(page).toContain(token);
    }
  });
});

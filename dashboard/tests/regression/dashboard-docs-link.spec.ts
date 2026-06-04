import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { getDashboardDocsHref } from "../../src/lib/dashboard-docs-link";

test.describe("dashboard docs link", () => {
  test("does not hardcode production docs in the dashboard shell", async () => {
    const shellSource = await readFile(
      path.join(process.cwd(), "src/components/dashboard/shell.tsx"),
      "utf8",
    );

    expect(shellSource).not.toContain('href="https://unipost.dev/docs"');
  });

  test("maps dashboard app origins to matching docs origins", () => {
    expect(
      getDashboardDocsHref({
        landingUrl: "",
        baseUrl: "",
        appUrl: "https://dev-app.unipost.dev",
      }),
    ).toBe("https://dev.unipost.dev/docs/quickstart");

    expect(
      getDashboardDocsHref({
        landingUrl: "",
        baseUrl: "",
        currentOrigin: "https://staging-app.unipost.dev",
      }),
    ).toBe("https://staging.unipost.dev/docs/quickstart");

    expect(
      getDashboardDocsHref({
        landingUrl: "",
        baseUrl: "",
        appUrl: "https://app.unipost.dev",
      }),
    ).toBe("https://unipost.dev/docs/quickstart");
  });

  test("prefers an explicit landing docs origin when configured", () => {
    expect(
      getDashboardDocsHref({
        landingUrl: "https://dev.unipost.dev",
        baseUrl: "",
        appUrl: "https://app.unipost.dev",
      }),
    ).toBe("https://dev.unipost.dev/docs/quickstart");
  });

  test("does not let a production base fallback override a dev app URL", () => {
    expect(
      getDashboardDocsHref({
        landingUrl: "",
        baseUrl: "https://unipost.dev",
        appUrl: "https://dev-app.unipost.dev",
      }),
    ).toBe("https://dev.unipost.dev/docs/quickstart");
  });

  test("prefers current app origin before CI app URL fallback", () => {
    const previousAppUrl = process.env.NEXT_PUBLIC_APP_URL;
    process.env.NEXT_PUBLIC_APP_URL = "http://localhost:3000";

    try {
      expect(
        getDashboardDocsHref({
          landingUrl: "",
          baseUrl: "",
          currentOrigin: "https://staging-app.unipost.dev",
        }),
      ).toBe("https://staging.unipost.dev/docs/quickstart");
    } finally {
      if (previousAppUrl === undefined) {
        delete process.env.NEXT_PUBLIC_APP_URL;
      } else {
        process.env.NEXT_PUBLIC_APP_URL = previousAppUrl;
      }
    }
  });
});

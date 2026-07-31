import { expect, test, type Page, type TestInfo } from "@playwright/test";

import {
  bootstrapSyntheticUser,
  cleanupSyntheticUser,
  createSyntheticClerkUser,
  loadSyntheticAuthConfig,
  runWithCleanup,
  signInSyntheticUser,
} from "./support/synthetic-auth.mjs";

const config = loadSyntheticAuthConfig();

test("core dashboard routes load and preserve plan-gated navigation with a passwordless Clerk ticket", async ({ page }, testInfo) => {
  let identity: Awaited<ReturnType<typeof createSyntheticClerkUser>> | undefined;
  let authState: Awaited<ReturnType<typeof bootstrapSyntheticUser>> | undefined;

  await runWithCleanup(
    async () => {
      if (!identity) return;
      await cleanupSyntheticUser(config, identity, authState);
    },
    async () => {
      identity = await createSyntheticClerkUser(config);
      await signInSyntheticUser(page, config, identity);
      authState = await bootstrapSyntheticUser(page, config);
      const profileID = authState.profileID;
      const youtubeAccountFixture = await installYouTubeAccountFixture(page, profileID);
      await installYouTubePostFixture(page, profileID);

      await expectDashboardRoute(page, `/projects/${profileID}`);
      await expectYouTubeAccountIdentity(page, profileID, testInfo, youtubeAccountFixture);
      await expectYouTubeResultLinkValidation(page, profileID);
      await expectPinterestPublishingGuards(page, profileID);
      await expectDashboardRoute(page, `/projects/${profileID}/analytics`);
      await expectDashboardRoute(page, `/projects/${profileID}/settings`);

      await expectPlanGatedAnalyticsRoute(page, `/projects/${profileID}/analytics/platforms/tiktok`);
      await expectPlanGatedAnalyticsRoute(page, `/projects/${profileID}/analytics/platforms/youtube`);
    },
  );
});

async function installYouTubeAccountFixture(page: Page, profileID: string) {
  let accountRequests = 0;

  await page.route("**/v1/profiles", async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [{
          id: profileID,
          workspace_id: "youtube-audit-workspace",
          name: "YouTube Audit Profile",
          created_at: "2026-07-29T12:00:00.000Z",
          updated_at: "2026-07-29T12:00:00.000Z",
        }],
      }),
    });
  });

  await page.route(`**/v1/profiles/${profileID}/accounts*`, async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    accountRequests += 1;

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [
          {
            id: "youtube-audit-fixture",
            profile_id: profileID,
            platform: "youtube",
            account_name: "UniPost Audit Channel",
            external_account_id: "UCunipostAuditFixture",
            account_avatar_url: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='80' height='80'%3E%3Crect width='80' height='80' rx='40' fill='%23334155'/%3E%3Ctext x='40' y='48' text-anchor='middle' font-size='24' fill='white'%3EUA%3C/text%3E%3C/svg%3E",
            connected_at: "2026-07-29T12:00:00.000Z",
            status: "active",
            connection_type: "byo",
            scope: ["youtube.readonly", "yt-analytics.readonly"],
          },
          {
            id: "youtube-broken-avatar-fixture",
            profile_id: profileID,
            platform: "youtube",
            account_name: null,
            external_account_id: "UCbrokenAvatarFixture",
            account_avatar_url: "data:image/png;base64,broken",
            connected_at: "2026-07-29T12:00:00.000Z",
            status: "active",
            connection_type: "byo",
            scope: ["youtube.readonly"],
          },
          {
            id: "youtube-initials-fixture",
            profile_id: profileID,
            platform: "youtube",
            account_name: "Broken Avatar Channel",
            external_account_id: "UCbrokenAvatarWithNameFixture",
            account_avatar_url: "data:image/png;base64,also-broken",
            connected_at: "2026-07-29T12:00:00.000Z",
            status: "active",
            connection_type: "byo",
            scope: ["youtube.readonly"],
          },
          {
            id: "youtube-disconnected-fixture",
            profile_id: profileID,
            platform: "youtube",
            account_name: null,
            external_account_id: "disconnected:youtube-disconnected-fixture",
            account_avatar_url: null,
            connected_at: "2026-07-29T12:00:00.000Z",
            status: "disconnected",
            connection_type: "byo",
            scope: [],
          },
          {
            id: "pinterest-audit-primary",
            profile_id: profileID,
            platform: "pinterest",
            account_name: "Pinterest Primary",
            external_account_id: "pinterest-primary-user",
            account_avatar_url: null,
            connected_at: "2026-07-29T12:00:00.000Z",
            status: "active",
            connection_type: "byo",
            scope: ["boards:read", "boards:write", "pins:write", "user_accounts:read"],
          },
          {
            id: "pinterest-audit-secondary",
            profile_id: profileID,
            platform: "pinterest",
            account_name: "Pinterest Secondary",
            external_account_id: "pinterest-secondary-user",
            account_avatar_url: null,
            connected_at: "2026-07-29T12:00:00.000Z",
            status: "active",
            connection_type: "byo",
            scope: ["boards:read", "boards:write", "pins:write", "user_accounts:read"],
          },
        ],
      }),
    });
  });

  return {
    getAccountRequests: () => accountRequests,
  };
}

function youtubePostFixture(profileID: string) {
  return {
    id: "youtube-result-fixture",
    caption: "YouTube source validation fixture",
    status: "published",
    created_at: "2026-07-29T12:00:00.000Z",
    published_at: "2026-07-29T12:01:00.000Z",
    source: "ui",
    profile_ids: [profileID],
    target_platforms: ["youtube"],
    results: [
      {
        id: "youtube-valid-result",
        social_account_id: "youtube-audit-fixture",
        platform: "youtube",
        account_name: "Valid YouTube result",
        status: "published",
        url: "https://www.youtube.com/watch?v=unipostAuditFixture",
        published_at: "2026-07-29T12:01:00.000Z",
      },
      {
        id: "youtube-invalid-result",
        social_account_id: "youtube-audit-fixture",
        platform: "youtube",
        account_name: "Invalid YouTube result",
        status: "published",
        url: "https://example.com/not-a-youtube-destination",
        published_at: "2026-07-29T12:01:00.000Z",
      },
      {
        id: "pinterest-board-failure-result",
        social_account_id: "pinterest-audit-primary",
        platform: "pinterest",
        account_name: "Pinterest Board Failure",
        status: "failed",
        error_message: "provider prose must not choose the customer guidance",
        error_code: "target_not_found",
        failure_stage: "destination_preflight",
        platform_error_code: "29",
        is_retriable: false,
        next_action: "select_valid_target",
        error_source: "platform",
        error_temporality: "permanent",
        provider_error: { provider: "pinterest", http_status: 403, code: "29" },
      },
      {
        id: "pinterest-token-failure-result",
        social_account_id: "pinterest-audit-primary",
        platform: "pinterest",
        account_name: "Pinterest Token Failure",
        status: "failed",
        error_message: "board access denied",
        error_code: "auth_token_invalid",
        failure_stage: "destination_preflight",
        platform_error_code: "2",
        is_retriable: false,
        next_action: "reconnect_account",
        error_source: "platform",
        error_temporality: "permanent",
        provider_error: { provider: "pinterest", http_status: 401, code: "2" },
      },
      {
        id: "pinterest-media-failure-result",
        social_account_id: "pinterest-audit-primary",
        platform: "pinterest",
        account_name: "Pinterest Media Failure",
        status: "failed",
        error_message: "raw fetch response must not choose the customer guidance",
        error_code: "media_error",
        failure_stage: "media_preflight",
        is_retriable: false,
        next_action: "fix_media",
        error_source: "unipost",
        error_temporality: "permanent",
      },
    ],
  };
}

async function installYouTubePostFixture(page: Page, profileID: string) {
  const post = youtubePostFixture(profileID);

  await page.route("**/v1/posts?*", async (route) => {
    if (route.request().method() !== "GET") {
      await route.continue();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: [post],
        meta: { has_more: false, next_cursor: "", total: 1 },
      }),
    });
  });

  await page.route("**/v1/posts/youtube-result-fixture/queue", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: { post, jobs: [] } }),
    });
  });
}

async function expectYouTubeAccountIdentity(
  page: Page,
  profileID: string,
  testInfo: TestInfo,
  fixture: { getAccountRequests: () => number },
) {
  await expectDashboardRoute(page, `/projects/${profileID}/accounts`);
  await expect.poll(fixture.getAccountRequests).toBeGreaterThan(0);

  const identity = page.locator('[data-youtube-account-id="youtube-audit-fixture"]');
  const avatar = identity.locator("[data-youtube-channel-avatar]");
  const sourceLink = identity.locator("[data-youtube-source-link]");
  const brokenIdentity = page.locator('[data-youtube-account-id="youtube-broken-avatar-fixture"]');
  const initialsIdentity = page.locator('[data-youtube-account-id="youtube-initials-fixture"]');
  const disconnectedIdentity = page.locator('[data-youtube-account-id="youtube-disconnected-fixture"]');
  const status = identity.locator("xpath=ancestor::tr").locator("[data-unipost-account-status]");
  await expect(identity).toContainText("UniPost Audit Channel");
  await expect(avatar).toBeVisible();
  await expect.poll(
    () => avatar.evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0),
  ).toBe(true);
  await expect(sourceLink).toContainText("Source: YouTube");
  await expect(sourceLink).toHaveAttribute(
    "href",
    "https://www.youtube.com/channel/UCunipostAuditFixture",
  );
  await expect(sourceLink).toHaveAttribute("target", "_blank");
  await expect(sourceLink).toHaveAttribute("rel", "noopener noreferrer");
  await expect(status).toContainText("Active");

  await expect(brokenIdentity).toContainText("YouTube channel");
  await expect(brokenIdentity.locator("[data-youtube-channel-avatar-fallback]")).toBeVisible();
  await expect(brokenIdentity.locator("[data-youtube-destination-icon]")).toBeVisible();
  await expect(initialsIdentity).toContainText("Broken Avatar Channel");
  await expect(initialsIdentity.locator("[data-youtube-channel-avatar-fallback]")).toHaveText("BA");
  await expect(initialsIdentity.locator("[data-youtube-destination-icon]")).toHaveCount(0);
  await expect(disconnectedIdentity).toContainText("Disconnected YouTube channel");
  await expect(disconnectedIdentity.locator("[data-youtube-source-link]")).toHaveCount(0);

  const sourceLinkBox = await sourceLink.boundingBox();
  expect(sourceLinkBox?.height).toBeGreaterThanOrEqual(44);
  await expect(page.locator("[data-youtube-destination-icon]").first()).toBeVisible();
  await expect(page.locator("[data-youtube-official-mark]")).toHaveCount(0);
  await expect(page.locator('path[fill="#ff0000"]')).toHaveCount(0);

  await sourceLink.focus();
  await expect(sourceLink).toBeFocused();
  expect(await sourceLink.evaluate((element) => getComputedStyle(element).outlineStyle)).not.toBe("none");

  await setDashboardTheme(page, "light");
  await testInfo.attach("youtube-accounts-light", {
    body: await page.screenshot({ fullPage: true }),
    contentType: "image/png",
  });
  await setDashboardTheme(page, "dark");
  await testInfo.attach("youtube-accounts-dark", {
    body: await page.screenshot({ fullPage: true }),
    contentType: "image/png",
  });

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(identity).toBeVisible();
  await expect(sourceLink).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= document.documentElement.clientWidth,
    ),
  ).toBe(true);
  await testInfo.attach("youtube-accounts-mobile", {
    body: await page.screenshot({ fullPage: true }),
    contentType: "image/png",
  });
  await page.setViewportSize({ width: 1280, height: 900 });
}

async function setDashboardTheme(page: Page, theme: "light" | "dark") {
  const root = page.locator("html");
  if (await root.getAttribute("data-theme") !== theme) {
    await page.getByRole("button", { name: `Switch to ${theme} theme` }).click();
  }
  await expect(root).toHaveAttribute("data-theme", theme);
}

async function expectYouTubeResultLinkValidation(page: Page, profileID: string) {
  await expectDashboardRoute(
    page,
    `/projects/${profileID}/posts/list?post=youtube-result-fixture`,
  );

  const resultNamed = (name: string) =>
    page.locator(".posts-result-card", {
      has: page.locator(".posts-result-name", { hasText: new RegExp(`^${name}$`) }),
    });
  const validResult = resultNamed("Valid YouTube result");
  const invalidResult = resultNamed("Invalid YouTube result");
  const pinterestBoardFailure = resultNamed("Pinterest Board Failure");
  const pinterestTokenFailure = resultNamed("Pinterest Token Failure");
  const pinterestMediaFailure = resultNamed("Pinterest Media Failure");
  const validSourceLink = validResult.locator("[data-youtube-source-link]");

  await expect(validResult.locator("[data-youtube-destination-icon]")).toBeVisible();
  await expect(validSourceLink).toContainText("View on YouTube");
  await expect(validSourceLink).toHaveAttribute(
    "href",
    "https://www.youtube.com/watch?v=unipostAuditFixture",
  );
  await expect(invalidResult.locator("[data-youtube-destination-icon]")).toBeVisible();
  await expect(invalidResult.locator("[data-youtube-source-link]")).toHaveCount(0);
  await expect(pinterestBoardFailure).toContainText("Pinterest board unavailable");
  await expect(pinterestBoardFailure).toContainText("The selected Pinterest board is no longer available for this account. Choose another board and publish again.");
  await expect(pinterestBoardFailure).not.toContainText("provider prose must not choose");
  await expect(pinterestTokenFailure).toContainText("Reconnect Pinterest");
  await expect(pinterestTokenFailure).toContainText("Pinterest rejected this connection. Reconnect the account, then try again.");
  await expect(pinterestMediaFailure).toContainText("Pinterest media needs changes");
  await expect(pinterestMediaFailure).toContainText("Pinterest could not use this media. Replace it with a publicly available supported image or video.");
  await expect(pinterestMediaFailure).not.toContainText("raw fetch response must not choose");
  await expect(page.locator("[data-youtube-official-mark]")).toHaveCount(0);
}

async function expectPinterestPublishingGuards(page: Page, profileID: string) {
  type BoardState = { sandboxMode: boolean; boards: Array<{ id: string; name: string }> };
  const boardStates: Record<string, BoardState> = {
    "pinterest-audit-primary": {
      sandboxMode: false,
      boards: [{ id: "101", name: "Primary Production Board" }],
    },
    "pinterest-audit-secondary": {
      sandboxMode: false,
      boards: [{ id: "201", name: "Secondary Production Board" }],
    },
  };
  const boardReads: Record<string, number> = {};
  let createPostRequests = 0;

  await page.route("**/v1/platforms/capabilities", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { platforms: {} } }) });
  });
  await page.route("**/v1/publishing-restrictions", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ data: { restrictions: [] } }) });
  });
  await page.route("**/v1/profiles/*/accounts/*/pinterest/boards", async (route) => {
    const match = new URL(route.request().url()).pathname.match(/\/accounts\/([^/]+)\/pinterest\/boards$/);
    const accountID = match?.[1] || "";
    if (route.request().method() === "POST") {
      const board = { id: "301", name: "Created Sandbox Board" };
      boardStates[accountID] = { sandboxMode: true, boards: [board] };
      await route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ data: { board } }) });
      return;
    }
    boardReads[accountID] = (boardReads[accountID] || 0) + 1;
    const state = boardStates[accountID] || { sandboxMode: false, boards: [] };
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: { boards: state.boards, sandbox_mode: state.sandboxMode } }),
    });
  });
  await page.route("**/v1/media**", async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    if (request.method() === "POST" && pathname === "/v1/media") {
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          data: {
            id: "media-pinterest-regression",
            status: "pending",
            upload_url: `${new URL(page.url()).origin}/mock-pinterest-upload`,
          },
        }),
      });
      return;
    }
    if (request.method() === "GET" && pathname === "/v1/media/media-pinterest-regression") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ data: { id: "media-pinterest-regression", status: "uploaded" } }),
      });
      return;
    }
    await route.continue();
  });
  await page.route("**/mock-pinterest-upload", async (route) => {
    await route.fulfill({ status: 200, headers: { "access-control-allow-origin": "*" }, body: "" });
  });
  await page.route("**/v1/posts/validate", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ data: { valid: true, errors: [], warnings: [] } }),
    });
  });
  await page.route("**/v1/posts", async (route) => {
    if (route.request().method() !== "POST") {
      await route.continue();
      return;
    }
    createPostRequests += 1;
    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({ data: { id: "unexpected-pinterest-post", results: [] } }),
    });
  });

  await expectDashboardRoute(page, `/projects/${profileID}/posts`);
  const createPostButton = page.getByRole("button", { name: "Create +" });
  await expect(createPostButton).toBeEnabled();
  await createPostButton.click();
  await expect(page.getByRole("heading", { name: "Create post" })).toBeVisible();
  await page.getByRole("button", { name: "Select Pinterest account Pinterest Primary" }).click();
  await expect.poll(() => boardReads["pinterest-audit-primary"] || 0).toBeGreaterThan(0);

  const boardSelect = page.locator("select").filter({ hasText: /Pinterest board|Board/ }).first();
  await expect(boardSelect.locator('option[value="101"]')).toHaveText("Primary Production Board");
  await boardSelect.selectOption("101");

  boardStates["pinterest-audit-primary"] = {
    sandboxMode: true,
    boards: [{ id: "111", name: "Primary Sandbox Board" }],
  };
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(boardSelect).toHaveValue("");
  await expect(boardSelect.locator('option[value="111"]')).toHaveText("Primary Sandbox Board");

  boardStates["pinterest-audit-primary"] = { sandboxMode: true, boards: [] };
  await page.getByRole("button", { name: "Refresh", exact: true }).click();
  await expect(page.getByText("This Pinterest account has no available boards. Create a board before publishing a Pin.", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Publish now" })).toBeDisabled();

  await page.getByPlaceholder("Sandbox test board").fill("Created Sandbox Board");
  await page.getByRole("button", { name: "Create", exact: true }).click();
  await expect(boardSelect).toHaveValue("301");
  await expect(boardSelect.locator('option[value="301"]')).toHaveText("Created Sandbox Board");

  await page.getByRole("button", { name: "Unselect Pinterest account Pinterest Primary" }).click();
  await page.getByRole("button", { name: "Select Pinterest account Pinterest Secondary" }).click();
  await expect.poll(() => boardReads["pinterest-audit-secondary"] || 0).toBeGreaterThan(0);
  await expect(boardSelect).toHaveValue("");
  await expect(boardSelect.locator('option[value="201"]')).toHaveText("Secondary Production Board");
  await boardSelect.selectOption("201");

  await page.getByPlaceholder("What's on your mind?").fill("Pinterest stale-board regression");
  await page.locator('input[type="file"]').setInputFiles({
    name: "pinterest-regression.png",
    mimeType: "image/png",
    buffer: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", "base64"),
  });
  await expect(page.getByText("pinterest-regression.png", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Publish now" })).toBeEnabled();

  await page.evaluate(() => {
    const staleNow = Date.now() + 61_000;
    Date.now = () => staleNow;
  });
  boardStates["pinterest-audit-secondary"] = { sandboxMode: false, boards: [] };
  await page.getByRole("button", { name: "Publish now" }).click();
  await expect(page.getByText("The selected Pinterest board is no longer available for this account. Choose another board and publish again.", { exact: true })).toBeVisible();
  await expect(boardSelect).toHaveValue("");
  expect(createPostRequests).toBe(0);
}

async function expectDashboardRoute(page: Page, path: string) {
  const failedRequests: string[] = [];
  const recordFailure = (response: { status(): number; url(): string }) => {
    if (response.status() >= 500) failedRequests.push(`${response.status()} ${response.url()}`);
  };
  page.on("response", recordFailure);
  try {
    await page.goto(path, { waitUntil: "domcontentloaded" });
    await expect(page.locator("body")).toContainText(/Navigate|Settings|Posts|Analytics|Connections|Profiles/);
    expect(failedRequests).toEqual([]);
  } finally {
    page.off("response", recordFailure);
  }
}

async function expectPlanGatedAnalyticsRoute(page: Page, path: string) {
  await expectDashboardRoute(page, path);
  await expect(page).toHaveURL(new RegExp(`${path.replaceAll("/", "\\/")}$`));
  await expect(page.getByText("Analytics is a paid plan feature", { exact: true })).toBeVisible();
}

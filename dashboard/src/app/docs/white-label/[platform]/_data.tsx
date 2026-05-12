export type WhiteLabelGuide = {
  slug: "meta" | "linkedin" | "tiktok" | "youtube" | "twitter";
  name: string;
  title: string;
  lead: string;
  portalName: string;
  portalUrl: string;
  dashboardCard: string;
  clientIdLabel: string;
  clientSecretLabel: string;
  callbacks: string[];
  bestFor: string;
  appReview: string;
  beforeYouStart: string[];
  screenshotSteps?: Array<{ title: string; caption?: string; image: string }>;
  apiWorkflow?: {
    title: string;
    intro: string;
    steps: Array<{
      title: string;
      body: string;
      snippets?: Array<{ lang: string; label: string; code: string }>;
    }>;
  };
  consoleSteps?: Array<{ title: string; body: string }>;
  steps: Array<{ title: string; body: string }>;
  fieldMap: Array<[string, string, string]>;
  gotchas: Array<[string, string]>;
  doneChecklist: string[];
  relatedPlatformHref: string;
  relatedPlatformTitle: string;
};

export const WHITE_LABEL_GUIDES: Record<string, WhiteLabelGuide> = {
  meta: {
    slug: "meta",
    name: "Meta",
    title: "Meta White-label Setup",
    lead: "Use your own Meta app so Instagram, Threads, and Facebook OAuth prompts show your brand instead of UniPost. This is the guide to get one Meta app working inside UniPost quickly.",
    portalName: "Meta for Developers",
    portalUrl: "https://developers.facebook.com",
    dashboardCard: "Meta (Instagram / Threads)",
    clientIdLabel: "App ID",
    clientSecretLabel: "App Secret",
    callbacks: [
      "https://api.unipost.dev/v1/oauth/callback/instagram",
      "https://api.unipost.dev/v1/oauth/callback/threads",
      "https://api.unipost.dev/v1/oauth/callback/facebook",
    ],
    bestFor: "Products onboarding customer-owned Instagram, Threads, or Facebook assets with a single branded Meta app.",
    appReview: "Plan for App Review / Advanced Access before broad production rollout, especially if you need public customer onboarding.",
    beforeYouStart: [
      "A Meta Business account with access to Meta for Developers.",
      "A clear list of which surfaces you need first: Instagram, Threads, Facebook Pages, or a combination.",
      "A test business asset you can safely reconnect more than once during setup.",
      "Your production brand name and logo, because Meta surfaces them during review and consent.",
    ],
    steps: [
      {
        title: "Create one Meta app for your first launch surface",
        body: "Start with the smallest surface set you need right now. A single Meta app can back multiple UniPost flows, but setup is faster if you first prove one happy path with a test asset.",
      },
      {
        title: "Add every UniPost callback you plan to use",
        body: "Meta setups are easiest when you whitelist all expected UniPost callback URLs up front. If your team will connect Instagram and Threads from the same app, add both callback URLs before testing.",
      },
      {
        title: "Copy the App ID and App Secret into UniPost",
        body: "Open the White-label Credentials screen in UniPost, find the Meta card, and paste the App ID and App Secret exactly as shown in Meta. Save once; you do not need separate credentials per profile field.",
      },
      {
        title: "Run one connection test with a real test asset",
        body: "Use an actual Instagram account, Threads profile, or Facebook Page you control. The goal is to confirm the consent screen shows your app and that UniPost returns to the workspace without an OAuth error.",
      },
      {
        title: "Only then expand scope and review work",
        body: "After one test asset connects end to end, add any extra products, permissions, or review submissions. This keeps your troubleshooting surface small while you prove the base wiring.",
      },
    ],
    fieldMap: [
      ["Meta app field", "UniPost field", "Notes"],
      ["App ID", "App ID", "Paste into the Meta credential card in UniPost."],
      ["App Secret", "App Secret", "Stored encrypted; UniPost never shows it back in the read view."],
      ["OAuth redirect allowlist", "Callback URLs below", "Add every Meta callback your rollout needs before testing."],
    ],
    gotchas: [
      ["One Meta app, multiple surfaces", "Instagram, Threads, and Facebook can share one Meta app, but each UniPost flow may use a different callback path. Whitelist all needed paths before you test."],
      ["Review can block scale", "A test user flow may work before full public rollout is approved. Do not assume one successful internal test means public customer onboarding is ready."],
      ["Use a real test asset", "Meta permissions often behave differently if the connected asset is incomplete or not properly linked to a business account."],
    ],
    doneChecklist: [
      "Your Meta app redirects back to UniPost without an OAuth error.",
      "The consent screen shows your app name, not UniPost.",
      "At least one target asset appears connected inside UniPost.",
      "You have recorded which of the three Meta callback URLs your rollout actually depends on.",
    ],
    relatedPlatformHref: "/docs/platforms/instagram",
    relatedPlatformTitle: "Instagram platform guide",
  },
  linkedin: {
    slug: "linkedin",
    name: "LinkedIn",
    title: "LinkedIn White-label Setup",
    lead: "LinkedIn is usually the fastest white-label platform to bring live. This guide is optimized for getting your own LinkedIn app connected in UniPost with minimal back-and-forth.",
    portalName: "LinkedIn Developer Portal",
    portalUrl: "https://developer.linkedin.com",
    dashboardCard: "LinkedIn",
    clientIdLabel: "Client ID",
    clientSecretLabel: "Client Secret",
    callbacks: ["https://api.unipost.dev/v1/oauth/callback/linkedin"],
    bestFor: "B2B products that need a low-friction first white-label rollout.",
    appReview: "The basic OIDC + posting flow is comparatively straightforward. Higher-tier LinkedIn products are where approval gets slower.",
    beforeYouStart: [
      "A company-owned LinkedIn developer account and company page context.",
      "One real LinkedIn member account you can use as the first connection test.",
      "A final app name you are comfortable showing to customers during consent.",
      "A note of whether you only need posting now, or also broader LinkedIn products later.",
    ],
    steps: [
      {
        title: "Create the LinkedIn app and keep the first scope set narrow",
        body: "For a fast first launch, focus on the base LinkedIn sign-in + posting path that UniPost already expects. Avoid adding unrelated products until the first connect succeeds.",
      },
      {
        title: "Whitelist UniPost's LinkedIn callback",
        body: "Add the UniPost LinkedIn callback URL exactly as shown below. LinkedIn is strict about redirect URI matching, so copy it verbatim instead of retyping.",
      },
      {
        title: "Save the Client ID and Client Secret in UniPost",
        body: "Paste the LinkedIn app credentials into the LinkedIn card under White-label Credentials. Once saved, new LinkedIn OAuth flows in that workspace will use your app branding.",
      },
      {
        title: "Connect one real LinkedIn account end to end",
        body: "Your first success criterion is simple: your app name appears on the LinkedIn consent screen and UniPost lands back in the workspace with a connected account row.",
      },
      {
        title: "Capture the exact app config that worked",
        body: "Before you move on, record the redirect URI, enabled products, and the LinkedIn app owner account so teammates can reproduce the setup without re-discovering it.",
      },
    ],
    fieldMap: [
      ["LinkedIn app field", "UniPost field", "Notes"],
      ["Client ID", "Client ID", "Paste exactly as issued in the LinkedIn app settings."],
      ["Client Secret", "Client Secret", "Treat as production secret material; store it once in UniPost and in your password manager."],
      ["Authorized redirect URL", "Callback URL below", "LinkedIn rejects near-matches, so do not improvise the path."],
    ],
    gotchas: [
      ["Redirect URI mismatch", "LinkedIn is unforgiving here. If the consent screen errors immediately, compare the callback URL character by character."],
      ["Over-scoping too early", "Adding extra LinkedIn products before the first working connect usually creates more review and troubleshooting surface than you need."],
      ["Wrong app owner context", "Use a company-controlled developer account so the app does not disappear behind a single employee login."],
    ],
    doneChecklist: [
      "LinkedIn consent shows your app branding.",
      "The callback returns to UniPost without a redirect mismatch error.",
      "A real LinkedIn account is connected in the workspace.",
      "Your team has documented the exact LinkedIn app that backs production.",
    ],
    relatedPlatformHref: "/docs/platforms/linkedin",
    relatedPlatformTitle: "LinkedIn platform guide",
  },
  tiktok: {
    slug: "tiktok",
    name: "TikTok",
    title: "TikTok White-label Setup",
    lead: "TikTok usually takes more operational prep than LinkedIn, so this guide focuses on the shortest reliable path: get one app, one callback, and one creator account connected before you think about scale.",
    portalName: "TikTok for Developers",
    portalUrl: "https://developers.tiktok.com",
    dashboardCard: "TikTok",
    clientIdLabel: "Client Key",
    clientSecretLabel: "Client Secret",
    callbacks: ["https://api.unipost.dev/v1/oauth/callback/tiktok"],
    bestFor: "Products that need customer TikTok connections and are ready for a more compliance-heavy setup than LinkedIn.",
    appReview: "Expect more scrutiny than LinkedIn. Build time for audit / review into the rollout plan, not after engineering is done.",
    beforeYouStart: [
      "A company-owned TikTok developer account.",
      "A creator account you can use for repeatable integration tests.",
      "A clear internal owner for TikTok review and policy follow-up.",
      "A rollout plan that starts with internal or pilot traffic before broad public launch.",
    ],
    steps: [
      {
        title: "Create the TikTok app with production ownership in mind",
        body: "Do not build your rollout around a personal side account. TikTok setup tends to involve review follow-up, so start from the company-owned developer identity you expect to keep long term.",
      },
      {
        title: "Add UniPost's TikTok callback before anything else",
        body: "TikTok callback mismatches are an easy source of false negatives. Whitelist the exact callback URL first so every later test is measuring the real integration, not a routing typo.",
      },
      {
        title: "Store the Client Key and Client Secret in UniPost",
        body: "TikTok labels the public identifier as Client Key. In UniPost, paste that into the TikTok credential card along with the Client Secret, then save the pair before you attempt any user-facing tests.",
      },
      {
        title: "Run one creator-account smoke test",
        body: "Do one complete connect with a controlled creator account and verify that the consent surface shows your app. Resist the urge to broaden rollout before this single path is stable.",
      },
      {
        title: "Prepare for review, not just for code-complete",
        body: "Once the technical path works, line up screenshots, product descriptions, and ownership details so review work does not stall the launch after engineering signs off.",
      },
    ],
    fieldMap: [
      ["TikTok app field", "UniPost field", "Notes"],
      ["Client Key", "Client Key", "TikTok uses Client Key where many platforms say Client ID."],
      ["Client Secret", "Client Secret", "Save once in UniPost and rotate through your internal secrets process when needed."],
      ["Redirect URI allowlist", "Callback URL below", "Add before testing, not after the first failure."],
    ],
    gotchas: [
      ["Creator-account assumptions", "Use a real creator account early so you discover any account-type issues while the setup surface is still small."],
      ["Review is part of the project", "Treat TikTok review as part of the rollout timeline, not a post-launch admin chore."],
      ["Naming mismatch", "TikTok says Client Key; UniPost mirrors that label in the dashboard so teammates do not paste the wrong field."],
    ],
    doneChecklist: [
      "TikTok shows your app during consent.",
      "The callback returns successfully to UniPost.",
      "A test creator account is connected in the workspace.",
      "Your team has named an owner for TikTok review and ongoing credential maintenance.",
    ],
    relatedPlatformHref: "/docs/platforms/tiktok",
    relatedPlatformTitle: "TikTok platform guide",
  },
  youtube: {
    slug: "youtube",
    name: "YouTube",
    title: "YouTube White-label Setup",
    lead: "YouTube setup is mostly a Google Cloud task. This guide strips it down to the pieces UniPost needs so you can get one branded OAuth flow working without wandering around Google Cloud menus.",
    portalName: "Google Cloud Console",
    portalUrl: "https://console.cloud.google.com",
    dashboardCard: "YouTube",
    clientIdLabel: "Client ID",
    clientSecretLabel: "Client Secret",
    callbacks: ["https://api.unipost.dev/v1/oauth/callback/youtube"],
    bestFor: "Products connecting customer-owned YouTube channels while keeping the Google consent experience under your brand.",
    appReview: "Scope verification may be needed depending on your rollout shape. Even before formal verification, start by proving the OAuth wiring with one real channel owner.",
    beforeYouStart: [
      "A Google Cloud project owned by the company, not by an individual engineer.",
      "The YouTube channel owner account you will use for the first smoke test.",
      "A consent-screen brand name and support email you are comfortable shipping.",
      "A plan for how your team will handle Google verification if the project grows beyond internal use.",
    ],
    screenshotSteps: [
      {
        title: "Step 1: pick or create a Google Cloud project",
        image: "/docs/white-label/youtube/step1.png",
      },
      {
        title: "Step 2: search `YouTube Data API v3` and enable it",
        image: "/docs/white-label/youtube/step2.png",
      },
      {
        title: "Step 3a: open the OAuth consent screen from APIs & Services",
        image: "/docs/white-label/youtube/step3-1.png",
      },
      {
        title: "Step 3b: finish the initial app setup in Google Auth Platform",
        image: "/docs/white-label/youtube/step3-2.png",
      },
      {
        title: "Step 4: go to Clients and click `Create client`",
        image: "/docs/white-label/youtube/step4.png",
      },
      {
        title: "Step 5: choose `Web application` and add UniPost's redirect URI",
        caption: "Authorized redirect URI: `https://api.unipost.dev/v1/oauth/callback/youtube`",
        image: "/docs/white-label/youtube/step5.png",
      },
      {
        title: "Step 6: copy the Google `Client ID` and `Client Secret`",
        image: "/docs/white-label/youtube/step6.png",
      },
      {
        title: "Step 7: paste them into UniPost's YouTube row and click `Save`",
        image: "/docs/white-label/youtube/step7.png",
      },
    ],
    consoleSteps: [
      {
        title: "Pick or create the Google Cloud project",
        body: "From the Google Cloud home page, click `Select a project` in the top bar. If you do not already have the project you want, click `New Project`, create it, and switch into that project before doing anything else.",
      },
      {
        title: "Enable YouTube Data API v3",
        body: "Open the top-left navigation menu, go to `APIs & Services` → `Library`, search for `YouTube Data API v3`, open that API, and click `Enable`. Do this before creating the OAuth client so the project is clearly configured for YouTube.",
      },
      {
        title: "Open Google Auth Platform and complete the initial app setup",
        body: "In the same project, go to `Google Auth Platform`. If Google shows a `Get started` flow, complete the minimum setup: app name, support email, and audience. Use the customer-facing brand name you want people to see during consent.",
      },
      {
        title: "Create the OAuth client",
        body: "Inside `Google Auth Platform`, open `Clients`, click `Create client`, and choose `Web application` as the application type. Name it something obvious such as `UniPost YouTube OAuth` so teammates can find it later.",
      },
      {
        title: "Add UniPost's redirect URI",
        body: "In the `Authorized redirect URIs` section, add the exact UniPost callback URL shown below: `https://api.unipost.dev/v1/oauth/callback/youtube`. Copy and paste it exactly; a redirect typo will break the flow even if the rest of the setup is correct.",
      },
      {
        title: "Create and copy the credentials immediately",
        body: "Click `Create`. Google will show the `Client ID` and `Client Secret` in the confirmation dialog. Copy both immediately and store them safely. Google may only show the full secret at creation time.",
      },
      {
        title: "Paste both values into UniPost",
        body: "Return to UniPost, open the White-label Credentials screen, find the YouTube row, paste the `Client ID` and `Client Secret`, and click `Save`. After that, start a fresh YouTube connection test with a real channel owner account.",
      },
    ],
    steps: [
      {
        title: "Create the OAuth client in the correct Google Cloud project",
        body: "Set up the OAuth client in the long-lived Google Cloud project you intend to keep in production. Moving later is possible, but it slows down verification and team operations.",
      },
      {
        title: "Add UniPost's YouTube callback exactly once, exactly right",
        body: "Copy the callback URI below directly into the Google OAuth client settings. This is the most common place to lose time because the rest of Google Cloud can look configured while the redirect URI is still wrong.",
      },
      {
        title: "Paste the Client ID and Client Secret into UniPost",
        body: "Open White-label Credentials in UniPost, find the YouTube row, and paste the Google OAuth credentials. Save first, then test the flow; do not troubleshoot OAuth against stale credentials.",
      },
      {
        title: "Test with an account that actually owns a YouTube channel",
        body: "A Google account without a YouTube channel is not a useful smoke test. Use a real channel owner so the first pass validates the whole path, not just the Google login step.",
      },
      {
        title: "Write down the working project and consent-screen settings",
        body: "Google Cloud setups drift easily across environments. After the first success, capture the project ID, OAuth client name, callback URI, and support contacts in your team docs.",
      },
    ],
    fieldMap: [
      ["Google Cloud field", "UniPost field", "Notes"],
      ["OAuth Client ID", "Client ID", "Paste from the Google OAuth client you intend to run in production."],
      ["OAuth Client Secret", "Client Secret", "Keep in your secrets system; UniPost stores it encrypted."],
      ["Authorized redirect URI", "Callback URL below", "Must match exactly or Google rejects the callback."],
    ],
    apiWorkflow: {
      title: "After the account connects: verify the API flow",
      intro: "Once your YouTube account is connected in the dashboard, the next job is to prove your API path is wired correctly too. The sequence below starts from the dashboard, then moves into the API calls you will keep using in your own backend.",
      steps: [
        {
          title: "Step 1: create your first API key in the dashboard",
          body: "Open UniPost in the same workspace where you saved the YouTube credentials, create an API key, and store it immediately. The first key must be created in the dashboard because there is no API key available yet to call `POST /v1/api-keys`.",
        },
        {
          title: "Step 2: list profiles and copy the `profile_id` you will use for branding",
          body: "Every workspace gets at least one profile. Start by listing profiles so you can grab the `id` for the profile that should own your hosted Connect branding.",
          snippets: [
            {
              lang: "curl",
              label: "cURL",
              code: `curl "https://api.unipost.dev/v1/profiles" \\
  -H "Authorization: Bearer <API_KEY>"`,
            },
          ],
        },
        {
          title: "Step 3: patch the profile branding that should show on hosted Connect",
          body: "With the `profile_id` in hand, update the logo, display name, and primary color. This is the fastest way to confirm your hosted Connect surface is reading your own branding instead of UniPost defaults.",
          snippets: [
            {
              lang: "curl",
              label: "cURL",
              code: `curl -X PATCH "https://api.unipost.dev/v1/profiles/<PROFILE_ID>" \\
  -H "Authorization: Bearer <API_KEY>" \\
  -H "Content-Type: application/json" \\
  -d '{
    "branding_logo_url": "https://yourcdn.com/logo.png",
    "branding_display_name": "Your Brand",
    "branding_primary_color": "#10b981"
  }'`,
            },
          ],
        },
        {
          title: "Step 4: confirm the connected YouTube account is visible through the API",
          body: "Now verify that the same workspace can see the YouTube connection over API. This is the handoff point between 'the dashboard connect worked' and 'my backend can rely on this account now'.",
          snippets: [
            {
              lang: "curl",
              label: "cURL",
              code: `curl "https://api.unipost.dev/v1/accounts?platform=youtube" \\
  -H "Authorization: Bearer <API_KEY>"`,
            },
          ],
        },
        {
          title: "Step 5: use that account and profile in your first real API workflow",
          body: "At this point you have the three values you need most often: `API_KEY`, `profile_id`, and the connected YouTube `account_id`. From here, the natural next move is a small end-to-end publish test so you know the connection is usable, not just visible.",
        },
      ],
    },
    gotchas: [
      ["No channel on the test account", "A Google login can succeed even when the account has no YouTube channel. Use a real channel owner for the first test."],
      ["Wrong Cloud project", "Creating the client in the wrong Google Cloud project is a common source of later operational pain. Pick the production owner early."],
      ["Verification lag", "Do not wait for full verification to begin technical testing, but do budget time for it before broad public rollout."],
    ],
    doneChecklist: [
      "Google consent shows your app branding.",
      "The callback returns to UniPost successfully.",
      "A channel-owning account is connected in the workspace.",
      "Your team has recorded the exact Google Cloud project and OAuth client that worked.",
    ],
    relatedPlatformHref: "/docs/platforms/youtube",
    relatedPlatformTitle: "YouTube platform guide",
  },
  twitter: {
    slug: "twitter",
    name: "X / Twitter",
    title: "X / Twitter White-label Setup",
    lead: "X setup is best approached as a deliberate paid-platform integration: get the app approved for your intended tier, wire the callback cleanly, and validate one branded OAuth flow before scaling out.",
    portalName: "X Developer Portal",
    portalUrl: "https://developer.x.com",
    dashboardCard: "X / Twitter",
    clientIdLabel: "Client ID",
    clientSecretLabel: "Client Secret",
    callbacks: ["https://api.unipost.dev/v1/oauth/callback/twitter"],
    bestFor: "Products that need customer X connections and are comfortable planning around X's tier and access constraints.",
    appReview: "Access and feature availability are tier-dependent. Treat billing / plan approval as part of setup, not an afterthought.",
    beforeYouStart: [
      "A company-owned X developer account with the intended paid access level.",
      "One test X account you control for the first branded connect.",
      "A rollout plan that accounts for X access limits and commercial constraints.",
      "An internal owner for credential rotation and developer-portal billing.",
    ],
    steps: [
      {
        title: "Confirm the X developer access level before wiring code",
        body: "If your developer access is not aligned with the rollout you want, no amount of callback tweaking will save the integration. Confirm the commercial and access prerequisites first.",
      },
      {
        title: "Register UniPost's X callback in the developer portal",
        body: "Add the callback URL below exactly as written. X OAuth setups are easy to derail with tiny environment mismatches, so treat the redirect URI as copy-paste material, not something to type from memory.",
      },
      {
        title: "Paste the Client ID and Client Secret into UniPost",
        body: "Use the X / Twitter credential row in UniPost's White-label Credentials screen. Save the credentials before testing, then retry from a clean browser session if you had earlier failures.",
      },
      {
        title: "Run one full connect on a controlled X account",
        body: "The first success goal is simple: the end user sees your X app branding during consent, and UniPost returns with a connected account rather than a plan or callback error.",
      },
      {
        title: "Document access tier and owner details immediately",
        body: "X integrations are operationally fragile if the billing owner, developer app owner, and engineering team are not aligned. Capture those details while the setup is fresh.",
      },
    ],
    fieldMap: [
      ["X app field", "UniPost field", "Notes"],
      ["Client ID", "Client ID", "Use the OAuth 2.0 client identifier from your X app."],
      ["Client Secret", "Client Secret", "Store once in UniPost, then manage future rotations like any production secret."],
      ["Callback / redirect URL", "Callback URL below", "Copy exactly to avoid redirect mismatch errors."],
    ],
    gotchas: [
      ["Tier mismatch", "X problems often look like OAuth bugs when the real issue is access level or commercial eligibility."],
      ["Old browser session state", "If you tested with the wrong credentials first, retry in a clean session after saving the correct app details."],
      ["Operational ownership", "If one person's personal developer account owns production, future billing and rotation work becomes much harder."],
    ],
    doneChecklist: [
      "The X consent screen shows your app branding.",
      "The redirect returns to UniPost successfully.",
      "A real X account is connected in the workspace.",
      "Your team has documented the owning developer account and access tier.",
    ],
    relatedPlatformHref: "/docs/platforms/twitter",
    relatedPlatformTitle: "X / Twitter platform guide",
  },
};

export const WHITE_LABEL_GUIDE_ORDER: WhiteLabelGuide["slug"][] = [
  "meta",
  "linkedin",
  "tiktok",
  "youtube",
  "twitter",
];

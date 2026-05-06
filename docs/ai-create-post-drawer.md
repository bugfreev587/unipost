# AI Create Post Drawer

## In-Development Feature Gate

UniPost now keeps dashboard-only in-development features in [`FEATURES_IN_DEV`](/Users/xiaoboyu/unipost/dashboard/src/lib/features-in-dev.ts:1). Any feature listed there must remain hidden from non-super-admin users. During development:

- Add the feature name to `FEATURES_IN_DEV`.
- Gate all dashboard entry points with `isFeatureInDevEnabledForMe(feature, isSuperAdmin)`.
- Leave the API-side source of truth for identity on `SUPER_ADMINS` via `/v1/me`.
- Remove the feature name from `FEATURES_IN_DEV` when the rollout is complete.

Current entries:

- `facebook_pages`
- `ai_assist_create_post_drawer`

## Drawer Layout

Desktop should use a progressive disclosure layout:

1. Default compose state: two columns.
2. AI-assisted state: expand into three columns.

Default:

```text
+---------------------------------------------------------------+
| Create post                                                   |
+--------------------------------------+------------------------+
| Compose                              | Platforms              |
| - Main caption                       | - Selected accounts    |
| - Media                              | - Per-platform status  |
| - Publish mode                       | - Quick override entry |
+--------------------------------------+------------------------+
```

AI active:

```text
+-----------------------------------------------------------------------------------+
| Create post                                                                       |
+--------------------------------+------------------------+--------------------------+
| Edit                           | Platforms              | AI Assist                |
| - Main caption                 | - Selected accounts    | - Generate from brief    |
| - Media                        | - Suggestion badges    | - Improve current draft  |
| - Publish mode                 | - Apply per platform   | - Adapt per platform     |
| - Validation summary           | - Override state       | - Write from media       |
+--------------------------------+------------------------+--------------------------+
```

Recommended width split on desktop:

- Edit: `42%`
- Platforms: `28%`
- AI: `30%`

Behavior:

- Keep AI closed by default.
- Clicking `AI Assist` expands the drawer and slides in the AI panel from the right.
- Closing AI returns to the normal two-column compose view without losing draft state.

## Column Responsibilities

### Edit Column

Owns the current left-side compose flow:

- `mainContent`
- media upload / preview
- publish mode
- validation summary

This stays the primary work surface. AI should not replace it.

### Platforms Column

This becomes the “destination + diff” column, not a second full editor. It should show:

- selected platform accounts
- whether each platform is using main copy or an override
- whether AI has a pending suggestion for that platform
- one-click actions: `Apply`, `Keep`, `Rewrite`

Each `PlatformEditorBlock` can still expand for manual editing, but the collapsed state should become more informative once AI is active.

### AI Column

This is a suggestion and action panel, not a generic chat window.

Sections:

1. `Quick actions`
2. `Prompt / brief`
3. `Generated suggestions`
4. `Apply controls`
5. `Why this suggestion`

Recommended quick actions:

- `Generate from brief`
- `Improve`
- `Shorten`
- `Make punchier`
- `Adapt per platform`
- `Write from media`
- `Fix validation issues`

## Interaction Model

### Global AI

Triggered from the drawer header or main caption area.

Use cases:

- draft from scratch
- rewrite the main caption
- generate suggestions for all selected platforms

### Platform AI

Triggered inside each platform card.

Use cases:

- rewrite this platform only
- shorten to fit platform limit
- match the tone of this platform

### Validation AI

Triggered from the validation panel.

Use cases:

- fix caption length
- rephrase weak CTA
- convert a main caption into platform-specific variants

Do not auto-fill compliance-sensitive fields like TikTok privacy or YouTube made-for-kids.

## Suggested State Shape

The AI layer should not write directly into publish payloads. It should generate suggestion objects first, then the user applies them.

```ts
type AIAssistMode =
  | "brief"
  | "improve"
  | "adapt"
  | "media"
  | "fix_validation";

type AISuggestion = {
  request_id: string;
  mode: AIAssistMode;
  summary?: string;
  main_caption?: string;
  platform_captions?: Array<{
    account_id: string;
    platform: string;
    caption: string;
    reason?: string;
  }>;
  first_comment_suggestions?: Array<{
    account_id: string;
    text: string;
  }>;
  hashtags?: string[];
  youtube_fields?: Array<{
    account_id: string;
    title?: string;
    tags?: string[];
  }>;
  warnings?: string[];
};
```

Front-end state should track:

```ts
type AIAssistState = {
  open: boolean;
  loading: boolean;
  mode: AIAssistMode | null;
  brief: string;
  objective: "awareness" | "engagement" | "clicks" | "sales";
  tone: "professional" | "friendly" | "bold" | "playful";
  suggestions: AISuggestion | null;
  selectedSuggestionAccountIds: string[];
  error: string | null;
};
```

## API Draft

### 1. Generate AI Suggestions

`POST /v1/ai/post-assist`

Purpose:

- generate new copy from a brief
- improve an existing draft
- adapt one draft into platform-specific variants
- generate copy from uploaded media context
- propose fixes for text-related validation problems

Request:

```json
{
  "mode": "adapt",
  "profile_id": "prof_123",
  "main_caption": "Launching our new hydration bottle this Friday.",
  "selected_account_ids": ["sa_twitter_1", "sa_linkedin_1", "sa_instagram_1"],
  "platform_posts": [
    {
      "account_id": "sa_linkedin_1",
      "caption": ""
    }
  ],
  "media_ids": ["med_123"],
  "objective": "sales",
  "tone": "bold",
  "include_cta": true,
  "brief": "Focus on leak-proof design, 24h cold retention, and launch discount.",
  "validation_issues": [
    {
      "account_id": "sa_twitter_1",
      "field": "caption",
      "code": "exceeds_max_length",
      "message": "Caption exceeds the maximum length for this platform."
    }
  ]
}
```

Response:

```json
{
  "data": {
    "request_id": "aireq_123",
    "mode": "adapt",
    "summary": "Created platform-specific variants optimized for short-form, professional, and visual-first channels.",
    "main_caption": "Meet the bottle that keeps up with your day.",
    "platform_captions": [
      {
        "account_id": "sa_twitter_1",
        "platform": "twitter",
        "caption": "Launching Friday: a leak-proof bottle that stays cold for 24 hours. Early discount drops at launch.",
        "reason": "Shortened for X and moved the hook to the first sentence."
      }
    ],
    "hashtags": ["#productlaunch", "#hydration", "#ecommerce"],
    "warnings": []
  }
}
```

### 2. Optional Preview Endpoint

`POST /v1/ai/post-assist/preview`

Purpose:

- return a diff-oriented response without storing anything
- useful if later you want streaming or side-by-side compare

This can be deferred if `POST /v1/ai/post-assist` already returns enough structured suggestion data.

### 3. No Auto-Publish Endpoint

Avoid combining AI generation and publish in one endpoint.

Recommended flow:

1. User drafts content.
2. User requests AI suggestions.
3. User applies some or all suggestions into local form state.
4. Existing `validateSocialPost` and `createSocialPost` flow remains unchanged.

## Implementation Notes

- Reuse `/v1/platforms/capabilities` semantics so AI suggestions respect platform limits.
- Reuse the current `CreateSocialPostPayload` shape when handing context to the AI service.
- Keep AI write paths text-only at first; do not mutate sensitive platform options automatically.
- When possible, generate per-account captions keyed by `account_id` so the output maps directly to `overrides[accountId].caption`.

## Recommended MVP Scope

First release:

- `Generate from brief`
- `Improve current draft`
- `Adapt for selected platforms`
- `Fix caption-related validation issues`

Later:

- `Write from media`
- analytics-informed suggestions
- first-comment suggestions
- richer YouTube/TikTok field suggestions

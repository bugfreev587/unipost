# Facebook Pages — App Review Evidence

**App**: unipost-dev
**Permissions requested**: 7 `pages_*` scopes
**Status**: Ready for App Review submission
**Last updated**: 2026-04-21

This document maps each requested permission to the concrete code path that exercises it. Meta App Review requires evidence that every permission is actually used by the product — not just requested. Screencast script at the bottom covers the end-to-end flow a reviewer will replay.

---

## Per-permission evidence

Every entry includes the permission name, where the user-facing feature lives in the product, the Graph API endpoint we call, and the Go file + function that issues the call.

### 1. `pages_show_list`
**Feature**: Page Picker shown after a user finishes Facebook OAuth in UniPost. Lists every Page the authorizing user admins so they can pick which ones to connect.
**Endpoint**: `GET /v22.0/me/accounts?fields=id,name,access_token,category,picture,tasks`
**Code**: `api/internal/platform/facebook.go` — `FacebookAdapter.FetchPages`
**User flow**: Connections → "Connect Facebook" → OAuth consent → modal lists Pages → select → "Connect selected".

### 2. `pages_manage_posts`
**Feature**: Compose + publish a post from UniPost to a connected Page (text / link / photo / video).
**Endpoints**:
- `POST /v22.0/{page_id}/feed` (text / link)
- `POST /v22.0/{page_id}/photos` (single photo)
- `POST /v22.0/{page_id}/videos` (single video)
**Code**: `api/internal/platform/facebook.go` — `FacebookAdapter.postFeed / postPhoto / postVideo`.
**User flow**: Create post → select FB Page → compose → Publish now. Post appears on Page.

### 3. `pages_read_engagement`
**Feature**: Per-post analytics (impressions, reach, clicks, engaged users) shown in UniPost's Analytics dashboard.
**Endpoint**: `GET /v22.0/{post_id}?fields=reactions.summary(true),comments.summary(true),shares,insights.metric(post_impressions,post_impressions_unique,post_clicks,post_engaged_users,post_video_views)`
**Code**: `api/internal/platform/facebook.go` — `FacebookAdapter.GetAnalytics`
**User flow**: Analytics → by-post drill-in renders these numbers.

### 4. `pages_read_user_content`
**Feature**: Inbox — Comments tab surfaces every comment left on posts the user published via UniPost so they can read + reply.
**Endpoint**: `GET /v22.0/{post_id}/comments?fields=id,message,from{id,name,picture},created_time&limit=25`
**Code**: `api/internal/platform/facebook.go` — `FacebookAdapter.FetchComments`. Sync loop in `api/internal/worker/inbox_sync.go` (case "facebook"). Scoped to posts published via UniPost per our design decision to avoid scanning the whole Page timeline.
**User flow**: Inbox → Comments tab → row per comment grouped by post.

### 5. `pages_manage_engagement`
**Feature**: Reply to a comment from inside UniPost Inbox.
**Endpoint**: `POST /v22.0/{comment_id}/comments?message=...`
**Code**: `api/internal/platform/facebook.go` — `FacebookAdapter.ReplyToComment`. Dispatched by `api/internal/handler/inbox.go` in the `case "fb_comment"` branch of the reply handler.
**User flow**: Inbox → Comments → Reply → message appears on Facebook as a reply from the Page.

### 6. `pages_messaging`
**Feature**: Inbox — DMs tab surfaces Messenger conversations and lets the user reply.
**Endpoints**:
- Read: `GET /v22.0/{page_id}/conversations?platform=messenger&fields=id,participants{id,name,picture{url}},messages.limit(25){id,message,from,created_time}`
- Send: `POST /v22.0/{page_id}/messages` with JSON body `{recipient:{id:PSID}, messaging_type:"RESPONSE", message:{text:...}}`
**Code**: `api/internal/platform/facebook.go` — `FacebookAdapter.FetchConversations / SendDM`. 24-hour reply window enforced client-side with an amber banner above the input when closed.
**User flow**: Inbox → DMs tab → select conversation → type → send. Outside the 24h window, the Send button is disabled.

### 7. `pages_manage_metadata`
**Feature**: Page Picker displays Page category + profile picture + admin `tasks` so the user can tell Pages apart, and admin-permission rows grey out when the authorizing user's role on a given Page lacks publishing permission.
**Endpoint**: same `/me/accounts` fields call as `pages_show_list`, plus webhook subscription management once webhooks are enabled: `POST /v22.0/{page_id}/subscribed_apps` to subscribe the app to the Page's feed + messages fields.
**Code**: `api/internal/platform/facebook.go` — `FacebookAdapter.FetchPages` reads `tasks`, `PageHasPublishTask` decides can-publish; webhook subscription lives in `FacebookAdapter.SubscribePageToWebhooks` (POST `/{page_id}/subscribed_apps` for `feed,messages,messaging_postbacks`), called from `handler.OAuthHandler.PendingConnectionFinalize` right after each Page's `social_accounts` row is written. Inbound events land in `handler/meta_webhook.go` at `handleFacebookEntry`.
**User flow**: Page Picker shows "no publish permission" hint when a returned Page's `tasks` doesn't include `CREATE_CONTENT`.

---

## Screencast script (≤ 3 minutes)

Record once the reviewer-facing flow stabilizes. Keep it short — Meta reviewers scan, they don't watch start-to-finish.

### 1. Connect (0:00 – 0:30)
- Open UniPost dashboard → Connections tab.
- Click **Connect Facebook**.
- OAuth consent screen appears — voice-over: "We request 7 Page permissions, each gated by SUPER_ADMINS during development."
- Approve.
- Page Picker modal opens showing the test Page (`Catherine's bakery store`).
- Select → Connect selected.
- Account appears in the list.

### 2. Publish a text + photo (0:30 – 1:10)
- Create post → select the FB Page.
- Compose: caption + attach one image.
- Publish now.
- Switch to facebook.com/{page} in a second tab — show the post live on the Page.
- Back to UniPost posts list → status `published`, View on Facebook link resolves.

### 3. Manage comments (1:10 – 1:50)
- From the other FB account, leave a comment on that post.
- Switch back to UniPost → Inbox → Comments tab.
- Within 5 minutes (or immediately after the webhook hook fires) the comment is listed.
- Click Reply → type → send.
- Back to Facebook → reply is visible on the comment thread.

### 4. Messenger DM (1:50 – 2:30)
- From the other FB account, send a Messenger message to the Page.
- Inbox → DMs tab → conversation appears.
- Send a reply from UniPost.
- Back to Facebook Messenger → reply delivered.

### 5. Analytics (2:30 – 3:00)
- Analytics tab → per-post row shows impressions + reactions + engagements from the post published earlier.
- Optional: `/facebook/page-insights` endpoint can be hit via the Admin panel; mention the 100-like threshold and show the "Keep growing!" state if the Page is below that.

---

## Submission checklist

- [x] Each of the 7 permissions documented above has a concrete API call in the product code.
- [x] All endpoints use the `v22.0` API version consistently.
- [x] Feature-flagged behind `SUPER_ADMINS` — no regular user can hit FB flows until App Review approves.
- [x] 24-hour window for Messenger surfaced in the UI before the user hits Send.
- [x] Page Tokens are stored encrypted (AES-256-GCM). User Token stored in `meta_user_tokens` for "add another Page" later.
- [x] Webhook verify + receive endpoints signed with the App Secret (HMAC SHA-256) per Meta's spec.
- [x] Page-specific webhook subscription (`POST /{page_id}/subscribed_apps`) invoked after each Page is connected; inbox absorbs `feed` comments and `messages` DMs in near real-time.
- [ ] Screencast recorded and uploaded — **pending**.
- [ ] App Review submission — **pending screencast**.

---

## Notes for the reviewer

- All 7 permissions are exercised in v1 — no "requested but unused" scopes.
- Traffic is server-to-Graph; we do not surface Page Tokens to the browser.
- The 24-hour Messenger window is enforced in both directions: client-side (disables the Send button) and server-side (Meta itself rejects, we show a clean error).
- Page Insights below the 100-like threshold returns a `below_100_likes_notice=true` flag rather than a hard error so the dashboard can show a "Keep growing!" empty state.

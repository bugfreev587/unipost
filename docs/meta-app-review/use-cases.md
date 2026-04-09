# Meta App Review — Per-Scope Use Case Justifications

Copy-paste these into the Meta dev portal's "Request access" forms.
Each section is the exact text reviewers want: what UniPost does,
how the scope is used, who benefits, and what data flows where.

---

## Instagram Basic (`instagram_basic`)

UniPost is a unified social media publishing API that lets developers
integrate posting capabilities into their products without dealing
with each platform individually. Our customers — typically SaaS
companies — use UniPost to let their end users connect their own
social media accounts (including Instagram Business / Creator
accounts) and publish content the end user composed.

We need `instagram_basic` exclusively to:

1. Read the connected Instagram account's username and profile
   picture so we can display it in our customer's dashboard
   ("Connected as @example_business").
2. Verify the account is an Instagram Business or Creator account
   (which is a prerequisite for using the Content Publishing API).

We do NOT use `instagram_basic` to read media, comments, insights,
followers, or any user-generated content. Account display info is
the only data we touch and it's stored as a string in our
`social_accounts` database row.

**Data flow:** Instagram → UniPost API → Customer dashboard. No
third party. No advertising. Account data is encrypted at rest
and deleted on disconnect via our standard `DELETE /v1/social-accounts/{id}`
endpoint OR via Meta's data deletion callback at
`POST /v1/meta/data-deletion`.

---

## Instagram Content Publishing (`instagram_content_publish`)

UniPost's customers use the Content Publishing API to publish
content their end users have composed in the customer's own
product. The end user authorizes UniPost to publish on their
behalf via Meta's standard OAuth flow; the customer's product
calls UniPost's `POST /v1/social-posts` endpoint with the
caption + media URLs; UniPost in turn calls
`POST /{ig-user-id}/media` and `POST /{ig-user-id}/media_publish`
on the user's behalf.

The end user is in full control. They:
- Explicitly authorize UniPost via Instagram's OAuth consent
  screen at the moment of connection.
- See every post UniPost publishes on their feed (it's their feed).
- Can revoke access at any time via Instagram's connected apps
  page or the customer's dashboard.
- Can request data deletion via Meta's data deletion callback,
  which we honor at `POST /v1/meta/data-deletion`.

We do NOT use this scope to:
- Publish content the end user did not author or approve
- Run automated campaigns without explicit user opt-in
- Cross-post content from other platforms without consent

Typical use case: a marketing operations SaaS lets its end users
draft a post once and publish it to Instagram + LinkedIn + Twitter
in one click. The end user composes the post in the SaaS's UI,
clicks "Publish," and the SaaS calls UniPost's API to fan it out.

**Demo video:** see `videos.md` § Instagram Connect Flow for the
60-second screen recording showing end-to-end OAuth + publish.

---

## Threads Basic (`threads_basic`)

Identical use case to `instagram_basic`. UniPost reads only the
connected Threads account's display name and profile picture so we
can show "Connected as @example" in our customer's dashboard. We
do not read posts, replies, mentions, or any user-generated content.

The same data flow, encryption-at-rest, and deletion callbacks
that apply to Instagram apply to Threads.

---

## Threads Content Publishing (`threads_content_publish`)

Identical use case to `instagram_content_publish`. End users
authorize UniPost via Threads' OAuth flow; the customer's product
calls `POST /v1/social-posts` with caption + optional media; UniPost
calls Threads' Graph API on the user's behalf using the user's
access token.

End user controls:
- Explicit OAuth consent at connection time
- Every post visible on their Threads profile
- Revocation via Threads' connected apps page OR Meta's data
  deletion callback

We do NOT use this scope for inbox, DMs, mentions handling, or
any reply-to-incoming flows. UniPost is publish-only.

---

## Out of scope (NOT requested)

UniPost intentionally does not request the following scopes
even though they're in the same product family. Listing them here
as a transparency signal to the reviewer:

- `instagram_manage_messages` — UniPost is a publishing API, not a
  customer support tool. Inbox handling is a separate product
  surface that we don't intend to build.
- `instagram_manage_insights` — Analytics access would change the
  data flow significantly (we'd be storing engagement metrics for
  the end user). Out of scope for v1.
- `instagram_manage_comments` — Comment management is the same
  data-flow concern as insights. Out of scope.
- `instagram_branded_content_brand` / `_creator` — UniPost has no
  branded-content product surface.
- `pages_manage_posts` etc. — UniPost only supports user-context
  publishing (the end user authorizes their own account), not
  Pages-context posting via business managers.

We may request some of these in the future as we build out
analytics + inbox products, but they will be requested in
separate App Review submissions with their own justifications.

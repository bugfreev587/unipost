# Meta App Review — Submission Package

This directory contains everything needed to submit UniPost to Meta's
App Review for the **Instagram Content Publishing** and **Threads
Content Publishing** product tracks.

Sprint 4 PR7 ships the technical pieces (data-deletion endpoint,
documentation drafts, demo video scripts) so the submission can go
out the moment Meta business verification clears.

## Submission checklist

Each item in order. Items marked **[manual]** are Xiaobo-only — they
require logging into the Meta dev portal and clicking through forms.

### Pre-submission

- [ ] **[manual]** Meta business verification complete (3-7 day wait)
- [ ] **[manual]** Verified business entity attached to the dev portal
- [ ] **[manual]** Privacy policy URL published at `https://unipost.dev/privacy` and matches `privacy-policy.md` in this directory
- [ ] **[manual]** Terms of service URL published at `https://unipost.dev/terms`
- [ ] **[manual]** Data deletion URL published at `https://api.unipost.dev/v1/meta/data-deletion` (already live as of Sprint 4 PR7)
- [ ] **[manual]** App Icon (1024×1024 PNG) uploaded to dev portal
- [ ] **[manual]** Three demo videos recorded per `videos.md` and uploaded

### Code prerequisites (already done in Sprint 4 PR7)

- [x] `POST /v1/meta/data-deletion` endpoint live in production
- [x] Endpoint verifies signed_request HMAC-SHA256 signatures
- [x] Returns Meta-compliant `{url, confirmation_code}` response
- [x] Returns 503 NOT_CONFIGURED until `META_APP_SECRET` is set
- [x] Endpoint unit-tested against synthetic signed_requests

### Submission steps (after pre-submission complete)

1. **[manual]** Log into https://developers.facebook.com/apps/{APP_ID}/app-review
2. **[manual]** Click "Request" next to **instagram_basic**
3. **[manual]** Paste `use-cases.md` § Instagram Basic into the justification field
4. **[manual]** Attach the Twitter / Bluesky demo video as a "platform usage example"
5. **[manual]** Click "Request" next to **instagram_content_publish**
6. **[manual]** Paste `use-cases.md` § Instagram Content Publishing
7. **[manual]** Attach the Connect flow demo video
8. **[manual]** Repeat steps 2–7 for **threads_basic** and **threads_content_publish**
9. **[manual]** **Do NOT** request inbox / DM / management scopes — those are explicitly out of scope per Sprint 4 D8.
10. **[manual]** Click **Submit for review**

### Post-submission

- [ ] Wait 5-15 business days for Meta's first response
- [ ] If approved → set `META_APP_SECRET` in Railway, set `INSTAGRAM_CLIENT_ID` / `INSTAGRAM_CLIENT_SECRET`, set `THREADS_CLIENT_ID` / `THREADS_CLIENT_SECRET`, then ship the Sprint 5/6 Meta connector code
- [ ] If rejected → Meta sends a feedback email with specific issues. Iterate and re-submit. First-attempt rejection rate is ~50%; budget for 2-3 cycles.

## What's in this directory

| File | Purpose |
|---|---|
| `README.md` | This file — submission checklist and overview |
| `use-cases.md` | Per-scope justification text to paste into the dev portal |
| `privacy-policy.md` | Required updates to the existing privacy policy |
| `videos.md` | Scripts for the three demo videos |

## Why Sprint 4 ships this without submitting

Per Sprint 4 founder decision, Meta business verification was deferred
("skip for now") so the launch on April 28 isn't gated on the 3-7 day
verification wait + the 5-15 day review wait. The endpoint code lives
in production from day 1; the actual submission button gets clicked
whenever you're ready to add Instagram + Threads to UniPost (probably
Sprint 5 or 6).

This way: zero engineering work blocks on Meta's process, but the
moment business verification clears the submission can go out the
same day with no scrambling for missing documentation.

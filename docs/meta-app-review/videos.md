# Meta App Review — Demo Video Scripts

Meta requires a 30-60 second screen recording demonstrating each
permission in use. Three videos to record before submission. Each
should be exported as MP4 (H.264, 30fps, 1080p) and uploaded
directly to the dev portal — Meta does not accept YouTube links
for review submissions.

## Recording setup

- **Tool**: QuickTime Player (Mac), OBS, or Loom (export to MP4)
- **Browser**: Use a clean browser profile with no extensions visible
- **Audio**: Optional. If included, narrate concisely; if not, no audio is fine
- **Resolution**: 1920×1080 or higher
- **Duration**: 30-60 seconds per video; Meta auto-rejects anything over 90s
- **Format**: MP4 (H.264). Output to `docs/meta-app-review/videos/` —
  use git-lfs or .gitignore the binaries and link from this file

The videos can use the existing Twitter / LinkedIn / Bluesky Connect
flows (which work in production today) as stand-ins for the eventual
Instagram / Threads flows. Reviewers want to see THE PATTERN, not
the specific platform — they understand the Meta integration will
look identical once approved.

---

## Video 1 — End-User Connect Flow (60s)

**Purpose:** Show how an end user authorizes UniPost to publish
on their behalf via standard OAuth.

**Script:**

| Time | Action | What's on screen |
|---|---|---|
| 0:00 | Open the customer dashboard, navigate to "Connected Accounts" | Empty connected accounts list |
| 0:05 | Click "Connect Instagram" button | Hosted Connect page appears with "Connect Instagram to Acme Corp" header + Authorize button |
| 0:10 | Click "Authorize Instagram" | Browser navigates to instagram.com OAuth consent screen |
| 0:15 | Sign in to Instagram with test account | Instagram login screen → consent screen showing requested permissions |
| 0:25 | Highlight the consent screen — show the user what they're approving | Pause for 2-3 seconds on the consent screen with the scopes visible |
| 0:30 | Click "Authorize" on instagram.com | Bounce through OAuth callback → land on customer's "connected!" success page |
| 0:35 | Customer dashboard now shows "Connected as @test_account" | Connected accounts list updated, showing the new account |
| 0:45 | (Optional) Show the encrypted token in the database — NEVER readable | Brief flash of database admin showing encrypted access_token blob |
| 0:55 | End on the connected accounts screen | Final state |

**Voiceover** (if audio is included): "When an end user wants to
publish to Instagram through Acme Corp, they're redirected to a
hosted Connect page powered by UniPost. They click Authorize,
sign in to Instagram, see exactly what permissions are being
requested, and consent. The token is then encrypted at rest in
UniPost's database — never visible in plaintext to anyone."

---

## Video 2 — Publishing on the User's Behalf (45s)

**Purpose:** Show that publishing happens via deliberate, user-
initiated API calls — not automated, not background.

**Script:**

| Time | Action | What's on screen |
|---|---|---|
| 0:00 | Open the customer's compose interface | Empty draft form |
| 0:05 | Type a caption and attach an image | Caption + image preview |
| 0:15 | Click "Publish to Instagram" | Confirmation modal appears asking the user to confirm |
| 0:20 | Click confirm | Spinner / loading state |
| 0:25 | Switch to instagram.com in another tab | The freshly-published post visible on the user's feed |
| 0:35 | Switch back to the customer dashboard | Post appears in "Published" history with green checkmark |
| 0:40 | Show the API request log line for transparency | Brief flash of the curl command that produced the post |

**Voiceover:** "The end user composes their post in Acme's UI,
clicks Publish, and Acme's product calls UniPost's publish API.
UniPost decrypts the OAuth token in memory, calls Instagram's
publishing API on the user's behalf, and the post lands on the
user's Instagram feed exactly as if they had posted from
Instagram's own app. The user is in control: every post originates
from a deliberate click, never from automation."

---

## Video 3 — Disconnect / Data Deletion (30s)

**Purpose:** Show that the end user can revoke access and that
deletion is honored immediately.

**Script:**

| Time | Action | What's on screen |
|---|---|---|
| 0:00 | Open the customer's "Connected Accounts" list with the test account showing | Connected accounts list with test_user@example.com Instagram |
| 0:05 | Click the Disconnect button next to the Instagram row | Confirmation modal: "This will permanently delete your encrypted Instagram access token. Continue?" |
| 0:10 | Click confirm | Account disappears from the list |
| 0:15 | Open the database admin in another tab, query the social_accounts table | Show the row is now disconnected_at NOT NULL, access_token field is wiped |
| 0:20 | (Alternative path) Navigate to instagram.com/accounts/manage_access | Show UniPost no longer in the connected apps list |
| 0:25 | Brief overlay text: "Deletion is permanent and immediate" | Final frame |

**Voiceover:** "Disconnection is immediate and permanent.
The encrypted token is wiped from UniPost's database within
milliseconds, and the same operation happens automatically when
Meta's data deletion callback fires — for example, when the user
revokes access via Instagram's own connected apps page. UniPost
respects user control end-to-end."

---

## After recording

1. Trim each clip to under 60 seconds in QuickTime / iMovie
2. Export as MP4 to this directory
3. Add to .gitignore (don't commit binaries) OR set up git-lfs
4. Update this file with the local paths (e.g. `videos/01-connect.mp4`)
5. When submitting to Meta, upload directly via the dev portal's
   "Add demo video" button on each scope's request form

---

## Reviewer red flags to avoid

Per Meta's documentation and various developer postmortems, these
are the things that cause the most frequent rejections:

1. **Showing automated posting without user consent.** Always show
   a click-to-publish action by the end user, never a scheduled job
   or webhook-triggered post.
2. **Privacy policy URL doesn't match the disclosed scopes.** Make
   sure `privacy-policy.md` updates have been published before
   recording begins.
3. **Test account with zero followers / activity.** Reviewers
   sometimes flag this as "not a real product." Use a test account
   with some baseline activity (a few existing posts, a profile
   picture, a non-default bio).
4. **Voiceover that contradicts the visual.** If the voiceover says
   "the user clicks publish" but the video shows an automated post,
   reviewers reject. Pick one — silent demo is safer.
5. **Showing scopes you're NOT requesting.** If the OAuth consent
   screen shows `instagram_manage_messages` because your test app
   has all permissions enabled, reviewers will get confused. Disable
   any unused permissions in the dev portal before recording.

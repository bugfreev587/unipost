# Calendar Edit Post Design

## Goal

Replace the Calendar event popover's `Open in List` handoff with an in-place `Edit` flow that lets users fully edit draft and scheduled posts from the calendar.

## Scope

- The Calendar popover keeps its compact details view for quick inspection.
- The primary action changes from `Open in List` to `Edit`.
- `Edit` opens an Apple Calendar-style anchored inspector near the selected event instead of navigating to List View or opening a right-side drawer.
- The edit inspector reuses the Create Post composer behavior so users can change the same information they supplied when creating a post:
  - selected social accounts and platforms
  - main caption
  - per-platform captions and fields
  - first comments, replies, and thread positions
  - media attachments
  - scheduled publish time
- Full editing is available only while a post is still `draft` or `scheduled`.
- Posts in publishing, published, failed, cancelled, or archived states remain inspectable but do not expose full editing in this flow.
- No new feature flag is added. This behavior lives under the existing Calendar View feature surface.

## Architecture

The implementation should split the existing create drawer into reusable composer internals with explicit modes:

- `create` mode preserves the current right-side Create Post Drawer.
- `edit` mode renders the same editing surface inside an anchored calendar inspector.

The dashboard client should call a shared save path that uses `POST /v1/posts` for creation and `PATCH /v1/posts/{id}` for draft or scheduled edits. The patch payload should use the same shape as create payloads so UI and validation behavior stay consistent.

## Backend Behavior

`PATCH /v1/posts/{id}` currently supports full content updates only for drafts and allows scheduled posts to edit only `scheduled_at`. This must change:

- For `draft` and `scheduled` rows, accept the same parsed publish payload used by draft editing today.
- Update `caption`, `media_urls`, `metadata`, `scheduled_at`, and `profile_ids` from the submitted platform posts.
- Keep optimistic state protection in SQL so rows that have moved to publishing or a terminal state cannot be overwritten.
- Re-run publish validation after saving and return validation data in the response.
- Preserve lifecycle patches such as cancel/archive/restore.
- Continue to reject edits for non-editable states with a conflict response.

## Calendar Interaction

The compact popover remains anchored to the selected event and closes on outside click. Its primary button becomes `Edit`.

When `Edit` is clicked:

- The selected event stays visually connected to the inspector.
- The compact detail content expands into a larger editor panel positioned by the same anchoring model.
- The panel should fit within the viewport, support scrolling, and preserve the calendar behind it.
- Closing the panel returns the user to the calendar without navigation.
- Saving refreshes calendar data and closes or returns to the detail view after success.

## Composer Requirements

The edit composer must initialize from the existing post response:

- derive selected accounts from stored `platform_posts` metadata when available
- restore main caption and per-account captions
- restore media IDs or media URLs sufficiently for display and payload rebuild
- restore platform options such as YouTube, TikTok, Instagram, LinkedIn, Facebook, and Pinterest settings
- restore scheduled time into the local datetime input

If an older post lacks metadata required for full restoration, the UI should show the best available data and block saving only when the user would otherwise submit an invalid payload.

## Testing

Backend tests should cover scheduled-post full edits:

- scheduled post accepts content/media/platform metadata edits
- scheduled post rejects edits after it is no longer scheduled or draft
- scheduled post keeps lifecycle patch behavior intact

Frontend validation should include:

- Calendar popover no longer renders `Open in List`
- editable posts render an `Edit` action
- clicking `Edit` opens the anchored editor rather than navigating
- saving invokes the edit path and refreshes calendar data

Manual verification should include month view and a narrow viewport to confirm the anchored inspector stays within the visible calendar surface.

# Publish GIF Status Tags Design

## Goal

Make the `UniPost status` enum values in the Publish GIFs support matrix easier to scan without changing any documentation claims or other pages.

## Scope

- Change only `/docs/guides/publish-gifs`.
- Render `Supported` and `Coming soon` as compact semantic labels in the existing table.
- Preserve the current status text exactly for accessibility, search, and documentation accuracy.
- Do not introduce a reusable cross-document component yet. If the development result is accepted, the pattern can be generalized in a separate task.

## Visual design

Use the selected compact Slack Docs-style label:

- 24px high inline label.
- 6px corner radius rather than a fully rounded pill.
- 9px horizontal padding.
- No border, icon, or decorative status dot.
- `Supported`: muted green background with readable green text.
- `Coming soon`: muted amber background with readable amber text.
- Light and dark themes each receive explicit semantic foreground and background colors.

The labels remain text-first and do not rely on color alone.

## Implementation

Add a page-local `PublishGifStatusTag` component with a strict `"Supported" | "Coming soon"` union. Use it only in the `UniPost status` cells. Add narrowly named styles to the existing docs shell stylesheet so light and dark modes remain consistent with the docs theme.

## Acceptance criteria

1. The Publish GIFs support matrix shows every `Supported` and `Coming soon` value as the compact label.
2. Green communicates supported and amber communicates coming soon.
3. Labels have a subtle rounded rectangle shape, no border, and no dot.
4. The page content, platform matrix, and recommended actions are otherwise unchanged.
5. The page builds and passes the existing source contract test.
6. The deployed development page is visually verified in both light and dark themes at desktop and mobile widths.

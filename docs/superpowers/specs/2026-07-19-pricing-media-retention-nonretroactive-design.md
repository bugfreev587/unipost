# Pricing Media Retention Non-Retroactivity Design

Date: 2026-07-19
Owner area: Marketing Pricing
Status: Approved for implementation

## Goal

Explain on the public Pricing page that media retention is calculated from the
workspace Plan in effect when the retention period begins, and that later Plan
changes do not retroactively change an existing retention period.

The explanation must be available without competing with plan selection,
comparison, or FAQ content.

## User-Facing Copy

> Media retention is based on the workspace plan in effect when the retention
> period begins. Later plan upgrades or downgrades do not retroactively extend
> or shorten an existing retention period.

## Placement and Presentation

- Render the note immediately after the Pricing FAQ grid and before the global
  marketing footer.
- Keep it inside the existing `.pr-page` content boundary.
- Present it as a single muted paragraph with a subtle top border.
- Use a 12px font, comfortable line height, and no icon, card, heading, CTA, or
  animation.
- Keep the text left-aligned on desktop and mobile.

This placement makes the policy discoverable at the bottom of the page while
preserving the visual priority of plans, comparison details, and FAQs.

## Implementation

- Add one static paragraph to
  `dashboard/src/app/pricing/pricing-page-client.tsx` after `.pr-faq-grid`.
- Add a narrowly scoped `.pr-retention-policy-note` rule to the Pricing page's
  existing CSS string.
- Do not change the shared site footer or other Pricing content.

## Verification

- Add a source-contract test that fails before implementation and proves:
  - the non-retroactivity copy is present;
  - the note renders after the FAQ grid;
  - the dedicated muted style remains low emphasis.
- Run the focused source test and the Dashboard production build.
- Verify `/pricing` at desktop and mobile widths with no console errors.

## Non-Goals

- No change to retention durations or backend cleanup behavior.
- No retroactive deadline migration.
- No change to `/docs/pricing`.
- No feature flag.

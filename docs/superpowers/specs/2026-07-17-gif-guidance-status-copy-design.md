# GIF Guidance Status Copy Design

## Goal

Remove ambiguity about the current GIF-to-MP4 capability without expanding the guide beyond its approved X and Facebook focus.

## User-facing truth

- Direct GIF publishing through UniPost is available for X and Facebook Pages.
- LinkedIn and Threads direct GIF integrations are still coming soon.
- GIF-to-MP4 conversion is already available through the UniPost API.
- Destination-specific publishing guidance for Instagram, TikTok, Pinterest, YouTube, and Bluesky is still coming soon.
- Conversion and publishing remain separate operations.

## Copy changes

For Instagram, TikTok, Pinterest, YouTube, and Bluesky, replace:

> MP4 conversion supported; GIF guidance coming soon

with:

> GIF-to-MP4 conversion available; destination-specific publishing guidance coming soon

Also remove two related ambiguities elsewhere on the page:

- The lead must describe conversion as available rather than an upcoming workflow.
- The conversion section must call the unpublished material “destination-specific publishing guides,” not generic “GIF guidance,” because the current page already documents the conversion workflow.

## Scope

- No component, layout, styling, navigation, API, or behavior changes.
- No new destination-specific workflow instructions.
- Preserve the existing recommended action: convert the GIF, then use the destination’s video workflow.

## Acceptance criteria

- The old ambiguous phrase no longer appears.
- The new status appears for all five video-oriented destinations.
- The lead no longer implies that conversion itself is upcoming.
- The final conversion note distinguishes available API conversion from upcoming destination-specific publishing guides and the Dashboard control.
- Existing GIF documentation source tests, docs AI tests, and the Dashboard production build pass.
- An independent PM review finds no unresolved factual contradiction or user-facing ambiguity.
- The deployed dev page renders the revised copy without browser console errors.

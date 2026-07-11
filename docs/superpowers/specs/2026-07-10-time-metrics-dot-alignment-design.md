# Time Metrics Dot Alignment Design

**Date:** 2026-07-10
**Branch:** `dev-time-metrics-dot-alignment`
**Status:** Approved for implementation

## Goal

Align each Time Metrics timeline dot with the first-line phase title, such as `Post created`, `Scheduled`, or `Published`, instead of centering the dot between the title and timestamp.

## Chosen Approach

Make a CSS-only adjustment in the shared platform-results styles.

- Keep the existing Time Metrics markup and three-column event grid.
- Top-align the dot within each event and use a small fixed offset so its center matches the phase-title line.
- Replace the single timeline-wide vertical rule with per-event connector segments running from the current dot center to the next dot center.
- Do not render a connector after the final `Published` dot.

This keeps the alignment correct when an event row grows while avoiding a JSX restructure for a small visual correction.

## Preserved Behavior

- Timestamp position, typography, and truncation remain unchanged.
- The right-side Duration badge retains its current vertical alignment and responsive behavior.
- Published-dot color and all other Time Metrics styling remain unchanged.
- Calendar and List View receive the adjustment together because both use the shared component.

## Testing

Add a source regression assertion covering:

- the dot's top alignment and title-line offset;
- per-event connectors for every event except the last;
- removal of the old timeline-wide connector rule.

Run the focused Time Metrics tests, dashboard production build, and dashboard regression suite before integrating into `dev`.

## Non-Goals

- No changes to duration calculations or timestamps.
- No changes to Time Metrics panel content or interaction.
- No changes to Queue Diagnostics, Submitted Settings, or platform result cards.

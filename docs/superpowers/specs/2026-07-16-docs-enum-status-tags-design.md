# Documentation Enum Status Tags Design

## Goal

Make reader-facing enum values in documentation tables visually distinct and
consistent with the compact Slack-docs-inspired status tags already approved for
the Publish GIFs guide. Preserve code-like enum values as inline code and avoid
changing ordinary prose that happens to contain the same words.

## Scope

The shared table renderer will recognize enum values only when both conditions
are true:

1. the cell is a plain string; and
2. its column header is a known semantic enum column.

The initial semantic columns are:

- `Support`;
- `Available`;
- `Required`;
- `Severity`;
- `Default on`;
- `Use this page?`;
- `UniPost status`.

This covers platform capability, analytics, inbox, and requirement tables;
notification channel and event tables; the Quickstart routing table; the
Instagram Stories requirement table; and the Publish GIFs status table.

The platform overview's dense capability matrices retain their existing check
and dash marks so that each row stays compact. Their textual `Limited` state will
use the shared partial-support tag.

The following are explicitly out of scope:

- CLI and API contract values such as `passed`, `input_required`, `read_only`,
  and HTTP status codes;
- enum-like words in prose, notes, callouts, and summary cards;
- descriptive values in semantic columns that are not members of the approved
  enum set, such as `Exactly 1 video`.

## Component Design

`DocsTable` will pass the current column header into its cell renderer. A small
pure resolver will normalize the header and cell value, then return a semantic
tone only for an approved header/value pair. Recognized cells render through one
shared enum-tag component; unrecognized cells continue through the existing rich
content renderer unchanged.

The Publish GIFs guide will pass plain `Supported` and `Coming soon` strings to
`DocsTable` and remove its page-local status component. The shared implementation
will preserve the approved appearance while eliminating page-specific markup and
CSS names.

## Visual Language

All enum tags share the approved B-style geometry:

- 24 px minimum height;
- 6 px corner radius;
- 9 px horizontal padding;
- compact semibold label;
- softly tinted background with no prominent border;
- equivalent light- and dark-theme contrast.

Tone mapping is semantic rather than purely lexical:

| Meaning | Values | Tone |
| --- | --- | --- |
| Positive support or enabled state | `Supported`, `Yes` | green |
| Partial or pending support | `Coming soon`, `Partial`, `Partially`, `Limited` | amber |
| Negative support state | `No` | red |
| Required input | `Required` | blue |
| Optional input | `Optional` | neutral gray |
| Invalid input | `Rejected` | red |
| Critical severity | `Critical` | red |
| High severity | `High` | orange |
| Medium severity | `Medium` | amber |

The same word may receive a tone only in an approved semantic column. For
example, `Supported` in a Notes column remains ordinary prose.

## Accessibility

Color is supplemental: every tag keeps its complete text label. Foreground and
background pairs must remain readable in both themes, and the tag must not rely
on hover state or animation to communicate meaning.

## Verification

Automated coverage will verify that:

- approved header/value pairs resolve to the expected tag tone;
- unknown values in semantic columns remain normal content;
- known enum words in unrelated columns remain normal content;
- Publish GIFs uses the shared renderer;
- the dense matrix `Limited` state uses the shared partial-support tag;
- the Dashboard production build succeeds.

Browser acceptance will inspect representative tables on the development,
staging, and production documentation sites in both light and dark themes. The
release proceeds through `origin/dev`, a `dev` to `staging` promotion PR, and a
`staging` to `main` production PR, with deployment checks and real-environment
verification at every stage.

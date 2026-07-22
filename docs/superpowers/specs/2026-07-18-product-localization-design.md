# Product Localization Design

## Goal

Make UniPost's core customer journey usable in each customer's preferred language without changing authenticated Dashboard URLs, adding model latency to page requests, or weakening the stability of the public API.

The first release will support:

- English (`en`)
- Spanish (`es`)
- Vietnamese (`vi`)
- Hungarian (`hu`)
- Simplified Chinese (`zh-CN`)
- Traditional Chinese (`zh-TW`)

English remains the source language and default fallback. Spanish is the pilot locale. The other locales are released one at a time after the pilot meets the acceptance criteria.

## Expected Outcome

A visitor can discover UniPost, review pricing, sign up, complete onboarding, use the core Dashboard and posting flow, manage billing, and receive essential system email in a supported language. A signed-in user's explicit language choice follows them across the marketing and application domains and across sessions.

Public localized pages have stable, indexable URLs. Authenticated Dashboard deep links remain unchanged. The runtime only loads approved translations and formats dynamic values; it never calls a large language model to translate page content.

## Scope

The first localization release covers:

- The shared public header and footer.
- The marketing homepage and pricing/conversion path.
- Sign-in and sign-up entry points, including the embedded Clerk components used by UniPost.
- Welcome and onboarding.
- The Dashboard shell and navigation.
- The core profile, connection, post creation, queue, billing, account, and settings experiences required to complete the primary customer journey.
- User-facing errors in those experiences when a stable error code is available.
- Essential transactional email for signup, posting failures, connection failures, quota, and billing.
- Localized metadata, canonical URLs, alternate-language links, and sitemap entries for localized public pages.

The first release does not cover:

- Translation of user-authored social post content.
- Complete localization of API documentation, blog posts, changelog entries, long-tail SEO pages, tools, or every solution page.
- Internal Admin pages.
- Translation of API field names, error codes, code samples, platform names, or other developer contracts.
- A legally authoritative translation of Terms or Privacy. English remains the authoritative legal text until translated versions receive legal review.
- Hebrew or another right-to-left locale. The implementation must avoid new direction-dependent layout assumptions so RTL can be added later.
- Runtime machine translation or translation generated on a page request.
- An Unleash feature flag. A locale becomes selectable only when its reviewed catalogs and route coverage are included in the deployed supported-locale registry.

## Current-State Constraints

UniPost currently serves public and authenticated surfaces from one Next.js application and distinguishes the marketing and application domains in `dashboard/src/proxy.ts`. The application root declares `lang="en"`, loads Latin-only font subsets, and has no shared locale or message system. Public metadata, Dashboard copy, API error messages, and email templates contain English text directly.

The public navigation currently renders the main links, a `Developer` dropdown, and then authentication controls. This structure allows a self-contained language selector to be inserted after `Developer` without reordering the authentication area.

The backend already has a per-user `users` table and an AI-provider routing abstraction. User locale belongs on the user, not the workspace, because members of the same workspace can prefer different languages.

## Architecture Decision

Use a hybrid URL strategy with one translation system:

- Public pages use localized URLs.
- Authenticated Dashboard URLs remain locale-free.
- Both surfaces use the same locale registry, message conventions, and frontend internationalization library.
- Backend-rendered email uses separate Go-owned catalogs that share the same locale identifiers and terminology contract.

Use `next-intl` for frontend message formatting, Server and Client Component integration, localized navigation, locale negotiation, and public alternate links.

### Public URL behavior

Use an `as-needed` locale prefix:

- `/pricing` is the English page.
- `/es/pricing` is Spanish.
- `/vi/pricing` is Vietnamese.
- `/hu/pricing` is Hungarian.
- `/zh-CN/pricing` is Simplified Chinese.
- `/zh-TW/pricing` is Traditional Chinese.

Do not localize route slugs in the first release. Stable English slugs reduce redirect risk and keep route mapping understandable.

Each published locale route must contain translated page content and translated metadata. UniPost must not publish an English page under a non-English locale URL. A public page that is outside the first-release localized route manifest continues to use its English canonical URL and is not emitted as a localized route.

### Dashboard URL behavior

Keep external application URLs unchanged:

- `/projects/{id}/posts`
- `/projects/{id}/billing`
- `/settings/account`

The proxy resolves the locale and performs any internal locale rewrite needed by `next-intl`; the locale never appears in the browser's Dashboard URL. Existing links, bookmarks, Clerk redirects, OAuth flows, support links, and analytics paths therefore remain stable.

The locale-aware proxy must compose, rather than replace, the existing responsibilities:

- Marketing host rewrites.
- Application host protection through Clerk.
- Public route exemptions.
- Static asset and internal endpoint exclusions.
- Existing country-cookie behavior.
- Locale negotiation, redirect, and internal rewrite behavior.

API paths, static assets, webhooks, and provider callback endpoints must not be locale-rewritten.

## Locale Registry

Maintain one canonical locale definition with generated TypeScript and Go representations. The registry defines:

- Locale ID.
- Native display name.
- Default fallback.
- Text direction.
- Public prefix.
- Date, number, and currency formatting locale.
- Whether the locale is released and selectable.

The generated artifacts must be checked into the repository, and CI must fail if regeneration produces a diff. English is the only fallback locale. Fallback chains must not jump between Simplified and Traditional Chinese.

The initial registry is:

| Locale | Display name | Direction | Fallback |
| --- | --- | --- | --- |
| `en` | English | `ltr` | None |
| `es` | Español | `ltr` | `en` |
| `vi` | Tiếng Việt | `ltr` | `en` |
| `hu` | Magyar | `ltr` | `en` |
| `zh-CN` | 简体中文 | `ltr` | `en` |
| `zh-TW` | 繁體中文 | `ltr` | `en` |

## Locale Resolution and Persistence

### Anonymous public request

Resolve locale in this order:

1. An explicit supported locale in the URL.
2. The `unipost_locale` cookie.
3. A best-fit match from `Accept-Language`.
4. English.

An explicit URL always wins and refreshes the locale cookie. Unsupported locale prefixes return 404 instead of silently creating an incorrect localized page.

### Authenticated Dashboard request

Resolve locale in this order:

1. The authenticated user's explicit `users.locale` value.
2. The `unipost_locale` cookie.
3. A best-fit match from `Accept-Language`.
4. English.

Add a nullable `locale` column to `users`. A null value means the user has not made an explicit selection, so existing users can still benefit from cookie and browser detection. Do not default the database column to English, because that would incorrectly override existing users' browser preferences.

Expose the resolved locale in the existing `GET /v1/me` response and add `PATCH /v1/me/locale`. The patch endpoint accepts only a released locale from the registry.

### Language selection

Selecting a language:

1. Resolves the equivalent public route or keeps the current Dashboard route.
2. Writes `unipost_locale` for one year with `Secure`, `SameSite=Lax`, `Path=/`, and the `.unipost.dev` domain in deployed environments.
3. Persists the selection through `PATCH /v1/me/locale` for an authenticated user.
4. Updates Clerk localization where an embedded Clerk component supports it.
5. Refreshes server-rendered content without discarding an in-progress form.

If the current page has unsaved post or form state and a refresh is required, the selector must warn before switching.

## Language Selector Design

On the public desktop header, place the language selector immediately to the right of `Developer` and immediately to the left of authentication controls:

```text
UniPost | Solutions | Tools | Pricing | Blog | Developer | Language | Sign in | Get Started Free
```

The selector must:

- Show the current language's native name.
- Use a chevron to communicate that it opens a menu.
- Use no national flag, because countries and languages are not equivalent.
- Show the selected state in the menu.
- Preserve the equivalent localized public route when one exists.
- Route to the English canonical page when the current public page has no localized counterpart, while clearly identifying that the destination is English.
- Support keyboard navigation, screen readers, touch, and visible focus states.

On mobile, place the language control in the navigation menu after `Developer` and before authentication actions.

In the Dashboard, expose the durable language preference in account settings. A compact shortcut may also appear in the user/sidebar menu, but both controls must use the same preference action.

## Translation Catalogs

English messages are the only editable source messages. Organize frontend catalogs by locale and business namespace:

```text
dashboard/messages/
  en/
    common.json
    marketing.json
    auth.json
    onboarding.json
    navigation.json
    connections.json
    posts.json
    billing.json
    settings.json
    errors.json
  es/
  vi/
  hu/
  zh-CN/
  zh-TW/
```

Use semantic keys such as:

```text
billing.checkout.paymentFailed
posts.composer.scheduleButton
onboarding.workspaceName.helper
```

Do not use the English sentence as the key.

Backend email catalogs live under the Go service and are embedded into the API binary. They share locale IDs, terminology, ICU-style variable contracts where supported, and completeness rules, but the Go build does not import frontend files.

Load only the namespaces required by the rendered surface. Do not send the complete translation corpus to every Client Component.

## Translation Boundaries

Translate:

- Navigation and interaction labels.
- Headings, descriptions, help text, validation guidance, and empty states.
- Public metadata and conversion copy.
- Plan explanations and billing guidance.
- Essential transactional email templates.

Do not translate:

- UniPost and third-party platform names.
- API paths, JSON keys, enum values, identifiers, code, log fields, or request IDs.
- User-authored captions, replies, workspace names, profile names, or account names.
- Currency merely because the language changed.

Dates, times, numbers, percentages, lists, and display names are formatted at runtime with locale-aware `Intl` APIs. Their labels are translated, but their values are not sent to a model.

The frontend must localize known backend failures from a stable `error.code` plus typed parameters. Unknown failures display a safe English fallback, preserve the `request_id`, and offer a support path. The public API continues returning stable English developer messages and codes.

## Translation Production Workflow

Large language models participate only in an offline content-production workflow.

1. A developer changes an English source message.
2. Tooling detects added, deleted, and source-changed keys.
3. The translation generator sends the changed namespace, UI context, glossary, non-translatable terms, ICU variables, length constraints, and nearby messages to the draft model.
4. Structured output returns the key, target locale, proposed translation, uncertain terms, and review notes.
5. Deterministic validation checks key parity, placeholders, ICU syntax, markup, links, and forbidden changes.
6. The quality model reviews critical strings and flags risks without silently overwriting the draft.
7. A native-language reviewer approves the changed strings.
8. Translation files and review state merge in the same pull request as the source change.

For released locales, changes to critical user-facing copy are atomic with the feature. CI blocks a pull request that changes critical English copy without current, reviewed translations for every released locale.

Non-critical Dashboard messages may temporarily fall back to English, but CI records explicit translation debt. A source hash marks an existing translation stale whenever its English source changes, even if the key is unchanged.

Use:

- `gpt-5.6-terra` with low reasoning as the default draft model.
- `gpt-5.6-sol` as a quality reviewer for pricing, signup, onboarding, billing, destructive actions, errors, and transactional email.
- OpenAI Batch API for large offline backfills.
- Structured Outputs for schema-constrained translation results.

Treat `translation` as a configurable AI surface. Do not hard-code model selection in translation logic. The provider and model can be overridden and audited, while page-serving code has no access to or dependency on the translation model.

Record the source hash, prompt version, effective model, generation time, and human review state for each changed translation unit. Do not send customer data or user-authored content through the product-copy translation pipeline.

## Terminology and Review Assets

Maintain:

- A glossary for UniPost-specific concepts such as Workspace, Profile, Hosted Connect, Queue, and Platform Credentials.
- A do-not-translate list for product names, platform names, API tokens, and code.
- Per-locale style guidance covering tone, capitalization, punctuation, and preferred terminology.
- Translation state keyed by locale, namespace, message key, and English source hash.

Machine-generated translation is never marked ready solely because it passed schema validation. Critical strings require native-language review.

## SEO and Public Content

For every published localized public page:

- Set the correct `<html lang>`.
- Generate localized title, description, Open Graph text, and structured data.
- Emit a self-referencing canonical URL.
- Emit `hreflang` alternates for all published equivalents.
- Use the English page as `x-default`.
- Include the localized URL in the sitemap.

Do not emit alternate links to missing or English-fallback localized pages. A critical public locale catalog or metadata failure stops the build.

Legal navigation and summaries may be localized, but the full legal text remains English and must be labeled as the authoritative version until a legally reviewed translation is available.

## Fonts and Layout

The current Latin-only font setup is insufficient.

- English, Spanish, Hungarian, and Vietnamese use a font configuration that includes the necessary Latin Extended and Vietnamese glyphs.
- Simplified Chinese uses a locale-specific Simplified Chinese system font stack.
- Traditional Chinese uses a locale-specific Traditional Chinese system font stack.
- Do not make every visitor download both Chinese font families.
- Keep code and identifiers in the existing monospace treatment.

Migrate touched layout styles toward logical properties such as `margin-inline`, `padding-inline`, and `inset-inline`. This does not add RTL in the first release, but avoids creating new blockers.

Use the non-production `en-XA` pseudo-locale to expand text and expose truncation, overflow, and fixed-width assumptions before human translations are available.

## Clerk

Pass the resolved locale to Clerk's supported embedded-component localization. Clerk localization is currently experimental, and the hosted Clerk Account Portal can remain English. Therefore:

- Authentication must always fall back to English without blocking sign-in.
- The acceptance suite must cover each Clerk surface that UniPost actually embeds.
- Any remaining English-only hosted Clerk surface must be documented in the localized UI before release.
- UniPost must not promise full third-party localization beyond Clerk's verified behavior.

## Error Handling and Observability

Handle failures as follows:

- Unsupported URL locale: 404.
- Unsupported stored user locale: fall back to English, repair the cookie, and record an event.
- Dashboard catalog load failure: fall back to English and emit an alertable event.
- Public critical catalog load failure: fail the build.
- Missing non-critical key: use English and emit `i18n_missing_key`.
- Missing critical key: fail CI or deployment.
- Missing localized email template: send English so the notification is not lost, and emit an alert.
- Unknown API error code: show the safe fallback, request ID, and support link.

Telemetry must include locale, namespace, key, route, application version, and fallback reason without including user-authored content.

Measure:

- Language selection and persistence success.
- English fallback rate by locale and namespace.
- Missing and stale key counts.
- Localized core-flow completion and error rates.
- Public localized page indexing and canonical correctness.
- Translation review lead time.

## CI and Pull Request Policy

For any released locale, a pull request that changes critical customer-facing English copy must contain the corresponding locale updates.

CI must verify:

- Locale registry generation is clean.
- Catalog key parity.
- No unknown or raw key reaches rendered output.
- ICU variables and markup match the English contract.
- Source hashes and review state are current.
- Critical namespaces are complete and approved.
- Public route and metadata manifests are complete.
- Email subject, HTML, and text templates share the same variable contract.
- No runtime page path imports or calls the translation-model client.

Code-only, style-only, and non-user-visible changes do not require catalog changes. Internal Admin copy and content outside the first-release scope do not block localization CI.

## Testing

### Unit and contract tests

- Locale parsing and best-fit matching.
- URL, cookie, user preference, browser header, and English fallback priority.
- Locale cookie attributes.
- `GET /v1/me` and `PATCH /v1/me/locale`.
- Registry generation and backend/frontend parity.
- Catalog completeness, stale detection, ICU variables, and non-translatable tokens.
- Error-code localization and unknown-error fallback.
- Locale-aware number, date, percentage, and list formatting.
- Email template selection and English fallback.

### UI and route tests

- Public English unprefixed routes and non-English prefixed routes.
- Dashboard URLs remain unchanged after switching language.
- Language selector placement immediately after `Developer`.
- Desktop and mobile language menus.
- Unsaved-form protection during language switching.
- Correct `<html lang>`, canonical, alternate links, metadata, and sitemap output.
- `en-XA` overflow and truncation checks.
- Keyboard, focus, screen-reader labeling, and touch behavior.

### End-to-end tests

Run a smoke path for every released locale. Run the full critical journey for Spanish, Simplified Chinese, and Traditional Chinese:

1. Visit the localized public entry page.
2. Review pricing.
3. Start Clerk registration or sign-in.
4. Complete onboarding.
5. Enter the Dashboard with an unchanged external URL.
6. Connect or inspect an account.
7. Create and validate a post.
8. Inspect queue or delivery state.
9. Open billing and account settings.
10. Change language and confirm persistence across both domains.
11. Verify an essential transactional email fixture.

Use mobile and desktop viewports and light and dark themes for key screenshots.

## Rollout

### Phase 1: Foundation

- Add locale registry, generated contracts, `next-intl`, proxy composition, user preference storage, catalog tooling, and observability.
- Extract English messages for the agreed core journey.
- Add pseudo-locale and CI checks.
- Do not expose an incomplete non-English locale.

### Phase 2: Spanish pilot

- Generate and review Spanish catalogs.
- Release Spanish public routes and selector entry.
- Complete local, deployed development, SEO, Clerk, email, and end-to-end acceptance.
- Fix architecture and workflow issues before adding another locale.

### Phase 3: Remaining locales

Release one at a time:

1. Simplified Chinese.
2. Traditional Chinese.
3. Vietnamese.
4. Hungarian.

Each locale must independently satisfy the same critical coverage, native review, deployment, and real-environment acceptance gates.

## Acceptance Criteria

- English, Spanish, Vietnamese, Hungarian, Simplified Chinese, and Traditional Chinese are represented by one validated locale contract.
- Spanish launches first; later locales cannot become selectable until their gates pass.
- Public non-English core pages have stable prefixed URLs, localized metadata, valid canonical URLs, and correct alternate links.
- English public pages retain their current unprefixed URLs.
- Dashboard URLs, bookmarks, Clerk redirects, OAuth callbacks, and API paths remain locale-free and functional.
- A user's manual language choice persists in `users.locale` and the cross-subdomain locale cookie.
- The public language selector appears directly to the right of `Developer`, uses native language names, and uses no flag.
- Critical user journeys contain no raw translation key and no English fallback in a released non-English locale.
- User-facing copy changes and their released-locale translations merge atomically in the same pull request.
- Translation models run only in the offline translation workflow.
- `gpt-5.6-terra` is the default draft model and `gpt-5.6-sol` only reviews critical copy unless evaluation justifies a configuration change.
- Essential localized email is selected from persisted templates; no email is translated at send time.
- User-authored content, API identifiers, code, and legal source text are not machine-translated by this project.
- Relevant unit, contract, build, SEO, email, and Dashboard regression tests pass.
- After each push to `origin/dev`, the development deployment completes and the changed locale flow is personally verified on the correct development domains before the task is reported complete.

## References

- Next.js internationalization guide: https://nextjs.org/docs/app/guides/internationalization
- `next-intl` routing setup: https://next-intl.dev/docs/routing/setup
- `next-intl` routing configuration: https://next-intl.dev/docs/routing/configuration
- `next-intl` proxy composition: https://next-intl.dev/docs/routing/middleware
- Clerk localization: https://clerk.com/docs/guides/customizing-clerk/localization
- OpenAI current model guidance: https://developers.openai.com/api/docs/guides/latest-model
- OpenAI Structured Outputs: https://developers.openai.com/api/docs/guides/structured-outputs
- OpenAI Batch API: https://developers.openai.com/api/docs/guides/batch

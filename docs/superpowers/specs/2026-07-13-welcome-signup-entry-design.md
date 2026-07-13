# Welcome Signup Entry Design

## Goal

Public UniPost calls to action must start Clerk registration for signed-out visitors, then send newly registered users to `/welcome` for onboarding. Signed-in visitors should go directly to the dashboard.

## Root cause

`/welcome` is an authenticated onboarding route, not a public registration route. Several public pages use ordinary anchor tags that navigate signed-out visitors directly to `https://app.unipost.dev/welcome`. Those anchors bypass Clerk's `SignUpButton`, so Clerk never starts the registration flow and the protected route returns a 404 to anonymous visitors.

## Design

- Keep `/welcome` protected and retain its existing first-name and organization onboarding form.
- Reuse the existing `MarketingCTA` client component on every public page that currently links directly to `/welcome`.
- Allow `MarketingCTA` to preserve page-specific button copy and the homepage arrow icon without duplicating authentication logic.
- Keep the existing Clerk post-signup redirect at `https://app.unipost.dev/welcome`.
- Add a source regression test that rejects direct public anchors or constants targeting `/welcome` and confirms the affected pages use `MarketingCTA`.

## Expected flow

1. A signed-out visitor selects a public signup CTA.
2. Clerk opens the registration flow.
3. After successful registration, Clerk redirects to `/welcome`.
4. The authenticated user completes onboarding and enters the dashboard.
5. A signed-in visitor selecting the same CTA goes directly to the dashboard.

## Scope

The change covers the homepage, blog index, blog article CTA, and public analytics tool CTAs. It does not redesign the onboarding page or add a custom Clerk sign-up route.

## Verification

- The regression test must fail before implementation and pass after implementation.
- The dashboard production build and dashboard regression suite must pass.
- After pushing `origin/dev`, the development deployment must finish successfully.
- In the real development environment, a signed-out homepage CTA must open Clerk registration instead of returning 404, and successful signup must land on `/welcome`.

# Feature Flags and Unleash Decommission

UniPost no longer uses Unleash-backed feature flags for product rollout.

Current release controls:

- Product packaging is enforced through plan gates, such as `plans.allow_inbox`, `plans.allow_analytics`, and `plans.white_label`.
- Third-party integrations are controlled by credentials and provider configuration, such as `LOOPS_API_KEY`, TikTok OAuth credentials, and AI provider routes.
- New feature rollout follows the standard development, staging, and production release flow instead of remote flag toggles.

Operational cleanup after this code ships:

1. Remove `FEATURE_FLAGS_PROVIDER`, `UNLEASH_URL`, `UNLEASH_SERVER_TOKEN`, `UNLEASH_APP_NAME`, and `UNLEASH_ENVIRONMENT` from Railway environments.
2. Confirm `/v1/me/plan-gates` returns only plan gates.
3. Shut down the Unleash Railway service after development, staging, and production verification pass.
4. Remove DNS for `flags.unipost.dev` after the service is shut down.

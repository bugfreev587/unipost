# Admin Users Failed Posts Design

## Goal

Add a `Failed` column to `/admin/users` that shows each user's failed posts for the current calendar month, make that count link to `/admin/errors` with the matching user and month filters applied, and remove row-level detail opening so only the `View` button opens the user detail panel.

## Expected Outcome

- `/admin/users` includes a `Failed` column between `Scheduled` and `Posts Used`.
- The value is the count of distinct social posts created this calendar month that are failed at the parent post level or have at least one failed platform result.
- Nonzero failed counts link to `/admin/errors?user_id=<user_id>&period=this_month`.
- `/admin/errors` accepts the exact `user_id` filter and a `period=this_month` filter, applies both to the list API request, and shows only matching failure rows.
- Rows in `/admin/users` are no longer clickable; the existing `View` button remains the only detail-panel trigger.

## Architecture

The backend admin user list remains the source of truth for per-row aggregate values. `api/internal/handler/admin.go` will add `failed_posts_this_month` to the `adminUserRow` response and compute it in the existing user-list SQL with a distinct-post count.

The admin post-failures list already supports a user-scoped query internally. The public admin errors endpoint will expose that scope through `user_id` and add `period=this_month` as a normalized date-window option. The dashboard API client and Errors page will pass those parameters through from URL state.

## UI Behavior

The users table stays dense and operational. The new count uses the existing admin link styling, with a danger-toned emphasis only for nonzero counts. Zero counts are muted plain text so the table remains scannable.

The users table wrapper uses the existing static-table class so the cursor and hover treatment no longer imply row navigation. The `View` button keeps the current detail behavior.

## Testing

- Backend source tests verify the user-list response and SQL include `failed_posts_this_month`.
- Backend source tests verify post-failure filters include exact `user_id` and `period=this_month`.
- Dashboard source tests verify the new API types/params, table column order, failed-count link, static rows, and updated empty-state column span.
- Local validation runs focused source tests first, then full backend tests and dashboard build before merging to local `dev`.

## Release Verification

After pushing local `dev` to `origin/dev`, wait for triggered checks and the development deployment. Verify `https://dev-app.unipost.dev/admin/users` shows the new column, row clicks no longer open detail, `View` still opens detail, and a nonzero failed count opens `https://dev-app.unipost.dev/admin/errors` filtered to that user and this month.

# Admin Email Filters Design

## Goal

Improve `/admin/email` so an administrator can narrow the email activity list by an exact recipient email and an inclusive attempted-date range, while retaining every existing filter. The existing status filter remains part of the supported filter set. Audit filtering is explicitly out of scope.

## User experience

The filter bar keeps the existing search, status, provider, event key, quota trigger, period, row-limit controls, and period shortcuts.

It adds:

- An Email select whose default option is `All emails`.
- One option for every distinct, non-empty recipient `email` in the full admin email notification data set.
- Start and end date inputs for the attempted-date range.

Email options are deduplicated and sorted case-insensitively. Selecting an email performs a case-insensitive exact match against the recipient email snapshot shown in the Email table column. Owner email is not included in the option set and does not match this filter.

All active filters are combined with AND semantics. Changing Email, Status, either date, or any existing structured filter resets pagination to the first page. The existing loading, error, and empty states remain in place.

## Date semantics

The date range filters the `attempted_at` value displayed in the Attempted column.

Dates are interpreted in the administrator's browser-local timezone:

- The start date is inclusive from local 00:00.
- The end date is inclusive through the entire local calendar day.
- The frontend sends the start boundary as an RFC 3339 timestamp and sends the exclusive boundary at local 00:00 on the day after the selected end date.
- The backend applies `attempted_at >= start_at` and `attempted_at < end_at`.

Either boundary can be used independently. When both are present and the end date is earlier than the start date, the UI shows an inline validation error and does not issue a list request until the range is valid.

## API design

Add a read-only admin endpoint:

`GET /v1/admin/email-notifications/filter-options`

It returns a data object containing:

```json
{
  "emails": ["person@example.com"]
}
```

The endpoint uses the same unified email notification data set as the list endpoint, selects distinct non-empty recipient emails, and sorts them case-insensitively. It is protected by the existing admin route group.

Extend `GET /v1/admin/email-notifications` with these optional query parameters:

- `email`: recipient email, matched case-insensitively and exactly.
- `start_at`: inclusive RFC 3339 attempted-time boundary.
- `end_at`: exclusive RFC 3339 attempted-time boundary.

Malformed timestamps or `end_at <= start_at` return a validation error. Existing parameters and response shape remain backward compatible.

## Frontend data flow

On page load, the client requests filter options independently of the paginated email list. A filter-options failure is surfaced without discarding an already loaded list.

The selected email defaults to `all`. The list request omits `email` when `all` is selected. Local date strings remain in component state and are converted to timestamp boundaries only when list parameters are built.

The options request is not repeated on every filter change. A manual page refresh reloads both the options and the list so newly observed recipient emails become selectable.

## Scope boundaries

- No Audit filter is added.
- Existing filters are not removed or reinterpreted.
- The general Search field continues to match its current broad set of email, workspace, event, and ID fields.
- The existing Period filter continues to match the stored quota period and is not replaced by Date range.
- No feature flag is added.

## Testing

Backend tests cover:

- the unified query's exact recipient-email predicate;
- inclusive-start and exclusive-end attempted-time predicates;
- distinct, non-empty, case-insensitively sorted email options;
- valid independent date boundaries;
- malformed timestamps and reversed or zero-length ranges;
- admin route registration for the options endpoint.

Frontend tests cover:

- serialization of `email`, `start_at`, and `end_at`;
- omission of the default `all` email value;
- conversion of inclusive local calendar dates to RFC 3339 half-open boundaries;
- pagination reset when Email or either date changes;
- invalid-range handling without issuing an invalid list request;
- rendering `All emails` plus the returned options.

Required validation includes backend Go tests, the dashboard production build, and the dashboard regression suite when Playwright browsers are installed. After pushing `origin/dev`, the triggered checks and deployments must finish successfully, followed by browser acceptance on `https://dev-app.unipost.dev/admin/email`.

## Acceptance criteria

1. Email defaults to `All emails` and lists every distinct non-empty recipient email, not just emails from the current page.
2. Selecting an email returns only rows whose displayed recipient Email matches it case-insensitively.
3. Status continues to filter the same statuses as before.
4. A start-only, end-only, or complete Date range filters Attempted timestamps with inclusive calendar-day semantics in the viewer's local timezone.
5. Email, Status, Date range, and all retained filters work together with AND semantics.
6. Filter changes reset pagination, and loading, empty, and error states remain usable.
7. No Audit filter or feature flag is introduced.

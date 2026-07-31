# Secure Remote Media Fetcher and Pinterest Adoption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Safely retrieve untrusted customer media URLs, stage verified bytes on UniPost-controlled storage, and make Pinterest the first consumer without putting network-security policy in the Pinterest adapter.

**Architecture:** A provider-neutral `safefetch` package owns URL, DNS, IP, peer, redirect, timeout, byte-limit, and media-sniffing policy. It streams to a private temporary file and returns verified metadata. Storage persists the file and owns lifecycle/public URL. Pinterest distinguishes UniPost-managed media from external URLs, invokes storage only after destination preflight, and maps typed fetch failures into its product contract.

**Tech Stack:** Go 1.24, `net/http`, `net.Resolver`, custom `http.Transport`, TLS, `io` streaming, SHA-256, R2/S3 storage, existing Go test suite, Next.js structured failure presentation.

---

## Preconditions and release boundary

- Workstream A checkpoint on the same branch has passed local, exact-SHA Preview, Railway PR Environment, Vercel Preview, deployed regression, and browser acceptance.
- This workstream must remain disabled from external URL staging until its complete negative security suite passes and AppSec approves the exact head SHA.
- No feature flag or environment bypass is added. Before approval, the integration commits may exist on the Draft PR branch, but preview/development acceptance must not send arbitrary external URLs through the new fetcher.
- Existing `storage.Client.UploadFromURL` is not an approved implementation baseline and remains unavailable to the Pinterest external-URL path.

## Stable provider-neutral interface

Use these names across all tasks:

```go
// api/internal/safefetch/fetcher.go
package safefetch

type ErrorKind string

const (
	ErrorInvalidURL           ErrorKind = "invalid_url"
	ErrorForbiddenDestination ErrorKind = "forbidden_destination"
	ErrorRedirectRejected     ErrorKind = "redirect_rejected"
	ErrorSourceNotFound       ErrorKind = "source_not_found"
	ErrorSourceRejected       ErrorKind = "source_rejected"
	ErrorSourceTemporary      ErrorKind = "source_temporary"
	ErrorTimeout              ErrorKind = "timeout"
	ErrorTooLarge             ErrorKind = "too_large"
	ErrorUnsupportedMedia     ErrorKind = "unsupported_media"
	ErrorPeerMismatch         ErrorKind = "peer_mismatch"
)

type FetchError struct {
	Kind       ErrorKind
	HTTPStatus int
	Temporary  bool
}

type Policy struct {
	MaxBytes           int64
	AllowedMediaTypes  []string
	MaxRedirects       int
}

type Result struct {
	Path        string
	MediaType   string
	SizeBytes   int64
	SHA256Hex   string
}

func (r *Result) Open() (*os.File, error)
func (r *Result) Close() error

type Fetcher interface {
	Fetch(context.Context, string, Policy) (*Result, error)
}
```

`FetchError.Error()` returns fixed sanitized text keyed only by `Kind`; it never includes input URL, query, userinfo, DNS answers, redirect target, peer address, or response body. `Result.Close()` is idempotent and deletes the private temporary file.

Production constructor:

```go
type Config struct {
	ConnectTimeout        time.Duration
	ResponseHeaderTimeout time.Duration
	IdleReadTimeout       time.Duration
	TotalTimeout          time.Duration
	TempDir               string
}

func New(config Config) Fetcher
func DefaultConfig() Config
```

## Task 1: Enforce URL, hostname, and IP policy

**Files:**

- Add: `api/internal/safefetch/errors.go`
- Add: `api/internal/safefetch/policy.go`
- Add: `api/internal/safefetch/policy_test.go`

- [ ] **Step 1: Add failing table tests for URL and address policy**

Test direct inputs and resolved answers for:

- malformed URL, missing host, non-HTTP schemes, URL userinfo;
- IPv4 unspecified, loopback, RFC1918 private, link-local, multicast, broadcast, documentation, benchmark, carrier-grade NAT, and reserved ranges;
- IPv6 unspecified, loopback, unique-local, link-local, multicast, documentation, IPv4-mapped prohibited addresses, and other non-global-unicast ranges;
- metadata IPs including `169.254.169.254`, `169.254.170.2`, and IPv6 link-local equivalents;
- metadata hostnames including `metadata.google.internal` and common cloud metadata aliases, case-insensitively and after trimming one trailing dot;
- mixed DNS answers containing one public and one prohibited address;
- a public-only answer such as `8.8.8.8`.

Use the standard library's `net/netip` ranges and an explicit denied-CIDR table. The decision rule is fail-closed: every returned address must be public and permitted.

- [ ] **Step 2: Run policy tests and prove RED**

From `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/safefetch -run 'URLPolicy|AddressPolicy|Metadata' -count=1
```

Expected: package or functions do not exist.

- [ ] **Step 3: Implement sanitized errors and pure policy functions**

Add unexported helpers:

```go
func parseWebURL(raw string) (*url.URL, error)
func normalizeHostname(host string) (string, error)
func validateResolvedAddresses(host string, addresses []netip.Addr) error
func isProhibitedAddress(addr netip.Addr) bool
func isMetadataHostname(host string) bool
```

Reject a hostname with no answers. Unmap IPv4-mapped IPv6 before classification. Do not use string-prefix IP checks.

- [ ] **Step 4: Run policy tests and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/safefetch -run 'URLPolicy|AddressPolicy|Metadata' -count=1
```

Expected: `ok`.

- [ ] **Step 5: Commit pure policy**

```bash
git add api/internal/safefetch/errors.go api/internal/safefetch/policy.go api/internal/safefetch/policy_test.go
git commit -m "feat: add safe remote destination policy"
```

## Task 2: Pin DNS resolution to the verified network peer

**Files:**

- Add: `api/internal/safefetch/network.go`
- Add: `api/internal/safefetch/network_test.go`

- [ ] **Step 1: Add failing resolver/dialer seam tests**

Define internal seams:

```go
type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type dialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type pinnedTarget struct {
	Hostname string
	Port     string
	IP       netip.Addr
}
```

Tests must prove:

1. all DNS answers are validated before selecting a target;
2. the dial address is the selected literal IP plus original port, never the hostname;
3. the transport retains original hostname for TLS ServerName and certificate validation;
4. `conn.RemoteAddr()` must equal one of the resolved/pinned IPs;
5. a resolver that changes from public to private after validation cannot redirect the dial because no second hostname lookup occurs;
6. a peer that reports loopback or an unvalidated public IP returns `ErrorPeerMismatch`.

The test dialer may connect to a local `httptest.Server`, but wrap `RemoteAddr()` to report the pinned test-public IP. This keeps tests deterministic while exercising the production peer check.

- [ ] **Step 2: Run network tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/safefetch -run 'Pinned|Rebinding|Peer|TLS' -count=1
```

Expected: missing network implementation.

- [ ] **Step 3: Implement resolve-once, dial-by-IP, peer-verified connections**

Resolve using `LookupNetIP(ctx, "ip", hostname)`, validate the complete set, select deterministically, dial the literal IP, and wrap the connection only after checking `RemoteAddr`. Set `TLSClientConfig.ServerName` to the original hostname and leave `InsecureSkipVerify=false`.

Add a connection wrapper whose `Read` sets a new read deadline of `now + IdleReadTimeout`, while writes retain the normal transport behavior.

- [ ] **Step 4: Run network tests with the race detector and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test -race ./internal/safefetch -run 'Pinned|Rebinding|Peer|TLS' -count=1
```

Expected: `ok`, no race report.

- [ ] **Step 5: Commit pinning controls**

```bash
git add api/internal/safefetch/network.go api/internal/safefetch/network_test.go
git commit -m "feat: pin safe fetch DNS and peers"
```

## Task 3: Manually validate redirects and enforce timeouts

**Files:**

- Add: `api/internal/safefetch/fetcher.go`
- Add: `api/internal/safefetch/fetcher_redirect_test.go`
- Add: `api/internal/safefetch/fetcher_timeout_test.go`

- [ ] **Step 1: Add failing redirect tests**

Cover:

- one same-host redirect succeeds after a second resolution/validation;
- a public URL redirecting to private IPv4/IPv6 or metadata host fails;
- a cross-host public redirect succeeds and uses the new host for TLS/DNS pinning;
- HTTPS-to-HTTP downgrade fails;
- redirect to `file:`, `ftp:`, `data:`, or a relative URL resolving to userinfo fails;
- a loop fails even before the hop cap;
- 5 redirects are permitted and the sixth is rejected;
- each hop re-runs full DNS policy, and no automatic redirect occurs.

- [ ] **Step 2: Add failing timeout tests**

Use deterministic fake clocks/connections where possible and local servers otherwise. Cover connect timeout, response-header timeout, idle-read timeout, and total-operation context deadline. All return `FetchError{Kind: ErrorTimeout, Temporary: true}` and no URL in the message.

- [ ] **Step 3: Run redirect/timeout tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/safefetch -run 'Redirect|Timeout|IdleRead' -count=1
```

Expected: missing fetcher and redirect state machine.

- [ ] **Step 4: Implement one-request-per-hop fetch control**

Construct a new pinned transport for every hop. Set `CheckRedirect` to `http.ErrUseLastResponse` even though the loop sends requests manually. Track canonical scheme/host/path/query hashes internally for loop detection without placing them in errors or logs. Resolve relative `Location` values against the current URL, then re-run `parseWebURL`, downgrade check, DNS validation, and pinning.

Wrap the full operation with `context.WithTimeout(config.TotalTimeout)`. Set connect and response-header timeouts in the transport and idle-read deadlines on the connection.

- [ ] **Step 5: Run tests and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test -race ./internal/safefetch -run 'Redirect|Timeout|IdleRead' -count=1
```

Expected: `ok`.

- [ ] **Step 6: Commit redirect and timeout controls**

```bash
git add api/internal/safefetch/fetcher.go api/internal/safefetch/fetcher_redirect_test.go api/internal/safefetch/fetcher_timeout_test.go
git commit -m "feat: validate safe fetch redirects and timeouts"
```

## Task 4: Stream bounded verified media to a private temporary file

**Files:**

- Add: `api/internal/safefetch/media.go`
- Add: `api/internal/safefetch/media_test.go`
- Modify: `api/internal/safefetch/fetcher.go`

- [ ] **Step 1: Add failing response and streaming tests**

Cover:

- `200` with valid JPEG, PNG, GIF, WebP, and supported video signatures as defined by existing Pinterest capabilities;
- `404` and other non-retryable `4xx` map to permanent source errors without body disclosure;
- `429` and `5xx` map to temporary source errors;
- missing body and zero-byte media fail permanently;
- header says image while bytes are HTML, JSON, SVG, or an executable signature;
- extension/query says image while bytes are unsupported;
- ISO-BMFF image, unknown, and generic-only brand combinations are rejected even when the major brand is `isom`; the complete declared `ftyp` box must fit in the bounded sample;
- a URL suffix cannot select a category budget: extensionless video uses the detected-video ceiling and a `.mp4` URL serving image bytes uses the detected-image ceiling;
- an exact `MaxBytes` response succeeds;
- `MaxBytes+1` fails after reading at most `MaxBytes+1`, closes the body, and removes the temp file;
- cancellation removes the temp file;
- success creates a `0600` file, correct SHA-256, size, detected MIME, and idempotent cleanup.

Use a counting reader to prove the implementation does not consume the rest of a large or infinite response after exceeding the bound.

- [ ] **Step 2: Run media tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/safefetch -run 'Media|Oversize|TemporaryFile|MIME' -count=1
```

Expected: no result streaming or MIME verification.

- [ ] **Step 3: Implement bounded streaming and byte sniffing**

Create the file with `os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`. Read at most the 512-byte sniff prefix first under the absolute policy ceiling, detect and normalize MIME, then select the strictest configured ceiling for that detected MIME before streaming the remainder through `io.LimitReader(remaining+1)` while hashing. URL extensions and query strings never select a limit. ISO-BMFF classification is fail-closed: require the entire declared `ftyp` box in the prefix, allow only reviewed brands, reject any unrecognized compatible brand, and require a definitive video brand rather than generic container brands alone. Never call unbounded `io.ReadAll`.

On every error, close and remove the partial file. On success, sync/close before returning `Result`. Ensure `Result.Open` opens read-only.

- [ ] **Step 4: Run the complete safe-fetch suite and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test -race ./internal/safefetch -count=1
```

Expected: `ok`, no race report.

- [ ] **Step 5: Commit bounded media validation**

```bash
git add api/internal/safefetch
git commit -m "feat: stream and verify remote media safely"
```

## Task 5: Hand verified media to existing storage

**Files:**

- Add: `api/internal/storage/external_media.go`
- Add: `api/internal/storage/external_media_test.go`
- Add: `api/internal/storage/publishing_object_lifecycle.go`
- Add: `api/internal/db/migrations/136_publishing_pull_object_lifecycle.sql`
- Add: `api/internal/db/queries/publishing_pull_objects.sql`
- Add: `api/internal/db/publishing_pull_objects_transactions.go`
- Add: `api/internal/db/publishing_pull_objects_postgres_integration_test.go`
- Add: `api/internal/handler/publishing_pull_objects.go`
- Modify: `api/internal/storage/r2.go`
- Modify: `api/internal/storage/media.go`
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_posts_media_retention.go`
- Modify: `api/internal/worker/media_cleanup.go`

- [ ] **Step 1: Add failing storage-boundary tests**

Use a fake `safefetch.Fetcher` and an injected narrow file uploader function. Assert:

- successful fetched bytes are stored under `pull/<sha256>.<detected-extension>`;
- public URL comes from existing `PublicURL` logic;
- the result temp file is removed after success and upload failure;
- the object key contains no source hostname, path, or query;
- identical content resolves to the same content-addressed key;
- a unique lifecycle usage ID is persisted for every staging attempt before upload and contains no source URL;
- each new usage starts as `upload_pending` with a 15-minute durable cleanup lease, becomes active only after upload succeeds, and an expired pending lease becomes cleanup-eligible after an abandonment persistence failure;
- upload failure marks only that attempt's usage immediately eligible for cleanup, preserving concurrent successful usages for the same post/content;
- activation and abandonment use a five-second bounded context derived through `context.WithoutCancel`, so request cancellation cannot suppress lifecycle persistence;
- a shared object remains while any post usage is active or inside retention;
- reservation and cleanup serialize through the same object-row lock, then cleanup rechecks usage eligibility in a later statement inside the same transaction; two-session PostgreSQL tests cover both reservation-wins and cleanup-wins ordering;
- cleanup claims the object before R2 deletion, releases the claim after R2 failure, and safely retries database-finalization failure;
- a fetch failure performs no storage call;
- a storage failure is distinguishable from a fetch failure and remains temporary.

- [ ] **Step 2: Run storage tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/storage -run 'ExternalMedia|VerifiedMedia' -count=1
```

Expected: storage has no approved safe-fetch handoff.

- [ ] **Step 3: Implement the narrow storage API**

Add:

```go
type ExternalMediaResult struct {
	PublicURL string
	ObjectKey string
	MediaType string
	SizeBytes int64
	SHA256Hex string
}

func (c *Client) StageExternalMedia(ctx context.Context, rawURL string, policy safefetch.Policy) (ExternalMediaResult, error)
```

Attach server-owned workspace/post lifecycle metadata to the dispatch context. Reserve `publishing_pull_objects` plus a unique per-attempt `publishing_pull_object_usages` row atomically before `PutFile`, return its usage ID, and initialize it as `upload_pending` with a database-clock 15-minute deadline. After `PutFile`, activate only that ID; on failure, abandon only that ID. Both lifecycle writes use independent five-second bounded contexts derived with `context.WithoutCancel`. A failed abandonment write self-heals when the pending deadline expires. Terminal post transitions update all usages for the post with `mediaretention.RetentionForPlanStatus`. Reservation and cleanup must contend on the same object row; cleanup first locks candidates, then checks usages in a separate Read Committed statement inside that transaction before marking an object deleting. The existing media cleanup worker deletes only claimed objects whose every usage is past deadline. This remains provider-neutral and introduces no Pinterest-specific retention window.

`storage.New` initializes a default production fetcher. The method fetches, defers `Result.Close`, derives extension only from detected MIME, calls existing `PutFile`, and returns the existing public URL. It does not call `UploadFromURL`.

Keep test seams unexported: a `stageExternalMedia` helper accepts `safefetch.Fetcher` and a `putFile` function. Production `Client.StageExternalMedia` supplies the real dependencies.

- [ ] **Step 4: Run storage and safe-fetch suites and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test -race ./internal/safefetch ./internal/storage -count=1
PUBLISHING_RESTRICTION_TEST_DATABASE_URL="$LOCAL_ISOLATED_POSTGRES_URL" GOCACHE=/tmp/unipost-go-build go test -tags=integration ./internal/db -run '^TestPublishingPullObjectClaimCoordinatesWithReservations$' -count=10
```

Expected: `ok`.

- [ ] **Step 5: Commit storage handoff**

```bash
git add api/internal/storage/external_media.go api/internal/storage/external_media_test.go api/internal/storage/r2.go api/internal/storage/media.go
git commit -m "feat: stage verified external media"
```

## Task 6: Preserve media provenance through generic dispatch

**Files:**

- Modify: `api/internal/platform/adapter.go`
- Modify: `api/internal/platform/adapter_test.go`
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_posts_test.go`

- [ ] **Step 1: Add failing provenance tests**

Add an additive field:

```go
type MediaOrigin string
const (
	MediaOriginExternal MediaOrigin = "external"
	MediaOriginManaged  MediaOrigin = "managed"
)

type MediaItem struct {
	URL string
	Kind MediaKind
	Origin MediaOrigin
}
```

Test that request `media_urls` become `external`, resolved `media_ids` become `managed`, and other adapters continue receiving the same URL/kind order. Empty legacy `Origin` is treated as external by Pinterest, the conservative choice.

- [ ] **Step 2: Run focused tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/handler -run 'MediaOrigin|ManagedMedia' -count=1
```

Expected: compile failures for the missing provenance field.

- [ ] **Step 3: Build media items without collapsing provenance**

In `publishOneContext`, stop combining URL strings before `MediaFromURLs`. Build external items from `pp.MediaURLs`, resolve `MediaIDs`, then append managed items with the existing kind sniffing. Do not add Pinterest logic to the handler.

- [ ] **Step 4: Run platform and handler tests and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/handler -run 'MediaOrigin|ManagedMedia' -count=1
```

Expected: `ok`.

- [ ] **Step 5: Commit provenance**

```bash
git add api/internal/platform/adapter.go api/internal/platform/adapter_test.go api/internal/handler/social_posts.go api/internal/handler/social_posts_test.go
git commit -m "refactor: preserve publishing media provenance"
```

## Task 7: Adopt safe staging in Pinterest after board preflight

**Files:**

- Modify: `api/internal/platform/pinterest.go`
- Modify: `api/internal/platform/pinterest_test.go`
- Modify: `api/internal/handler/social_post_queue_test.go`
- Modify: `api/internal/platform/dispatch_context.go`
- Modify: `api/internal/platform/dispatch_context_test.go`
- Modify: `api/internal/integrationlogs/normalize.go`
- Modify: `api/internal/integrationlogs/normalize_test.go`
- Modify: `dashboard/tests/post-result-errors.test.mts`
- Modify: `dashboard/tests/dashboard-regression.spec.ts`

- [ ] **Step 1: Add failing adapter integration tests**

Inject a fake staging interface:

```go
type PinterestMediaStager interface {
	StageExternalMedia(context.Context, string, safefetch.Policy) (storage.ExternalMediaResult, error)
}
```

Assert:

1. valid external image is staged after board preflight and create Pin uses only the staged URL;
2. `MediaOriginManaged` bypasses staging and keeps the owned publishing URL;
3. source 404/type/size failures stop before create Pin with `media_error`, `media_preflight`, `fix_media`, permanent;
4. source timeout/5xx and storage failure stop before create Pin with `temporary_platform_error`, `media_preflight`, `retry_later`, retriable;
5. board failure prevents any media fetch;
6. errors, debug records, and events contain no raw media URL or query.

- [ ] **Step 2: Run focused tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/handler -run 'Pinterest.*Media|MediaPreflight' -count=1
```

Expected: external ordinary URLs bypass UniPost staging.

- [ ] **Step 3: Implement the Pinterest-only mapping layer**

After `preflightBoard` succeeds, branch by `MediaItem.Origin`. Managed media uses the existing URL. External media calls `StageExternalMedia` with explicit Pinterest MIME and byte policy derived from existing platform capabilities.

Map provider-neutral errors without leaking their internal details:

- `ErrorSourceNotFound`, `ErrorSourceRejected`, `ErrorTooLarge`, `ErrorUnsupportedMedia`, invalid URL, and forbidden destination: `media_error`, permanent, `media_preflight`, public reason `customer_media_unreachable` or the precise normalized media reason.
- `ErrorSourceTemporary`, `ErrorTimeout`, and storage failure: `temporary_platform_error`, retriable, `media_preflight`.

Emit `pinterest_media_preflight_failed` or `pinterest_media_staged` with post/result/workspace/account IDs, safe normalized reason, size/type/duration, and no URL. Customer-input failures are tagged so the infrastructure SLO query excludes them.

- [ ] **Step 4: Complete Dashboard media-copy regression**

Assert `media_error` plus `failureStage=media_preflight` renders:

```text
Pinterest could not use this media. Replace it with a publicly available supported image or video.
```

The code branches on structured fields, never on `error_message`.

- [ ] **Step 5: Run integration and Dashboard tests and prove GREEN**

From `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test -race ./internal/safefetch ./internal/storage ./internal/platform ./internal/handler -count=1
```

From `dashboard/`:

```bash
node --test tests/post-result-errors.test.mts
npx playwright test tests/dashboard-regression.spec.ts --grep 'Pinterest.*media'
```

Expected: all commands exit `0`.

- [ ] **Step 6: Commit Pinterest adoption**

```bash
git add api/internal/platform/pinterest.go api/internal/platform/pinterest_test.go api/internal/handler/social_post_queue_test.go api/internal/platform/dispatch_context.go api/internal/platform/dispatch_context_test.go api/internal/integrationlogs/normalize.go api/internal/integrationlogs/normalize_test.go dashboard/tests/post-result-errors.test.mts dashboard/tests/dashboard-regression.spec.ts
git diff --cached --name-only
git commit -m "feat: safely stage Pinterest external media"
```

## Task 8: Run the release-blocking security and local verification gates

- [ ] **Step 1: Run the named negative security suite**

From `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test -race ./internal/safefetch -run 'URLPolicy|AddressPolicy|Metadata|Pinned|Rebinding|Peer|TLS|Redirect|Timeout|IdleRead|Media|Oversize|TemporaryFile|MIME' -count=10
```

Expected: ten clean iterations, no races, hangs, skips, or flakes.

- [ ] **Step 2: Run all backend and Dashboard local gates**

From `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./...
```

From `dashboard/`:

```bash
npm run build
npm run test:regression:dashboard
```

From repository root:

```bash
git diff --check
git status --short
```

Expected: every command exits `0`; no pending unrelated files.

- [ ] **Step 3: Obtain AppSec approval on the exact SHA**

Record:

```bash
git rev-parse HEAD
git diff --stat <workstream-a-checkpoint-sha>...HEAD
git diff --name-only <workstream-a-checkpoint-sha>...HEAD
```

Request review of the exact SHA and attach negative-suite output to the Draft PR. Required review topics are URL policy, full IPv4/IPv6 range table, metadata blocks, mixed DNS, resolve/dial pinning, peer equality, TLS SNI/certificate behavior, redirect revalidation/downgrade, all four timeouts, bounded streaming, MIME sniffing, cleanup, and redaction.

AppSec approval must be explicit in a PR review or linked review record and identify the reviewed SHA. A comment that only acknowledges the request is not approval. Any code change afterward invalidates approval and requires rerunning the security suite and re-approval.

- [ ] **Step 4: Stop on a missing or failed gate**

If the suite fails, flakes, times out, skips, cannot start, or AppSec does not approve the exact SHA, do not enable or accept external URL staging in Preview/development. Report the exact SHA, test, error, logs, and review state. Workstream A checkpoint remains valid only for its separately recorded SHA.

## Task 9: Complete Draft PR, exact-SHA Preview acceptance, and `dev` verification

- [ ] **Step 1: Audit branch content before the final push**

```bash
git fetch origin
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
git status --short
```

Every unique commit/file must belong to the approved PRD. Unrelated or unidentified changes are a hard blocker.

- [ ] **Step 2: Push the owned branch and monitor all exact-SHA gates**

```bash
git push origin dev-pinterest-403-analysis
```

Update the Draft PR with Workstream A checkpoint evidence, Workstream B security evidence, AppSec approval, and current head SHA. Monitor GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and Codex browser acceptance until all succeed on that SHA.

Preview acceptance includes:

1. Workstream A valid board and known failure scenarios still pass.
2. Valid external media stages to a UniPost URL and publishes through Pinterest.
3. Source `404` returns `media_error/fix_media` before create Pin.
4. Temporary source/staging failure follows retry policy.
5. Observability distinguishes customer input from infrastructure failure.
6. Browser renders board and media guidance from structured fields.

- [ ] **Step 3: Mark ready and merge only after all gates pass**

Re-fetch and repeat the commit/file audit, verify approvals/checks still match the head SHA, mark the PR ready, and merge to `dev` through GitHub. Do not push directly to `dev`.

- [ ] **Step 4: Monitor official development deployment**

Wait for all triggered GitHub, Railway `dev`, and Vercel `unipost-dev` checks/deployments. A queued, cancelled, skipped, timed-out, or SHA-mismatched result is failed.

- [ ] **Step 5: Perform browser/API acceptance on official development domains**

Verify on:

- API: `https://dev-api.unipost.dev`
- App: `https://dev-app.unipost.dev`

Repeat the changed critical scenarios and record deployment SHA/URLs. Only after this succeeds may the full PRD Goal be marked complete. Do not open staging or production promotion PRs without a separate explicit release instruction.

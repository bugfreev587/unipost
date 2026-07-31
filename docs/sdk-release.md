# SDK Release Guide

UniPost publishes four independent SDK repositories. Feature work is reviewed
and merged in each repository before any version-only release commit or tag is
created.

## Repositories and current 0.7.0 feature baselines

| SDK | Public source | X account-read feature baseline |
| --- | --- | --- |
| JavaScript | `github.com/unipost-dev/sdk-js` | `c6c1559bf5787011e49cdcd2b5c314b9026255fa` |
| Python | `github.com/unipost-dev/sdk-python` | `05ccc9b9c9c46e31a7a54e333540cf8e59164823` |
| Go | `github.com/unipost-dev/sdk-go` | `8a728fa429d034f813b27b91acc54b926bf13b8d` |
| Java | `github.com/unipost-dev/sdk-java` | `38e64a7cabb7f2ed3fdd97306c7fa85f1a472987` |

The Go module is publicly resolved at
`github.com/unipost-dev/sdk-go`; a `v0.7.0` tag in that repository is the Go
module release.

## Isolation requirement

Set `UNIPOST_DEV_ROOT` to an explicit release-owned directory containing
`sdk-js`, `sdk-python`, `sdk-go`, and `sdk-java`. Never point a release at a
shared checkout or another task's worktree.

```bash
export UNIPOST_DEV_ROOT=/absolute/path/to/release-owned-sdk-root
```

Every repository must be on `main`, clean, and contain the merged account-read
and Billing feature source. `create-sdk-release.sh` fails before a version bump
if any required public symbol is absent or if a target `v0.7.0` tag already
exists.

## Required acceptance inputs

The release gate requires:

- `UNIPOST_API_KEY`: Workspace API key for the target environment.
- `BASE_URL`: exact API environment, such as `https://dev-api.unipost.dev`.
- `TEST_X_ACCOUNT_ID`: connected X account owned by the acceptance workspace.
- `TEST_EXTERNAL_USER_ID`: Managed User bound to that account.
- `REQUIRE_X_ACCOUNT_READ_ACCEPTANCE=true`: turns missing fixtures into a hard
  failure rather than a skip.
- `TEST_ACCOUNT_ID`: existing general SDK fixture, when available.

The X fixture must have `users.read`, `tweet.read`, and `offline.access`, and
must preserve the X app identity used when it was connected. Validation performs
one live profile read, one minimum-size authored-post read, exact idempotent
replays, an optional cursor continuation, and the Credits snapshot/events path.
For a dedicated regression workspace, the source-validation runner may resolve
these two IDs from `GET /v1/accounts` only when exactly one eligible X account is
available (or `TEST_ACCOUNT_ID` identifies it). Zero or multiple candidates remain
a hard failure; release jobs should set both values explicitly for deterministic
acceptance.

When `x_credits_billing_v1` is off, account reads must remain available with
`meta.credits.accounting_enabled=false`, while the dedicated Billing endpoints
return `FEATURE_NOT_AVAILABLE`. When the flag is on, the same reads and Billing
inspection paths must succeed. Run acceptance in both states before release.

## Feature PR gate

For `0.7.0`, first push the four SDK feature branches and open pull requests to
their respective `main` branches. Do not bump versions in those feature PRs.

Run the main repository's `SDK Source Validation` workflow with:

- `sdk_js_ref`
- `sdk_python_ref`
- `sdk_go_ref`
- `sdk_java_ref`

Set each input to the exact feature branch or SHA. The workflow records all four
resolved SHAs in its artifact. Required SDK CI is:

- JavaScript: Node 18, 20, and 22; tests, typecheck, build, and package dry-run.
- Python: Python 3.9 through 3.12; pytest, mypy, and build.
- Go: Go 1.21 through 1.23; test and vet.
- Java: Java 17 and 21; Gradle test and build.

A failed, skipped, cancelled, timed-out, or missing required result blocks the
release. Merge the feature PRs only after their own CI and exact-head central
validation pass, then verify each merged `main` SHA.

## Create the 0.7.0 release commits and tags

After all feature PRs are merged:

```bash
UNIPOST_DEV_ROOT=/absolute/path/to/release-owned-sdk-root \
UNIPOST_API_KEY=up_live_xxx \
BASE_URL=https://dev-api.unipost.dev \
TEST_ACCOUNT_ID=sa_general_fixture \
TEST_X_ACCOUNT_ID=sa_x_fixture \
TEST_EXTERNAL_USER_ID=user_fixture \
REQUIRE_X_ACCOUNT_READ_ACCEPTANCE=true \
scripts/release/create-sdk-release.sh 0.7.0
```

This command:

1. validates repository, branch, cleanliness, tag, and required feature symbols;
2. updates every maintained version-bearing file;
3. rebuilds the JavaScript `dist` package;
4. runs all four source and live validation suites;
5. creates one focused `Release v0.7.0` commit and `v0.7.0` tag per repository.

Inspect all four commits and tags before adding `--push`. The push form is:

```bash
UNIPOST_DEV_ROOT=/absolute/path/to/release-owned-sdk-root \
UNIPOST_API_KEY=up_live_xxx \
BASE_URL=https://dev-api.unipost.dev \
TEST_X_ACCOUNT_ID=sa_x_fixture \
TEST_EXTERNAL_USER_ID=user_fixture \
REQUIRE_X_ACCOUNT_READ_ACCEPTANCE=true \
scripts/release/create-sdk-release.sh 0.7.0 --push
```

## Registry publication and verification

Tags trigger the repository release workflows:

- JavaScript publishes `@unipost/sdk@0.7.0` to npm.
- Python publishes `unipost==0.7.0` to PyPI.
- Java publishes `dev.unipost:sdk-java:0.7.0` to Maven Central.
- Go resolves directly from `github.com/unipost-dev/sdk-go@v0.7.0`.

Java publishing requires the repository's Maven Central credentials and signing
secrets. JavaScript and Python publishing require their configured trusted
publisher or registry credentials.

Verify from fresh temporary consumers, not the release worktrees:

```bash
npm view @unipost/sdk@0.7.0 version
python3 -m pip index versions unipost
go list -m github.com/unipost-dev/sdk-go@v0.7.0
```

For Java, resolve `dev.unipost:sdk-java:0.7.0` from Maven Central and compile a
consumer that calls both an existing API and the new account-read/Billing APIs.
Each fresh consumer must prove the package version, User-Agent version,
representative old calls, new public methods, full response envelopes, and
structured error metadata.

## Recovery rule

Never move or overwrite a published tag or registry version. If publication
fails before an artifact becomes public, fix the source with a new commit and
rerun the workflow. If any registry already accepted the version, publish a new
patch version after all four repositories are consistent.

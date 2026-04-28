# SDK Release Guide

This guide documents the current release flow for UniPost SDKs after source changes land in this repo.

## Fast path

After one-time GitHub secret setup, the release path for JavaScript and Python can be nearly one command:

```bash
UNIPOST_API_KEY=up_live_xxx \
TEST_ACCOUNT_ID=sa_xxx \
scripts/release/create-sdk-release.sh 0.2.1 --push
```

That command:

- bumps all SDK version strings
- auto-cleans leftover version/dist changes from a previously failed release attempt
- rebuilds the JS dist bundle
- runs lightweight local validation
- runs all three source-validation suites against the updated SDK source
- creates the release commit
- creates the git tag
- pushes `main`
- pushes `v0.2.1`

Once the tags land on GitHub:

- `sdk-js` publishes through `/Users/xiaoboyu/unipost-dev/sdk-js/.github/workflows/publish.yml`
- `sdk-python` publishes through `/Users/xiaoboyu/unipost-dev/sdk-python/.github/workflows/publish.yml`

Go still needs a separate release step unless you later align the public module path with this repo.

## Current release model

- JavaScript SDK source lives in `/Users/xiaoboyu/unipost-dev/sdk-js`
- Python SDK source lives in `/Users/xiaoboyu/unipost-dev/sdk-python`
- Go SDK source lives in `/Users/xiaoboyu/unipost-dev/sdk-go`

That means:

- JavaScript can be published directly from this repo
- Python can now be packaged and published directly from this repo
- Go should be released through the repo that actually serves `github.com/unipost-dev/sdk-go`, or the module path must be changed before public release

## 1. Bump the SDK version

Update the version string in these files:

- `/Users/xiaoboyu/unipost-dev/sdk-js/package.json`
- `/Users/xiaoboyu/unipost-dev/sdk-js/src/http.ts`
- `/Users/xiaoboyu/unipost-dev/sdk-python/pyproject.toml`
- `/Users/xiaoboyu/unipost-dev/sdk-python/unipost/__init__.py`
- `/Users/xiaoboyu/unipost-dev/sdk-go/unipost/client.go`

Example target version:

- `0.2.1`

## 2. Run validation before release

From repo root:

```bash
scripts/sdk-source-validation/run-suite.sh sdk-js
scripts/sdk-source-validation/run-suite.sh sdk-python
scripts/sdk-source-validation/run-suite.sh sdk-go
```

Recommended extra safety check:

```bash
scripts/regression/run-suite.sh smoke
```

## 3. Simplest release command

If your SDK repos are either clean or only contain leftover release-state changes from a previously failed run, and your npm / PyPI secrets are already configured in GitHub, use:

```bash
UNIPOST_API_KEY=up_live_xxx \
TEST_ACCOUNT_ID=sa_xxx \
scripts/release/create-sdk-release.sh 0.2.1 --push
```

If you want to stop before pushing:

```bash
UNIPOST_API_KEY=up_live_xxx \
TEST_ACCOUNT_ID=sa_xxx \
scripts/release/create-sdk-release.sh 0.2.1
```

`UNIPOST_API_KEY` is required because the release script hard-gates on the three source-validation suites before it will commit or tag a release.

## 4. Manual release steps

If you prefer the manual path, here is the equivalent flow.

### Commit the release bump

Example:

```bash
git -C /Users/xiaoboyu/unipost-dev/sdk-js status
git -C /Users/xiaoboyu/unipost-dev/sdk-python status
git -C /Users/xiaoboyu/unipost-dev/sdk-go status
```

### Create and push the Git tag

If you are using one shared SDK version across languages, create a repo tag after the release commit:

```bash
git -C /Users/xiaoboyu/unipost-dev/sdk-js tag v0.2.1
git -C /Users/xiaoboyu/unipost-dev/sdk-python tag v0.2.1
git -C /Users/xiaoboyu/unipost-dev/sdk-go tag v0.2.1
```

This is useful for release history and is required if you later align the Go module release with Git tags.

## 5. Source validation workflow

If you want a GitHub-hosted pre-release check from the `unipost` repo, run:

- `/Users/xiaoboyu/unipost/.github/workflows/sdk-source-validation.yml`

That workflow checks out the `sdk-js`, `sdk-python`, and `sdk-go` repos into the runner, then runs the same source-validation suites against them before release.

## 6. Publish the JavaScript SDK manually

Login first if needed:

```bash
npm whoami
npm login
```

Publish:

```bash
cd /Users/xiaoboyu/unipost-dev/sdk-js
npm publish --access public
```

Verify:

```bash
npm view @unipost/sdk version
```

## 7. Publish the Python SDK manually

Install build tools if needed:

```bash
python3 -m pip install --upgrade build twine
```

Build:

```bash
cd /Users/xiaoboyu/unipost-dev/sdk-python
python3 -m build
```

Upload to PyPI:

```bash
python3 -m twine upload dist/*
```

Verify:

```bash
python3 -m pip index versions unipost
```

## 8. Release the Go SDK

The Go module currently declares:

```go
module github.com/unipost-dev/sdk-go
```

So the public Go release must happen from the repository that actually resolves at `github.com/unipost-dev/sdk-go`.

If that repository is the same repository currently checked out at `/Users/xiaoboyu/unipost-dev/sdk-go`, the release flow is:

1. Sync the latest `/sdk/go` source into the `sdk-go` repository.
2. Commit the version bump there.
3. Tag that repo:

```bash
git tag v0.2.1
git push origin v0.2.1
```

4. Verify:

```bash
go list -m github.com/unipost-dev/sdk-go@v0.2.1
```

If you do not have a separate `sdk-go` repo yet, that is the missing piece before a proper public Go release.

## Release checklist

- Version strings updated everywhere
- JS, Python, Go regression suites green
- Optional smoke suite green
- Release commit pushed
- Git tag pushed
- npm package published
- PyPI package built and uploaded
- Go module released from the correct repository

## Notes

- The Python distribution name in `pyproject.toml` is currently `unipost`.
- The import path stays:

```python
from unipost import UniPost
```

- If your existing PyPI package name is different, update the `project.name` field before publishing.

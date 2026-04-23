# SDK Validation Tests

Live API tests for the current UniPost SDK surfaces.

All three validation scripts now use the local SDK packages checked into this repo so unreleased SDK changes can be verified against the live UniPost API before publishing:
- JavaScript: `/Users/xiaoboyu/unipost/sdk/javascript`
- Go: `/Users/xiaoboyu/unipost/sdk/go`
- Python: `/Users/xiaoboyu/unipost/sdk/python`

## JavaScript / TypeScript

```bash
cd js
npm install
UNIPOST_API_KEY=up_live_xxx node unipost-sdk-test.mjs
UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> node unipost-sdk-test.mjs
```

What it covers now:
- Account list and profile-filtered account reads
- Developer webhook signature verification and webhook CRUD
- Post list, get, and queue snapshot
- Draft and scheduled post creation
- Analytics rollup

## Python

```bash
cd python
pip install -r requirements.txt
UNIPOST_API_KEY=up_live_xxx python unipost_sdk_test.py
UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> python unipost_sdk_test.py
```

## Go

```bash
cd go
go mod tidy
UNIPOST_API_KEY=up_live_xxx go run main.go
UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> go run main.go
```

## Flags

| Env var | Purpose |
|---------|---------|
| `UNIPOST_API_KEY` | Required. API key from app.unipost.dev |
| `TEST_ACCOUNT_ID` | Optional. Enables post create/schedule/cancel tests |
| `TEST_PUBLISH_NOW=true` | Optional. Actually publishes a post (irreversible) |

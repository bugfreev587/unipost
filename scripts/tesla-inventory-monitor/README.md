# Tesla Inventory Monitor

Polls a Tesla inventory URL every minute (configurable) and alerts you the
instant a target configuration — e.g. **Premium AWD 5 seats** — becomes
available.

Single file, **zero dependencies** (Python 3.8+ standard library only).

## Why you run this yourself (not from the cloud agent)

This repo's Claude Code session runs in a sandboxed cloud environment whose
network egress policy **blocks `www.tesla.com`**, and the sandbox is ephemeral
(it gets reclaimed after inactivity). So it cannot poll Tesla for you around
the clock. Run this script on a machine that can reach Tesla and stays on:
your laptop, a small VPS, or a Raspberry Pi.

## Usage

```bash
python3 monitor.py \
  --url "https://www.tesla.com/inventory/RN129022653?PaymentType=finance" \
  --keywords premium awd "5 seat" \
  --interval 60 \
  --ntfy-topic my-secret-tesla-topic-9271
```

It logs one line per check and, on a match, fires every configured channel and
keeps running (add `--stop-on-hit` to exit on the first match).

## Get the phone push working (recommended: ntfy)

The easiest "notify me immediately on my phone" path is [ntfy.sh](https://ntfy.sh):

1. Install the **ntfy** app (iOS / Android), or open <https://ntfy.sh> in a browser.
2. Subscribe to a **hard-to-guess** topic name, e.g. `tesla-rn129022653-8f3k2`.
3. Pass that same name via `--ntfy-topic tesla-rn129022653-8f3k2`.

Any match then pushes an urgent notification straight to your phone.

### Other channels

| Channel  | Flags / env vars |
|----------|------------------|
| Desktop  | auto (macOS `osascript`, Linux `notify-send`, Windows PowerShell) |
| ntfy     | `--ntfy-topic` (`NTFY_TOPIC`), `--ntfy-server` (`NTFY_SERVER`) |
| Telegram | `--telegram-token` (`TELEGRAM_TOKEN`), `--telegram-chat-id` (`TELEGRAM_CHAT_ID`) |
| Webhook  | `--webhook-url` (`WEBHOOK_URL`) — POSTs `{title,message,url}` JSON |

Every flag has an environment-variable equivalent, so you can keep secrets out
of your shell history.

## How matching works

- **All** `--keywords` must appear on the fetched page (case-insensitive).
- If any "sold out / no longer available" marker is present, it's treated as
  **not** in stock even when keywords match.
- Tune keywords to the exact trim wording Tesla uses on the listing. Start
  broad (`premium awd "5 seat"`) and tighten if you get false positives.

> Note: Tesla renders some inventory detail client-side. If keyword matching
> against raw HTML proves unreliable for this particular listing, switch to
> Tesla's inventory JSON API or a headless browser — the notification plumbing
> in `monitor.py` stays the same; only `fetch()` / `check_page()` change.

## Run it durably

```bash
# stays alive, logs to a file, survives your terminal closing
nohup python3 monitor.py --url "..." --keywords premium awd "5 seat" \
  --ntfy-topic my-secret-topic > tesla-monitor.log 2>&1 &
```

For a set-and-forget setup, wrap it in a `systemd` service or a `launchd`
agent so it restarts on reboot.

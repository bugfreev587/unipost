#!/usr/bin/env python3
"""Tesla inventory monitor.

Polls a Tesla inventory URL on a fixed interval and alerts the moment a
target configuration (e.g. "Premium AWD 5 seats") shows up as available.

Zero third-party dependencies — uses only the Python standard library, so it
runs anywhere Python 3.8+ is installed. Run it on a machine that can actually
reach www.tesla.com (a personal laptop, a small VPS, a Raspberry Pi, etc.).

Quick start:

    python3 monitor.py \
        --url "https://www.tesla.com/inventory/RN129022653?PaymentType=finance" \
        --keywords "premium" "awd" "5 seat" \
        --interval 60 \
        --ntfy-topic my-secret-tesla-topic-name

When every keyword is found on the page (and no "sold out / unavailable"
marker is present), the script fires every notification channel you have
configured and keeps running (so you get reminded until you act, unless you
pass --stop-on-hit).

Notification channels (enable any combination):
  * Terminal bell + loud stdout banner   (always on)
  * Desktop notification                 (auto-detected: notify-send / osascript / msg)
  * ntfy.sh push to your phone           (--ntfy-topic)
  * Telegram message                     (--telegram-token + --telegram-chat-id)
  * Generic webhook (POST json)          (--webhook-url)

All flags can also be supplied via environment variables — see build_config().
"""

from __future__ import annotations

import argparse
import json
import os
import platform
import subprocess
import sys
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timezone


# Markers that indicate the specific car is NOT available. If any of these are
# present we treat the listing as "not in stock" even if keywords match.
DEFAULT_UNAVAILABLE_MARKERS = [
    "no longer available",
    "this vehicle is no longer available",
    "vehicle not found",
    "sold out",
    "out of stock",
    "no results",
    "no matches",
]

BROWSER_HEADERS = {
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
        "(KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"
    ),
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "en-US,en;q=0.9",
}


@dataclass
class Config:
    url: str
    keywords: list[str]
    interval: int = 60
    unavailable_markers: list[str] = field(
        default_factory=lambda: list(DEFAULT_UNAVAILABLE_MARKERS)
    )
    stop_on_hit: bool = False
    timeout: int = 30
    ntfy_topic: str | None = None
    ntfy_server: str = "https://ntfy.sh"
    telegram_token: str | None = None
    telegram_chat_id: str | None = None
    webhook_url: str | None = None


def log(msg: str) -> None:
    stamp = datetime.now(timezone.utc).astimezone().strftime("%Y-%m-%d %H:%M:%S")
    print(f"[{stamp}] {msg}", flush=True)


def fetch(url: str, timeout: int) -> str:
    req = urllib.request.Request(url, headers=BROWSER_HEADERS)
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        charset = resp.headers.get_content_charset() or "utf-8"
        return resp.read().decode(charset, errors="replace")


def check_page(page: str, cfg: Config) -> tuple[bool, str]:
    """Return (is_hit, reason)."""
    lowered = page.lower()

    present = [k for k in cfg.keywords if k.lower() in lowered]
    missing = [k for k in cfg.keywords if k.lower() not in lowered]

    blocking = [m for m in cfg.unavailable_markers if m.lower() in lowered]

    if missing:
        return False, f"missing keywords: {missing}"
    if blocking:
        return False, f"all keywords present but blocked by: {blocking}"
    return True, f"all keywords present: {present}"


# --------------------------------------------------------------------------- #
# Notification channels
# --------------------------------------------------------------------------- #
def notify_desktop(title: str, message: str) -> None:
    system = platform.system()
    try:
        if system == "Darwin":
            script = f'display notification "{message}" with title "{title}" sound name "Glass"'
            subprocess.run(["osascript", "-e", script], check=False, timeout=10)
        elif system == "Linux":
            # notify-send is the common freedesktop tool.
            subprocess.run(
                ["notify-send", "-u", "critical", title, message],
                check=False,
                timeout=10,
            )
        elif system == "Windows":
            ps = (
                "[reflection.assembly]::loadwithpartialname('System.Windows.Forms')"
                ";[System.Windows.Forms.MessageBox]::Show("
                f"'{message}','{title}')"
            )
            subprocess.run(["powershell", "-Command", ps], check=False, timeout=15)
    except Exception as exc:  # noqa: BLE001 — notifications are best-effort
        log(f"desktop notification failed: {exc}")


def notify_ntfy(cfg: Config, title: str, message: str) -> None:
    if not cfg.ntfy_topic:
        return
    url = f"{cfg.ntfy_server.rstrip('/')}/{cfg.ntfy_topic}"
    try:
        req = urllib.request.Request(
            url,
            data=message.encode("utf-8"),
            headers={
                "Title": title,
                "Priority": "urgent",
                "Tags": "car,rotating_light",
                "Click": cfg.url,
            },
            method="POST",
        )
        urllib.request.urlopen(req, timeout=cfg.timeout).read()
        log("ntfy push sent")
    except Exception as exc:  # noqa: BLE001
        log(f"ntfy push failed: {exc}")


def notify_telegram(cfg: Config, message: str) -> None:
    if not (cfg.telegram_token and cfg.telegram_chat_id):
        return
    api = f"https://api.telegram.org/bot{cfg.telegram_token}/sendMessage"
    payload = json.dumps(
        {"chat_id": cfg.telegram_chat_id, "text": message, "disable_web_page_preview": False}
    ).encode("utf-8")
    try:
        req = urllib.request.Request(
            api, data=payload, headers={"Content-Type": "application/json"}, method="POST"
        )
        urllib.request.urlopen(req, timeout=cfg.timeout).read()
        log("telegram message sent")
    except Exception as exc:  # noqa: BLE001
        log(f"telegram message failed: {exc}")


def notify_webhook(cfg: Config, title: str, message: str) -> None:
    if not cfg.webhook_url:
        return
    payload = json.dumps({"title": title, "message": message, "url": cfg.url}).encode("utf-8")
    try:
        req = urllib.request.Request(
            cfg.webhook_url,
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        urllib.request.urlopen(req, timeout=cfg.timeout).read()
        log("webhook fired")
    except Exception as exc:  # noqa: BLE001
        log(f"webhook failed: {exc}")


def fire_alert(cfg: Config, reason: str) -> None:
    title = "🚗 Tesla inventory MATCH"
    message = f"Target config is available!\n{reason}\n{cfg.url}"

    # Terminal: bell + loud banner.
    sys.stdout.write("\a")
    print("\n" + "=" * 60)
    print("  🚨🚨🚨  MATCH FOUND — CONFIG AVAILABLE  🚨🚨🚨")
    print(f"  {reason}")
    print(f"  {cfg.url}")
    print("=" * 60 + "\n", flush=True)

    notify_desktop(title, message)
    notify_ntfy(cfg, title, message)
    notify_telegram(cfg, message)
    notify_webhook(cfg, title, message)


# --------------------------------------------------------------------------- #
# Main loop
# --------------------------------------------------------------------------- #
def run(cfg: Config) -> None:
    log("Tesla inventory monitor starting")
    log(f"URL:       {cfg.url}")
    log(f"Keywords:  {cfg.keywords}  (all must be present, case-insensitive)")
    log(f"Interval:  every {cfg.interval}s")
    channels = ["terminal", "desktop"]
    if cfg.ntfy_topic:
        channels.append(f"ntfy:{cfg.ntfy_topic}")
    if cfg.telegram_token and cfg.telegram_chat_id:
        channels.append("telegram")
    if cfg.webhook_url:
        channels.append("webhook")
    log(f"Alerts:    {', '.join(channels)}")

    consecutive_errors = 0
    while True:
        try:
            page = fetch(cfg.url, cfg.timeout)
            consecutive_errors = 0
            hit, reason = check_page(page, cfg)
            if hit:
                fire_alert(cfg, reason)
                if cfg.stop_on_hit:
                    log("stop-on-hit set — exiting")
                    return
            else:
                log(f"not yet — {reason}")
        except urllib.error.HTTPError as exc:
            consecutive_errors += 1
            log(f"HTTP error {exc.code}: {exc.reason} (streak={consecutive_errors})")
        except Exception as exc:  # noqa: BLE001 — never let one bad poll kill the loop
            consecutive_errors += 1
            log(f"fetch error: {exc} (streak={consecutive_errors})")

        # Gentle backoff if the site is erroring repeatedly, capped at 5 min.
        delay = cfg.interval
        if consecutive_errors >= 3:
            delay = min(cfg.interval * consecutive_errors, 300)
            log(f"backing off to {delay}s after repeated errors")

        try:
            time.sleep(delay)
        except KeyboardInterrupt:
            log("interrupted — exiting")
            return


def build_config() -> Config:
    p = argparse.ArgumentParser(description="Monitor a Tesla inventory URL for a target config.")
    p.add_argument(
        "--url",
        default=os.environ.get(
            "TESLA_URL",
            "https://www.tesla.com/inventory/RN129022653?PaymentType=finance",
        ),
    )
    p.add_argument(
        "--keywords",
        nargs="+",
        default=(os.environ.get("TESLA_KEYWORDS", "premium awd 5 seat").split()),
        help="All keywords must appear on the page (case-insensitive).",
    )
    p.add_argument("--interval", type=int, default=int(os.environ.get("TESLA_INTERVAL", "60")))
    p.add_argument("--timeout", type=int, default=int(os.environ.get("TESLA_TIMEOUT", "30")))
    p.add_argument("--stop-on-hit", action="store_true", default=bool(os.environ.get("TESLA_STOP_ON_HIT")))
    p.add_argument("--ntfy-topic", default=os.environ.get("NTFY_TOPIC"))
    p.add_argument("--ntfy-server", default=os.environ.get("NTFY_SERVER", "https://ntfy.sh"))
    p.add_argument("--telegram-token", default=os.environ.get("TELEGRAM_TOKEN"))
    p.add_argument("--telegram-chat-id", default=os.environ.get("TELEGRAM_CHAT_ID"))
    p.add_argument("--webhook-url", default=os.environ.get("WEBHOOK_URL"))
    args = p.parse_args()

    return Config(
        url=args.url,
        keywords=args.keywords,
        interval=max(5, args.interval),
        stop_on_hit=args.stop_on_hit,
        timeout=args.timeout,
        ntfy_topic=args.ntfy_topic,
        ntfy_server=args.ntfy_server,
        telegram_token=args.telegram_token,
        telegram_chat_id=args.telegram_chat_id,
        webhook_url=args.webhook_url,
    )


if __name__ == "__main__":
    run(build_config())

#!/usr/bin/env python3

import argparse
import json
import sys
from typing import Any


def fail(message: str) -> None:
    print(message, file=sys.stderr)
    raise SystemExit(64)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Resolve one unambiguous connected X account from a UniPost accounts response."
    )
    parser.add_argument("--account-id", default="")
    parser.add_argument("--external-user-id", default="")
    parser.add_argument("--preferred-account-id", default="")
    return parser.parse_args()


def eligible_accounts(payload: Any) -> list[dict[str, Any]]:
    if not isinstance(payload, dict) or not isinstance(payload.get("data"), list):
        fail("account discovery response must contain a data array")

    return [
        account
        for account in payload["data"]
        if isinstance(account, dict)
        and str(account.get("platform", "")).lower() == "twitter"
        and isinstance(account.get("id"), str)
        and bool(account["id"].strip())
        and isinstance(account.get("external_user_id"), str)
        and bool(account["external_user_id"].strip())
    ]


def main() -> None:
    args = parse_args()
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        fail(f"account discovery returned invalid JSON: {exc}")

    candidates = eligible_accounts(payload)

    if args.account_id:
        candidates = [account for account in candidates if account["id"] == args.account_id]
    if args.external_user_id:
        candidates = [
            account
            for account in candidates
            if account["external_user_id"] == args.external_user_id
        ]

    if not args.account_id and not args.external_user_id and args.preferred_account_id:
        preferred = [
            account for account in candidates if account["id"] == args.preferred_account_id
        ]
        if len(preferred) == 1:
            candidates = preferred

    if len(candidates) != 1:
        fail(
            f"found {len(candidates)} eligible connected X accounts; "
            "set TEST_X_ACCOUNT_ID and TEST_EXTERNAL_USER_ID explicitly"
        )

    account = candidates[0]
    print(f"{account['id']}\t{account['external_user_id']}")


if __name__ == "__main__":
    main()

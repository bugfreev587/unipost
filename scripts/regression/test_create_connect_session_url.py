#!/usr/bin/env python3

import importlib.util
import io
from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "create_connect_session_url.py"


def load_script_module():
    spec = importlib.util.spec_from_file_location("create_connect_session_url", SCRIPT)
    if spec is None or spec.loader is None:
        raise AssertionError(f"could not load {SCRIPT}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class CreateConnectSessionURLTest(unittest.TestCase):
    def test_posts_connect_session_to_production_by_default_and_prints_url(self):
        module = load_script_module()
        calls = []

        def fake_request(method, url, headers, payload):
            calls.append(
                {
                    "method": method,
                    "url": url,
                    "headers": headers,
                    "payload": payload,
                }
            )
            return {
                "data": {
                    "id": "cs_test_123",
                    "platform": "tiktok",
                    "status": "pending",
                    "url": "https://app.unipost.dev/connect/tiktok?session=cs_test_123&state=state_test",
                    "expires_at": "2026-06-17T20:00:00Z",
                }
            }

        stdout = io.StringIO()
        stderr = io.StringIO()
        code = module.main(
            [
                "--platform",
                "tiktok",
                "--profile-id",
                "pr_test_123",
                "--external-user-id",
                "local-user-123",
                "--external-user-email",
                "alex@example.com",
                "--return-url",
                "https://example.com/oauth/done",
                "--allow-quickstart-creds",
            ],
            environ={"UNIPOST_API_KEY": "sk_test_123"},
            request_json=fake_request,
            stdout=stdout,
            stderr=stderr,
        )

        self.assertEqual(code, 0, stderr.getvalue())
        self.assertEqual(len(calls), 1)
        self.assertEqual(calls[0]["method"], "POST")
        self.assertEqual(calls[0]["url"], "https://api.unipost.dev/v1/connect/sessions")
        self.assertEqual(calls[0]["headers"]["Authorization"], "Bearer sk_test_123")
        self.assertEqual(calls[0]["headers"]["Content-Type"], "application/json")
        self.assertEqual(
            calls[0]["payload"],
            {
                "platform": "tiktok",
                "profile_id": "pr_test_123",
                "external_user_id": "local-user-123",
                "external_user_email": "alex@example.com",
                "return_url": "https://example.com/oauth/done",
                "allow_quickstart_creds": True,
            },
        )
        self.assertIn("Connection session URL:", stdout.getvalue())
        self.assertIn("https://app.unipost.dev/connect/tiktok?session=cs_test_123&state=state_test", stdout.getvalue())
        self.assertIn("Copy this URL into your browser", stdout.getvalue())

    def test_missing_api_key_returns_usage_error_without_network_call(self):
        module = load_script_module()
        calls = []
        stdout = io.StringIO()
        stderr = io.StringIO()

        code = module.main(
            ["--platform", "linkedin"],
            environ={},
            request_json=lambda *args: calls.append(args),
            stdout=stdout,
            stderr=stderr,
        )

        self.assertEqual(code, 2)
        self.assertEqual(calls, [])
        self.assertEqual(stdout.getvalue(), "")
        self.assertIn("Missing UNIPOST_API_KEY", stderr.getvalue())

    def test_accepts_short_profile_and_platform_env_aliases(self):
        module = load_script_module()
        calls = []

        def fake_request(method, url, headers, payload):
            calls.append(payload)
            return {
                "data": {
                    "id": "cs_env_alias",
                    "platform": "youtube",
                    "status": "pending",
                    "url": "https://app.unipost.dev/connect/youtube?session=cs_env_alias&state=state_test",
                }
            }

        code = module.main(
            ["--external-user-id", "local-user-456"],
            environ={
                "UNIPOST_API_KEY": "sk_test_456",
                "PLATFORM": "youtube",
                "PROFILE_ID": "16202f3f-0c3c-4b92-afae-177f279c692a",
            },
            request_json=fake_request,
            stdout=io.StringIO(),
            stderr=io.StringIO(),
        )

        self.assertEqual(code, 0)
        self.assertEqual(
            calls,
            [
                {
                    "platform": "youtube",
                    "external_user_id": "local-user-456",
                    "profile_id": "16202f3f-0c3c-4b92-afae-177f279c692a",
                }
            ],
        )


if __name__ == "__main__":
    unittest.main()

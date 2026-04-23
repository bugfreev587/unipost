import hashlib
import hmac
import json
import os
from dataclasses import dataclass
from typing import Any, Dict, List, Optional

import requests


class UniPostError(Exception):
    def __init__(self, message: str, status: int = 0, code: str = ""):
        super().__init__(message)
        self.status = status
        self.code = code


class _Object:
    def __init__(self, data: Dict[str, Any]):
        for key, value in data.items():
            setattr(self, key, _wrap(value))

    def __repr__(self) -> str:
        return f"_Object({self.__dict__!r})"


def _wrap(value: Any) -> Any:
    if isinstance(value, dict):
        return _Object(value)
    if isinstance(value, list):
        return [_wrap(item) for item in value]
    return value


class _HttpClient:
    def __init__(self, api_key: str, base_url: str, timeout: int = 30):
        if not api_key:
            raise UniPostError("UniPost API key is required")
        self.api_key = api_key
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout

    def request(self, method: str, path: str, *, params=None, json_body=None, headers=None):
        response = requests.request(
            method,
            f"{self.base_url}{path}",
            params=params,
            json=json_body,
            headers={
                "Authorization": f"Bearer {self.api_key}",
                "User-Agent": "unipost-python/0.2.0-local",
                **(headers or {}),
            },
            timeout=self.timeout,
        )
        if not response.ok:
            try:
                payload = response.json()
            except Exception:
                payload = {}
            error = payload.get("error", {})
            raise UniPostError(error.get("message", f"HTTP {response.status_code}"), response.status_code, error.get("code", ""))
        if response.status_code == 204 or not response.text:
            return {}
        return response.json()


class AccountsAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def list(self, **kwargs):
        params = {}
        if "platform" in kwargs:
            params["platform"] = kwargs["platform"]
        if "external_user_id" in kwargs:
            params["external_user_id"] = kwargs["external_user_id"]
        if "status" in kwargs:
            params["status"] = kwargs["status"]
        if "profile_id" in kwargs:
            params["profile_id"] = kwargs["profile_id"]
        payload = self._http.request("GET", "/v1/social-accounts", params=params)
        payload["data"] = [_wrap(item) for item in payload.get("data", [])]
        return payload


class PostsAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def list(self, **kwargs):
        payload = self._http.request("GET", "/v1/social-posts", params=kwargs)
        payload["data"] = [_wrap(item) for item in payload.get("data", [])]
        return payload

    def get(self, post_id: str):
        payload = self._http.request("GET", f"/v1/social-posts/{post_id}")
        return _wrap(payload["data"])

    def get_queue(self, post_id: str):
        payload = self._http.request("GET", f"/v1/social-posts/{post_id}/queue")
        return _wrap(payload["data"])

    def create(self, **kwargs):
        body = {}
        mapping = {
            "caption": "caption",
            "account_ids": "account_ids",
            "media_urls": "media_urls",
            "media_ids": "media_ids",
            "scheduled_at": "scheduled_at",
            "status": "status",
            "platform_posts": "platform_posts",
        }
        for source, target in mapping.items():
            if source in kwargs and kwargs[source] is not None:
                body[target] = kwargs[source]
        headers = {}
        if kwargs.get("idempotency_key"):
            headers["Idempotency-Key"] = kwargs["idempotency_key"]
        payload = self._http.request("POST", "/v1/social-posts", json_body=body, headers=headers)
        return _wrap(payload["data"])

    def cancel(self, post_id: str):
        payload = self._http.request("POST", f"/v1/social-posts/{post_id}/cancel")
        return _wrap(payload["data"])


class AnalyticsAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def rollup(self, **kwargs):
        params = {
            "from": kwargs.get("from_date") or kwargs.get("from"),
            "to": kwargs.get("to_date") or kwargs.get("to"),
            "granularity": kwargs.get("granularity"),
            "group_by": kwargs.get("group_by"),
        }
        payload = self._http.request("GET", "/v1/analytics/rollup", params=params)
        return _wrap(payload["data"])


class WebhooksAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def create(self, *, url: str, events: List[str]):
        payload = self._http.request("POST", "/v1/webhooks", json_body={"url": url, "events": events})
        return _wrap(payload["data"])

    def list(self):
        payload = self._http.request("GET", "/v1/webhooks")
        payload["data"] = [_wrap(item) for item in payload.get("data", [])]
        return payload

    def get(self, webhook_id: str):
        payload = self._http.request("GET", f"/v1/webhooks/{webhook_id}")
        return _wrap(payload["data"])

    def update(self, webhook_id: str, **kwargs):
        body = {}
        for key in ("url", "events", "active"):
            if key in kwargs and kwargs[key] is not None:
                body[key] = kwargs[key]
        payload = self._http.request("PATCH", f"/v1/webhooks/{webhook_id}", json_body=body)
        return _wrap(payload["data"])

    def rotate(self, webhook_id: str):
        payload = self._http.request("POST", f"/v1/webhooks/{webhook_id}/rotate")
        return _wrap(payload["data"])

    def delete(self, webhook_id: str):
        payload = self._http.request("DELETE", f"/v1/webhooks/{webhook_id}")
        return payload.get("data", payload)


class UniPost:
    def __init__(self, api_key: Optional[str] = None, base_url: Optional[str] = None):
        http = _HttpClient(api_key or os.environ.get("UNIPOST_API_KEY", ""), base_url or "https://api.unipost.dev")
        self.accounts = AccountsAPI(http)
        self.posts = PostsAPI(http)
        self.analytics = AnalyticsAPI(http)
        self.webhooks = WebhooksAPI(http)


def verify_webhook_signature(payload: bytes, signature: str, secret: str) -> bool:
    normalized = (signature or "").strip().lower()
    if normalized.startswith("sha256="):
        normalized = normalized[len("sha256="):]
    expected = hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, normalized)

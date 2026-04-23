import hashlib
import hmac
import os
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


def _compact(data: Optional[Dict[str, Any]]) -> Dict[str, Any]:
    if not data:
        return {}
    return {key: value for key, value in data.items() if value is not None}


def _snake_post_body(kwargs: Dict[str, Any]) -> Dict[str, Any]:
    body: Dict[str, Any] = {}
    mapping = {
        "caption": "caption",
        "account_ids": "account_ids",
        "media_urls": "media_urls",
        "media_ids": "media_ids",
        "scheduled_at": "scheduled_at",
        "status": "status",
        "archived": "archived",
    }
    for source, target in mapping.items():
        if source in kwargs and kwargs[source] is not None:
            body[target] = kwargs[source]
    if kwargs.get("platform_posts") is not None:
        body["platform_posts"] = kwargs["platform_posts"]
    return body


def _wrap_payload_data(payload: Dict[str, Any]) -> Dict[str, Any]:
    payload["data"] = _wrap(payload.get("data"))
    return payload


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
            raise UniPostError(
                error.get("message", f"HTTP {response.status_code}"),
                response.status_code,
                error.get("normalized_code") or error.get("code", ""),
            )
        if response.status_code == 204 or not response.text:
            return {}
        return response.json()


class WorkspaceAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def get(self):
        payload = self._http.request("GET", "/v1/workspace")
        return _wrap(payload["data"])

    def update(self, *, per_account_monthly_limit: Optional[int] = None):
        payload = self._http.request(
            "PATCH",
            "/v1/workspace",
            json_body=_compact({"per_account_monthly_limit": per_account_monthly_limit}),
        )
        return _wrap(payload["data"])


class ProfilesAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def list(self):
        payload = self._http.request("GET", "/v1/profiles")
        payload["data"] = [_wrap(item) for item in payload.get("data", [])]
        return payload

    def get(self, profile_id: str):
        payload = self._http.request("GET", f"/v1/profiles/{profile_id}")
        return _wrap(payload["data"])

    def update(self, profile_id: str, **kwargs):
        payload = self._http.request(
            "PATCH",
            f"/v1/profiles/{profile_id}",
            json_body=_compact(
                {
                    "name": kwargs.get("name"),
                    "branding_logo_url": kwargs.get("branding_logo_url"),
                    "branding_display_name": kwargs.get("branding_display_name"),
                    "branding_primary_color": kwargs.get("branding_primary_color"),
                }
            ),
        )
        return _wrap(payload["data"])


class AccountsAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def list(self, **kwargs):
        payload = self._http.request(
            "GET",
            "/v1/social-accounts",
            params=_compact(
                {
                    "platform": kwargs.get("platform"),
                    "external_user_id": kwargs.get("external_user_id"),
                    "status": kwargs.get("status"),
                    "profile_id": kwargs.get("profile_id"),
                }
            ),
        )
        payload["data"] = [_wrap(item) for item in payload.get("data", [])]
        return payload

    def get(self, account_id: str):
        payload = self.list()
        for account in payload.get("data", []):
            if getattr(account, "id", None) == account_id:
                return account
        raise UniPostError("Account not found", 404, "not_found")

    def connect(self, *, platform: str, credentials: Dict[str, str]):
        payload = self._http.request(
            "POST",
            "/v1/social-accounts/connect",
            json_body={"platform": platform, "credentials": credentials},
        )
        return _wrap(payload["data"])

    def disconnect(self, account_id: str):
        payload = self._http.request("DELETE", f"/v1/social-accounts/{account_id}")
        return _wrap(payload.get("data", payload))

    def capabilities(self, account_id: str):
        payload = self._http.request("GET", f"/v1/social-accounts/{account_id}/capabilities")
        return _wrap(payload["data"])

    def health(self, account_id: str):
        payload = self._http.request("GET", f"/v1/social-accounts/{account_id}/health")
        return _wrap(payload["data"])

    def tiktok_creator_info(self, account_id: str):
        payload = self._http.request("GET", f"/v1/social-accounts/{account_id}/tiktok/creator-info")
        return _wrap(payload["data"])

    def facebook_page_insights(self, account_id: str):
        payload = self._http.request("GET", f"/v1/social-accounts/{account_id}/facebook/page-insights")
        return _wrap(payload["data"])


class PlatformsAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def capabilities(self):
        payload = self._http.request("GET", "/v1/platforms/capabilities")
        return _wrap(payload["data"])


class PlansAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def list(self):
        payload = self._http.request("GET", "/v1/plans")
        return _wrap(payload["data"])


class PlatformCredentialsAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def create(self, workspace_id: str, *, platform: str, client_id: str, client_secret: str):
        payload = self._http.request(
            "POST",
            f"/v1/workspaces/{workspace_id}/platform-credentials",
            json_body={
                "platform": platform,
                "client_id": client_id,
                "client_secret": client_secret,
            },
        )
        return _wrap(payload["data"])

    def list(self, workspace_id: str):
        payload = self._http.request("GET", f"/v1/workspaces/{workspace_id}/platform-credentials")
        payload["data"] = [_wrap(item) for item in payload.get("data", [])]
        return payload

    def delete(self, workspace_id: str, platform: str):
        payload = self._http.request("DELETE", f"/v1/workspaces/{workspace_id}/platform-credentials/{platform}")
        return _wrap(payload.get("data", payload))


class PostsAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def list(self, **kwargs):
        payload = self._http.request("GET", "/v1/social-posts", params=_compact(kwargs))
        payload["data"] = [_wrap(item) for item in payload.get("data", [])]
        meta = payload.get("meta") or {}
        payload["next_cursor"] = meta.get("next_cursor") or payload.get("next_cursor")
        return payload

    def get(self, post_id: str):
        payload = self._http.request("GET", f"/v1/social-posts/{post_id}")
        return _wrap(payload["data"])

    def get_queue(self, post_id: str):
        payload = self._http.request("GET", f"/v1/social-posts/{post_id}/queue")
        return _wrap(payload["data"])

    def analytics(self, post_id: str, *, refresh: Optional[bool] = None):
        payload = self._http.request(
            "GET",
            f"/v1/social-posts/{post_id}/analytics",
            params=_compact({"refresh": refresh}),
        )
        return _wrap(payload["data"])

    def create(self, **kwargs):
        headers = {}
        if kwargs.get("idempotency_key"):
            headers["Idempotency-Key"] = kwargs["idempotency_key"]
        payload = self._http.request(
            "POST",
            "/v1/social-posts",
            json_body=_snake_post_body(kwargs),
            headers=headers,
        )
        return _wrap(payload["data"])

    def validate(self, **kwargs):
        payload = self._http.request("POST", "/v1/social-posts/validate", json_body=_snake_post_body(kwargs))
        return _wrap(payload["data"])

    def publish(self, post_id: str):
        payload = self._http.request("POST", f"/v1/social-posts/{post_id}/publish")
        return _wrap(payload["data"])

    def update(self, post_id: str, **kwargs):
        payload = self._http.request("PATCH", f"/v1/social-posts/{post_id}", json_body=_snake_post_body(kwargs))
        return _wrap(payload["data"])

    def archive(self, post_id: str):
        payload = self._http.request("POST", f"/v1/social-posts/{post_id}/archive")
        return _wrap(payload["data"])

    def restore(self, post_id: str):
        payload = self._http.request("POST", f"/v1/social-posts/{post_id}/restore")
        return _wrap(payload["data"])

    def cancel(self, post_id: str):
        payload = self._http.request("POST", f"/v1/social-posts/{post_id}/cancel")
        return _wrap(payload["data"])

    def delete(self, post_id: str):
        payload = self._http.request("DELETE", f"/v1/social-posts/{post_id}")
        return _wrap(payload.get("data", payload))

    def preview_link(self, post_id: str):
        payload = self._http.request("POST", f"/v1/social-posts/{post_id}/preview-link")
        return _wrap(payload["data"])

    def retry_result(self, post_id: str, result_id: str):
        payload = self._http.request("POST", f"/v1/social-posts/{post_id}/results/{result_id}/retry")
        return _wrap(payload["data"])

    def bulk_create(self, posts: List[Dict[str, Any]]):
        payload = self._http.request(
            "POST",
            "/v1/social-posts/bulk",
            json_body={"posts": [_snake_post_body(post) for post in posts]},
        )
        return _wrap(payload["data"])


class DeliveryJobsAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def list(self, **kwargs):
        states = kwargs.get("states")
        if isinstance(states, list):
            states = ",".join(states)
        payload = self._http.request(
            "GET",
            "/v1/post-delivery-jobs",
            params=_compact(
                {
                    "limit": kwargs.get("limit"),
                    "offset": kwargs.get("offset"),
                    "states": states,
                }
            ),
        )
        payload["data"] = [_wrap(item) for item in payload.get("data", [])]
        return payload

    def summary(self):
        payload = self._http.request("GET", "/v1/post-delivery-jobs/summary")
        return _wrap(payload["data"])

    def retry(self, job_id: str):
        payload = self._http.request("POST", f"/v1/post-delivery-jobs/{job_id}/retry")
        return _wrap(payload["data"])

    def cancel(self, job_id: str):
        payload = self._http.request("POST", f"/v1/post-delivery-jobs/{job_id}/cancel")
        return _wrap(payload["data"])


class MediaAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def upload(self, *, filename: str, content_type: str, size_bytes: int, content_hash: Optional[str] = None):
        payload = self._http.request(
            "POST",
            "/v1/media",
            json_body=_compact(
                {
                    "filename": filename,
                    "content_type": content_type,
                    "size_bytes": size_bytes,
                    "content_hash": content_hash,
                }
            ),
        )
        return _wrap(payload["data"])

    def get(self, media_id: str):
        payload = self._http.request("GET", f"/v1/media/{media_id}")
        return _wrap(payload["data"])

    def delete(self, media_id: str):
        payload = self._http.request("DELETE", f"/v1/media/{media_id}")
        return _wrap(payload.get("data", payload))


class AnalyticsAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def summary(self, **kwargs):
        payload = self._http.request(
            "GET",
            "/v1/analytics/summary",
            params=_compact(
                {
                    "from": kwargs.get("from_date") or kwargs.get("from"),
                    "to": kwargs.get("to_date") or kwargs.get("to"),
                    "platform": kwargs.get("platform"),
                    "status": kwargs.get("status"),
                }
            ),
        )
        return _wrap(payload["data"])

    def trend(self, **kwargs):
        payload = self._http.request(
            "GET",
            "/v1/analytics/trend",
            params=_compact(
                {
                    "from": kwargs.get("from_date") or kwargs.get("from"),
                    "to": kwargs.get("to_date") or kwargs.get("to"),
                    "platform": kwargs.get("platform"),
                    "status": kwargs.get("status"),
                }
            ),
        )
        return _wrap(payload["data"])

    def by_platform(self, **kwargs):
        payload = self._http.request(
            "GET",
            "/v1/analytics/by-platform",
            params=_compact(
                {
                    "from": kwargs.get("from_date") or kwargs.get("from"),
                    "to": kwargs.get("to_date") or kwargs.get("to"),
                    "platform": kwargs.get("platform"),
                    "status": kwargs.get("status"),
                }
            ),
        )
        return _wrap(payload["data"])

    def rollup(self, **kwargs):
        payload = self._http.request(
            "GET",
            "/v1/analytics/rollup",
            params=_compact(
                {
                    "from": kwargs.get("from_date") or kwargs.get("from"),
                    "to": kwargs.get("to_date") or kwargs.get("to"),
                    "granularity": kwargs.get("granularity"),
                    "group_by": kwargs.get("group_by") or kwargs.get("groupBy"),
                }
            ),
        )
        return _wrap(payload["data"])


class ConnectAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def create_session(self, *, platform: str, external_user_id: str, profile_id: Optional[str] = None,
                       external_user_email: Optional[str] = None, return_url: Optional[str] = None):
        payload = self._http.request(
            "POST",
            "/v1/connect/sessions",
            json_body=_compact(
                {
                    "platform": platform,
                    "profile_id": profile_id,
                    "external_user_id": external_user_id,
                    "external_user_email": external_user_email,
                    "return_url": return_url,
                }
            ),
        )
        return _wrap(payload["data"])

    def get_session(self, session_id: str):
        payload = self._http.request("GET", f"/v1/connect/sessions/{session_id}")
        return _wrap(payload["data"])


class UsersAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def list(self):
        payload = self._http.request("GET", "/v1/users")
        payload["data"] = [_wrap(item) for item in payload.get("data", [])]
        return payload

    def get(self, external_user_id: str):
        payload = self._http.request("GET", f"/v1/users/{external_user_id}")
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
        payload = self._http.request(
            "PATCH",
            f"/v1/webhooks/{webhook_id}",
            json_body=_compact(
                {
                    "url": kwargs.get("url"),
                    "events": kwargs.get("events"),
                    "active": kwargs.get("active"),
                }
            ),
        )
        return _wrap(payload["data"])

    def rotate(self, webhook_id: str):
        payload = self._http.request("POST", f"/v1/webhooks/{webhook_id}/rotate")
        return _wrap(payload["data"])

    def delete(self, webhook_id: str):
        payload = self._http.request("DELETE", f"/v1/webhooks/{webhook_id}")
        return _wrap(payload.get("data", payload))


class OAuthAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def connect(self, platform: str, *, redirect_url: Optional[str] = None):
        payload = self._http.request(
            "GET",
            f"/v1/oauth/connect/{platform}",
            params=_compact({"redirect_url": redirect_url}),
        )
        return _wrap(payload["data"])


class UsageAPI:
    def __init__(self, http: _HttpClient):
        self._http = http

    def get(self):
        payload = self._http.request("GET", "/v1/usage")
        return _wrap(payload["data"])


class UniPost:
    def __init__(self, api_key: Optional[str] = None, base_url: Optional[str] = None):
        http = _HttpClient(api_key or os.environ.get("UNIPOST_API_KEY", ""), base_url or "https://api.unipost.dev")
        self.workspace = WorkspaceAPI(http)
        self.profiles = ProfilesAPI(http)
        self.accounts = AccountsAPI(http)
        self.platforms = PlatformsAPI(http)
        self.plans = PlansAPI(http)
        self.platform_credentials = PlatformCredentialsAPI(http)
        self.posts = PostsAPI(http)
        self.delivery_jobs = DeliveryJobsAPI(http)
        self.media = MediaAPI(http)
        self.analytics = AnalyticsAPI(http)
        self.connect = ConnectAPI(http)
        self.users = UsersAPI(http)
        self.webhooks = WebhooksAPI(http)
        self.oauth = OAuthAPI(http)
        self.usage = UsageAPI(http)


def verify_webhook_signature(payload: bytes, signature: str, secret: str) -> bool:
    normalized = (signature or "").strip().lower()
    if normalized.startswith("sha256="):
        normalized = normalized[len("sha256="):]
    expected = hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, normalized)

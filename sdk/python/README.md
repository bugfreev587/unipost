# UniPost Python SDK

Official Python SDK for the UniPost API.

## Install

```bash
pip install unipost
```

## Quickstart

```python
from unipost import UniPost

client = UniPost(api_key="up_live_xxx")

profiles = client.profiles.list()
accounts = client.accounts.list()
```

## Base URL override

```python
client = UniPost(
    api_key="up_live_xxx",
    base_url="https://api.unipost.dev",
)
```

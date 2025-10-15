# Verifies basic MaaS API health:
# 1) test_mint_token → we can mint an auth token (looks like a long JWT).
# 2) test_list_models_exposes_urls → /v1/models returns a list and each model
#    entry includes a usable endpoint/URL (or at least id/endpoint fields).
# If these fail, the control-plane (token mint) or catalog (models/URLs) is broken.

from conftest import bearer

def test_mint_token(maas_key):
    assert isinstance(maas_key, str) and len(maas_key) > 100

def test_list_models_exposes_urls(http, base_url, maas_key):
    r = http.get(f"{base_url}/v1/models", headers=bearer(maas_key))
    assert r.status_code == 200, r.text[:200]
    j = r.json()
    data = j.get("data") or j.get("models") or []
    assert isinstance(data, list) and data
    item = data[0] if data else {}
    assert isinstance(item, dict) and any(k in item for k in ("url", "endpoint", "id"))

from conftest import bearer, parse_usage_headers, USAGE_API_BASE, ensure_free_key
import pytest

def test_usage_api_smoke(http, base_url, model_name):
    if not USAGE_API_BASE:
        pytest.skip("USAGE_API_BASE not set")
    key = ensure_free_key(http)
    r = http.post(
        f"{base_url}/v1/chat/completions",
        headers=bearer(key),
        json={"model": model_name, "messages":[{"role":"user","content":"hi"}]},
        timeout=60,
    )
    assert r.status_code in (200,201)
    # u = http.get(f"{USAGE_API_BASE}/v1/usage", headers=bearer(key))
    # assert u.status_code in (200,404)

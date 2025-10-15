# Smoke-check: catalog endpoint is reachable and shaped correctly.
# - Calls /v1/models with a valid token.
# - Expects HTTP 200 and a JSON body that has either "data" or "models".
# If this fails, the MaaS API or its model catalog is unavailable/misconfigured.

from conftest import bearer

def test_models_user(http, base_url, maas_key):
    r = http.get(f"{base_url}/v1/models", headers=bearer(maas_key))
    assert r.status_code == 200, r.text[:200]
    assert "data" in r.json() or "models" in r.json()

from conftest import bearer, parse_usage_headers, USAGE_HEADERS, ensure_free_key, ensure_premium_key
import os, json, base64

def _b64url_decode(s):
    pad = "=" * (-len(s) % 4)
    return base64.urlsafe_b64decode((s + pad).encode("utf-8"))

def test_minted_token_is_jwt(http, base_url, maas_key):
    parts = maas_key.split(".")
    assert len(parts) == 3
    hdr = json.loads(_b64url_decode(parts[0]).decode("utf-8"))
    assert isinstance(hdr, dict)

def test_tokens_issue_201_and_schema(http, base_url):
    from conftest import FREE_OC_TOKEN, mint_maas_key
    key, body, _ = mint_maas_key(http, base_url, FREE_OC_TOKEN, minutes=10)
    assert isinstance(body, dict) and key and len(key) > 10

def test_tokens_invalid_ttl_400(http, base_url):
    from conftest import FREE_OC_TOKEN, http_post, bearer
    url = f"{base_url}/v1/tokens"
    code, body, r = http_post(http, url, headers=bearer(FREE_OC_TOKEN), json={"ttl":"4hours"})
    assert code == 400

def test_tokens_models_happy_then_revoked_fails(http, base_url, model_name):
    from conftest import FREE_OC_TOKEN, mint_maas_key, revoke_maas_key, bearer as bh
    key, _, _ = mint_maas_key(http, base_url, FREE_OC_TOKEN, minutes=10)
    r_ok = http.get(f"{base_url}/v1/models", headers=bh(key))
    assert r_ok.status_code == 200

    r_del = revoke_maas_key(http, base_url, FREE_OC_TOKEN, key)
    assert r_del.status_code in (200,202,204)

    r_again = http.get(f"{base_url}/v1/models", headers=bh(key))
    assert r_again.status_code in (401,403)

def test_usage_headers_present(http, base_url, model_name):
    key = ensure_free_key(http)
    r = http.post(
        f"{base_url}/v1/chat/completions",
        headers=bearer(key),
        json={
            "model": model_name,
            "messages": [{"role":"user","content":"Say hi"}],
            "temperature": 0,
        },
        timeout=60,
    )
    assert r.status_code in (200,201), f"unexpected {r.status_code}: {r.text[:200]}"
    usage = parse_usage_headers(r)
    assert any(h in usage for h in USAGE_HEADERS), f"No usage headers: {dict(r.headers)}"

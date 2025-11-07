# ============================
# What this file tests (short)
# ============================
# - We can mint MaaS tokens and they look like real JWTs.
# - The minted token actually works to call /v1/models.
# - Bad TTL input is rejected with 400.
# - After we revoke a token, it stops working.
# - Model responses include usage headers (token counts).

from conftest import bearer, parse_usage_headers, USAGE_HEADERS, ensure_free_key
import json, base64

def _b64url_decode(s):
    pad = "=" * (-len(s) % 4)
    return base64.urlsafe_b64decode((s + pad).encode("utf-8"))

def test_minted_token_is_jwt(maas_key):
    parts = maas_key.split(".")
    assert len(parts) == 3
    hdr = json.loads(_b64url_decode(parts[0]).decode("utf-8"))
    assert isinstance(hdr, dict)

def test_tokens_issue_201_and_schema(http, base_url):
    from conftest import FREE_OC_TOKEN, mint_maas_key, bearer as bh
    # mint_maas_key returns a single string (the MaaS key)
    key = mint_maas_key(http, base_url, FREE_OC_TOKEN, minutes=10)
    assert isinstance(key, str) and len(key) > 10
    # prove the key works and don’t hang forever
    r_ok = http.get(f"{base_url}/v1/models", headers=bh(key), timeout=30)
    assert r_ok.status_code == 200

def test_tokens_invalid_ttl_400(http, base_url):
    from conftest import FREE_OC_TOKEN, http_post
    url = f"{base_url}/v1/tokens"
    code, body, r = http_post(
        http,
        url,
        headers=bearer(FREE_OC_TOKEN),
        json={"expiration": "4hours"},
        timeout=30,          # add timeout so it can’t hang
    )
    assert code == 400

def test_tokens_models_happy_then_revoked_fails(http, base_url, model_name):
    from conftest import FREE_OC_TOKEN, mint_maas_key, revoke_maas_key, bearer

    # 1) Mint a MaaS key from the current OC user token
    key = mint_maas_key(http, base_url, FREE_OC_TOKEN, minutes=10)

    # 2) Discover the model URL
    models = http.get(f"{base_url}/v1/models", headers=bearer(key), timeout=30).json()
    items = models.get("data") or models.get("models") or []
    target = next((m for m in items if m.get("id")==model_name or m.get("name")==model_name), None)
    assert target and target.get("url"), "model not found or missing url"
    murl = target["url"]

    payload = {"model": model_name,
               "messages":[{"role":"user","content":"hi"}],
               "max_tokens": 32}

    # 3) Works before revoke
    r_ok = http.post(f"{murl}/v1/chat/completions", headers=bearer(key), json=payload, timeout=60)
    assert r_ok.status_code in (200, 201)

    # 4) Revoke the key
    r_del = revoke_maas_key(http, base_url, FREE_OC_TOKEN, key)
    assert r_del.status_code in (200, 202, 204)

    # 5) Fails after revoke
    r_bad = http.post(f"{murl}/v1/chat/completions", headers=bearer(key), json=payload, timeout=60)
    assert r_bad.status_code in (401, 403)

def test_usage_headers_present(http, base_url, model_name):
    from conftest import bearer, ensure_free_key, parse_usage_headers

    key = ensure_free_key(http)

    # discover model URL
    models = http.get(f"{base_url}/v1/models", headers=bearer(key), timeout=30).json()
    items = models.get("data") or models.get("models") or []
    target = next((m for m in items if m.get("id")==model_name or m.get("name")==model_name), None)
    assert target and target.get("url"), "model not found or missing url"
    murl = target["url"]

    r = http.post(
        f"{murl}/v1/chat/completions",
        headers=bearer(key),
        json={"model": model_name, "messages":[{"role":"user","content":"Say hi"}], "temperature":0},
        timeout=60,
    )
    assert r.status_code in (200, 201), f"unexpected {r.status_code}: {r.text[:200]}"

    usage = parse_usage_headers(r)
    # assert presence and non-negative total
    assert "x-odhu-usage-total-tokens" in usage, f"No usage headers present: {dict(r.headers)}"
    assert int(usage["x-odhu-usage-total-tokens"]) >= 0

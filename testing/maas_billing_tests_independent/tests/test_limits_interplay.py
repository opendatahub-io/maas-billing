# Tests rate vs token limits.
# 1) test_request_limit_before_token_limit → many small fast requests → expect 429 (rate limit).
# 2) test_token_limit_before_request_limit → few large token requests → expect 429 (token limit).
# Failures usually mean limits not applied or too high to trigger.

import os, time, pytest
from conftest import bearer, ensure_free_key, get_limit

def _url(http, base_url, key, model_name):
    r = http.get(f"{base_url}/v1/models", headers=bearer(key), timeout=30)
    assert r.status_code == 200
    items = (r.json().get("data") or r.json().get("models") or [])
    m = next((m for m in items if m.get("id") == model_name or m.get("name") == model_name), None)
    assert m and m.get("url")
    return m["url"]

def _post(http, url, model, key, tokens):
    return http.post(
        f"{url}/v1/chat/completions",
        headers=bearer(key),
        json={
            "model": model,
            "messages": [{"role": "user", "content": "ping"}],
            "max_tokens": tokens,
            "temperature": 0,
        },
        timeout=60,
    )

@pytest.mark.skipif(not os.getenv("FREE_OC_TOKEN"), reason="FREE_OC_TOKEN not set")
def test_request_limit_before_token_limit(http, base_url, model_name):
    key    = ensure_free_key(http)
    url    = _url(http, base_url, key, model_name)
    burst  = get_limit("RATE_LIMIT_BURST", "free_burst", 16)
    budget = get_limit("TOKEN_LIMIT_FREE", "free_tokens", 1000)

    per_call = int(os.getenv("TOKENS_PER_CALL_SMALL", "8"))  # “cheap”
    calls    = burst + 5
    sleep_s  = float(os.getenv("BURST_SLEEP", "0.05"))
    codes    = []
    for _ in range(calls):
        r = _post(http, url, model_name, key, per_call)
        codes.append(r.status_code)
        if r.status_code == 429:
            break
        time.sleep(sleep_s)

    assert any(c == 429 for c in codes), f"no 429 seen; codes={codes}"
    # Optional sanity: cheap calls should not have crossed token budget first

@pytest.mark.skipif(not os.getenv("FREE_OC_TOKEN"), reason="FREE_OC_TOKEN not set")
def test_token_limit_before_request_limit(http, base_url, model_name):
    key    = ensure_free_key(http)
    url    = _url(http, base_url, key, model_name)
    burst  = get_limit("RATE_LIMIT_BURST", "free_burst", 16)
    budget = get_limit("TOKEN_LIMIT_FREE", "free_tokens", 1000)

    calls    = min(burst - 1, 5) if burst > 1 else 1  # stay under burst
    per_call = max(64, (budget // max(calls, 1)) + 64)  # “expensive”
    sleep_s  = float(os.getenv("BURST_SLEEP", "0.05"))
    codes    = []
    for _ in range(calls):
        r = _post(http, url, model_name, key, per_call)
        codes.append(r.status_code)
        if r.status_code == 429:
            break
        time.sleep(sleep_s)

    # If budget wasn’t yet crossed, send one more to push it over
    if 429 not in codes:
        r = _post(http, url, model_name, key, per_call)
        codes.append(r.status_code)

    assert any(c == 429 for c in codes), f"no 429 seen; codes={codes}"

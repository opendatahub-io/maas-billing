# Validates Free-tier request *rate* limiting works:
# - Sends N quick /v1/chat/completions
# - Expects at least one 429
# - If burst is known, expects >= burst successes before 429s

import os, time
from conftest import bearer, ensure_free_key, get_limit

def test_rate_limit_burst(http, base_url, model_name):
    key = ensure_free_key(http)

    # Discover configured burst if available
    burst = get_limit("RATE_LIMIT_BURST_FREE", "free_burst", None)

    # Discover the model URL once
    models = http.get(f"{base_url}/v1/models", headers=bearer(key), timeout=30)
    assert models.status_code == 200, f"/v1/models failed: {models.status_code} {models.text[:200]}"
    body = models.json()
    items = body.get("data") or body.get("models") or []
    target = next((m for m in items if m.get("id") == model_name or m.get("name") == model_name), None)
    assert target and target.get("url"), f"model {model_name!r} not found or missing url"

    # Use the catalog's model URL directly
    model_url = target["url"]

    # Choose N: just above burst if known, else a safe default
    N = (burst + 5) if burst is not None else int(os.getenv("GLOBAL_BURST_N", "25"))

    per_call_tokens = int(os.getenv("TOKENS_PER_CALL_SMALL", "16"))
    sleep_s = float(os.getenv("BURST_SLEEP", "0.05"))

    codes = []
    for _ in range(N):
        r = http.post(
            f"{model_url}/v1/chat/completions",
            headers=bearer(key),
            json={"model": model_name, "messages": [{"role": "user", "content": "hi"}],
                  "max_tokens": per_call_tokens, "temperature": 0},
            timeout=60,
        )
        codes.append(r.status_code)
        time.sleep(sleep_s)

    ok = sum(c in (200, 201) for c in codes)
    rl = sum(c == 429 for c in codes)

    assert rl >= 1, f"expected at least one 429 after burst; codes={codes}"
    if burst is not None:
        assert ok >= burst, f"expected >= {burst} successes before limiting; got {ok}, codes={codes}"

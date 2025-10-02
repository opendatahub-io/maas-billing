import os, time
from conftest import bearer, ensure_free_key, get_limit

def test_rate_limit_burst(http, base_url, model_name):
    """
    Sends N quick chat completions as a Free user to exercise the request-rate limiter.
    - N is chosen to exceed the discovered (or default) burst.
    - We *always* assert that we saw at least one 429 (limiting happened).
    - If we know the burst, we also assert we saw at least that many 2xx before 429s.
    """
    key = ensure_free_key(http)

    # Try to discover the free-tier burst (env -> RLP -> fallback None)
    burst = get_limit("RATE_LIMIT_BURST_FREE", "free_burst", None)

    # Discover the model URL once
    models = http.get(f"{base_url}/v1/models", headers=bearer(key), timeout=30)
    assert models.status_code == 200, f"/v1/models failed: {models.status_code} {models.text[:200]}"
    body = models.json()
    items = body.get("data") or body.get("models") or []
    target = next((m for m in items if m.get("id") == model_name or m.get("name") == model_name), None)
    assert target and target.get("url"), f"model {model_name!r} not found or missing url"
    model_url = target["url"]

    # Choose N: if we know burst, go just above it; else use a safe default (e.g., 25)
    N = (burst + 5) if burst is not None else int(os.getenv("GLOBAL_BURST_N", "25"))

    # Keep per-call tokens small so token-rate limiter doesn't interfere
    per_call_tokens = int(os.getenv("TOKENS_PER_CALL_SMALL", "16"))
    sleep_s = float(os.getenv("BURST_SLEEP", "0.05"))

    codes = []
    for _ in range(N):
        r = http.post(
            f"{model_url}/v1/chat/completions",
            headers=bearer(key),
            json={
                "model": model_name,
                "messages": [{"role": "user", "content": "hi"}],
                "max_tokens": per_call_tokens,
                "temperature": 0,
            },
            timeout=60,
        )
        codes.append(r.status_code)
        time.sleep(sleep_s)

    ok = sum(c in (200, 201) for c in codes)
    rl = sum(c == 429 for c in codes)  # request-rate limiter responses

    # Always: limiting must have happened at least once
    assert rl >= 1, f"expected at least one 429 after burst; codes={codes}"

    # If we *know* the burst, also assert we got >= burst successes
    if burst is not None:
        assert ok >= burst, f"expected >= {burst} successes before limiting; got {ok}, codes={codes}"

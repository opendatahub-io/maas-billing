"""
Purpose: Compare Free vs Premium *request-rate* behavior for the same model.
Result: Premium should be >= Free (not more restricted).

We do not require any env tuning:
- We'll try to discover bursts from the cluster. If unavailable, we simply pick N large
  enough by default so both tiers should hit the limiter at least once.

Example:
- Free burst = 8, Premium burst = 16
- N = burst(max) + 5 requests per user
  Free: first 8 → 200, later → 429  => free_ok = 8
  Premium: first 16 → 200, later → 429 => prem_ok = 16
- Assertion: prem_ok >= free_ok  (Premium is not worse than Free)

If both tiers allow ≥ N, you'll see free_ok = prem_ok = N → PASS (still correct).
If Premium were lower than Free, prem_ok < free_ok → FAIL.
"""

import os, pytest, time
from conftest import bearer, ensure_free_key, ensure_premium_key, get_limit

@pytest.mark.skipif(not os.getenv("PREMIUM_OC_TOKEN"), reason="PREMIUM_OC_TOKEN not set")
def test_free_vs_premium_quota(http, base_url, model_name):
    free_key = ensure_free_key(http)
    prem_key = ensure_premium_key(http)

    # Discover the model URL once (either key works)
    models = http.get(f"{base_url}/v1/models", headers=bearer(free_key), timeout=30).json()
    items = models.get("data") or models.get("models") or []
    target = next((m for m in items if m.get("id") == model_name or m.get("name") == model_name), None)
    assert target and target.get("url"), f"model {model_name!r} not found or missing 'url'"
    model_url = target["url"]

    # Bursts (env -> RLP -> None)
    free_burst = get_limit("RATE_LIMIT_BURST_FREE", "free_burst", None)
    prem_burst = get_limit("RATE_LIMIT_BURST_PREMIUM", "premium_burst", None)

    # Pick N slightly above the larger known burst; if unknown, default to 25
    if free_burst is not None or prem_burst is not None:
        N_default = max(free_burst or 0, prem_burst or 0) + 5
    else:
        N_default = 25
    N = int(os.getenv("N_BURST", str(N_default)))

    # Keep calls "cheap" so token-rate limiter does not trip here
    per_call_tokens = int(os.getenv("TOKENS_PER_CALL_SMALL", "16"))
    sleep_s        = float(os.getenv("BURST_SLEEP", "0.05"))

    def run(key):
        ok = 0
        rl = 0
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
            if r.status_code in (200, 201):
                ok += 1
            elif r.status_code == 429:
                rl += 1
            time.sleep(sleep_s)
        return ok, rl

    free_ok, free_rl = run(free_key)
    prem_ok, prem_rl = run(prem_key)

    # Both tiers should hit limiter at least once if N is large enough
    assert free_rl >= 1 or prem_rl >= 1, (
        f"no 429 seen; increase N_BURST (now {N}) or check rate-limits. "
        f"free_ok={free_ok}, free_rl={free_rl}, prem_ok={prem_ok}, prem_rl={prem_rl}"
    )
    # Core expectation: premium is not worse than free
    assert prem_ok >= free_ok, f"premium_ok={prem_ok} < free_ok={free_ok}"

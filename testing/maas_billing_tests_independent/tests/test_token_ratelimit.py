# Verifies the *token* budget limiter for the Free tier.
# Flow:
# 1) Discover model URL from /v1/models.
# 2) Send a few “expensive” chat/completions calls (large max_tokens) so we
#    stay under the request-rate burst but exceed the Free token budget.
# 3) Expect to see a 429 once the cumulative tokens cross the budget.
# Notes:
# - Reads token budget/burst from cluster (with env overrides).
# - If usage headers aren’t exposed, the test skips (can’t measure tokens).

import os, time, pytest
from conftest import bearer, ensure_free_key, get_limit

USAGE_HEADERS = (
    "x-odhu-usage-total-tokens",
    "x-odhu-usage-input-tokens",
    "x-odhu-usage-output-tokens",
)

def _model_url(http, base_url, key, model_name):
    r = http.get(f"{base_url}/v1/models", headers=bearer(key), timeout=30)
    assert r.status_code == 200, f"/v1/models failed: {r.status_code} {r.text[:200]}"
    items = (r.json().get("data") or r.json().get("models") or [])
    hit = next((m for m in items if m.get("id") == model_name or m.get("name") == model_name), None)
    assert hit and hit.get("url"), f"model {model_name!r} not found or missing url"
    return hit["url"]

def _tokens_used(h):
    tot = h.get("x-odhu-usage-total-tokens")
    if tot:
        try:
            return int(tot)
        except:
            pass
    try:
        return int(h.get("x-odhu-usage-input-tokens") or 0) + int(h.get("x-odhu-usage-output-tokens") or 0)
    except:
        return 0

@pytest.mark.skipif(not os.getenv("FREE_OC_TOKEN"), reason="FREE_OC_TOKEN not set")
def test_free_token_budget_enforced(http, base_url, model_name):
    key = ensure_free_key(http)
    url = _model_url(http, base_url, key, model_name)

    # Pull limits from cluster, allow env override
    token_budget = get_limit("TOKEN_LIMIT_FREE", "free_tokens", 1000)
    burst        = get_limit("RATE_LIMIT_BURST", "free_burst", 16)

    # Shape traffic so we stay under request burst but exceed token budget
    calls    = min(burst - 1, 5) if burst > 1 else 1
    per_call = max(64, (token_budget // max(calls, 1)) + 64)  # “expensive” calls
    sleep_s  = float(os.getenv("BURST_SLEEP", "0.05"))

    consumed, codes = 0, []
    for _ in range(calls):
        r = http.post(
            f"{url}/v1/chat/completions",
            headers=bearer(key),
            json={
                "model": model_name,
                "messages": [{"role": "user", "content": "Repeat X 500 times."}],
                "max_tokens": per_call,
                "temperature": 0,
            },
            timeout=60,
        )
        codes.append(r.status_code)
        if r.status_code in (200, 201):
            if not any(h in r.headers for h in USAGE_HEADERS):
                pytest.skip("Usage headers not present; token accounting disabled on this cluster.")
            consumed += _tokens_used(r.headers)
            if consumed >= token_budget:
                # Fire one extra to observe 429 due to token limit
                time.sleep(sleep_s)
                r2 = http.post(
                    f"{url}/v1/chat/completions",
                    headers=bearer(key),
                    json={
                        "model": model_name,
                        "messages": [{"role": "user", "content": "Repeat X 500 times."}],
                        "max_tokens": per_call,
                        "temperature": 0,
                    },
                    timeout=60,
                )
                codes.append(r2.status_code)
                break
        else:
            break
        time.sleep(sleep_s)

    assert any(c == 429 for c in codes), (
        f"never hit token limiter; budget={token_budget}, consumed={consumed}, codes={codes}"
    )

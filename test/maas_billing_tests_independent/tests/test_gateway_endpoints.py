# Test: Chat completion works end-to-end via the Gateway
# 1) GET {base_url}/v1/models with a MaaS token -> expect 200
# 2) Find the target model and its direct "url" from the payload
# 3) POST {model_url}/v1/chat/completions (NOT under /maas-api)
#    -> expect 200/201 and a JSON body with "choices" or "output"

import os
import time
from conftest import bearer  # via_gateway removed

def test_chat_completion_works(http, base_url, model_name, maas_key):
    # 1) Model catalog
    models_resp = http.get(
        f"{base_url}/v1/models",
        headers=bearer(maas_key),
        timeout=30,
    )
    assert models_resp.status_code == 200, (
        f"models list failed: {models_resp.status_code} {models_resp.text[:200]}"
    )

    body = models_resp.json()
    items = body.get("data") or body.get("models") or []

    # 2) Find our model
    target = next(
        (m for m in items if m.get("id") == model_name or m.get("name") == model_name),
        None,
    )
    assert target, f"model {model_name!r} not found in /v1/models payload"

    # Use the catalog's model URL directly (no rewrite)
    model_url = target["url"]

    payload = {
        "model": model_name,
        "messages": [{"role": "user", "content": "hello"}],
        "temperature": 0,
    }

    # 3) Call chat/completions (allow a single retry if the window is still hot)
    r = http.post(f"{model_url}/v1/chat/completions",
                  headers=bearer(maas_key), json=payload, timeout=60)
    if r.status_code == 429:
        time.sleep(float(os.getenv("RATE_WINDOW_WAIT", "3")))
        r = http.post(f"{model_url}/v1/chat/completions",
                      headers=bearer(maas_key), json=payload, timeout=60)

    assert r.status_code in (200, 201), f"{r.status_code} {r.text[:200]}"
    j = r.json()
    assert ("choices" in j and j["choices"]) or ("output" in j), f"unexpected response: {j}"

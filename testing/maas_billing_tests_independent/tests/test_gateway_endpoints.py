from conftest import bearer

def test_chat_completion_works(http, base_url, model_name, maas_key):
    # 1) Get the model catalog from MaaS API
    models_resp = http.get(
        f"{base_url}/v1/models",
        headers=bearer(maas_key),
        timeout=30,
    )
    assert models_resp.status_code == 200, \
        f"models list failed: {models_resp.status_code} {models_resp.text[:200]}"

    body = models_resp.json()
    # Some deployments use "data", others "models" — handle both
    items = body.get("data") or body.get("models") or []

    # Find the entry for our target model
    target = next(
        (m for m in items if m.get("id") == model_name or m.get("name") == model_name),
        None,
    )
    assert target, f"model {model_name!r} not found in /v1/models payload"

    model_url = target.get("url")
    assert model_url, f"model {model_name!r} missing 'url' in /v1/models payload"

    # 2) Invoke chat/completions at the model route (NOT under /maas-api)
    r = http.post(
        f"{model_url}/v1/chat/completions",
        headers=bearer(maas_key),
        json={
            "model": model_name,
            "messages": [{"role": "user", "content": "hello"}],
            "temperature": 0,
        },
        timeout=60,
    )

    # Original assertions (kept): HTTP success + JSON shape
    assert r.status_code in (200, 201), f"{r.status_code} {r.text[:200]}"
    j = r.json()
    assert ("choices" in j and j["choices"]) or ("output" in j), f"unexpected response: {j}"

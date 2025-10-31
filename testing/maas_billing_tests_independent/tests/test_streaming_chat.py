import os, json, shlex, subprocess, pytest

# Enable extra logs in report/console when STREAMING_DEBUG=1|true|yes
DEBUG = os.getenv("STREAMING_DEBUG", "false").lower() in ("1", "true", "yes")

@pytest.mark.skipif(
    os.getenv("STREAMING_ENABLED", "false").lower() not in ("1", "true", "yes"),
    reason="Streaming test not enabled for this environment",
)
def test_chat_completions_streaming(http, base_url, model_name):
    # 1) Mint MaaS token
    oc_token = subprocess.check_output(shlex.split("oc whoami -t"), text=True).strip()
    r = http.post(
        f"{base_url}/v1/tokens",
        headers={"Authorization": f"Bearer {oc_token}", "Content-Type": "application/json"},
        json={"expiration": "20m"},
        timeout=30,
    )
    assert r.status_code in (200, 201), "Token mint failed"
    token = r.json()["token"]

    # 2) Discover model URL  ← make sure everything below is indented inside the function
    r = http.get(f"{base_url}/v1/models", headers={"Authorization": f"Bearer {token}"}, timeout=30)
    assert r.status_code == 200, "Models list failed"
    models = r.json()["data"]
    model_entry = next((m for m in models if m["id"] == model_name), None)
    assert model_entry, f"Model {model_name} not found in /v1/models"

    chat_url = (model_entry.get("url") or "").rstrip("/")
    assert chat_url, "Model entry has no 'url'"
    if not chat_url.endswith("/v1/chat/completions"):
        chat_url = f"{chat_url}/v1/chat/completions"
    if DEBUG:
        print(f"[streaming] chat_url={chat_url}")

    # 3) Streaming request payload
    payload = {
        "model": model_name,
        "messages": [
            {"role": "system", "content": "You are concise."},
            {"role": "user", "content": "Say one short sentence about Texas Weather (just a sentence)."},
        ],
        "stream": True,
        "max_tokens": 20,
        "temperature": 0,
    }

    r = http.post(
        chat_url,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
        json=payload,
        timeout=90,
        stream=True,
    )
    # 4) Validate SSE and reconstruct text
    assert r.status_code == 200, "Streaming call failed"
    ctype = (r.headers.get("content-type") or r.headers.get("Content-Type") or "").lower()
    assert "text/event-stream" in ctype, f"Unexpected Content-Type: {ctype}"
    if DEBUG:
        print(f"[streaming] content-type={ctype}")

    saw_done = False
    saw_json = False
    chunks: list[str] = []

    for raw in r.iter_lines(decode_unicode=True):
        if not raw:
            continue
        if DEBUG:
            print(f"[sse] {raw}")
        if not raw.startswith("data:"):
            continue

        data = raw[5:].strip()
        if data == "[DONE]":
            saw_done = True
            break

        try:
            j = json.loads(data)
            saw_json = True
            ch = (j.get("choices") or [{}])[0]
            # OpenAI-style SSE frames: choices[].delta.{content|role}
            delta = ch.get("delta") or (ch.get("message") if "message" in ch else {}) or {}
            piece = delta.get("content") or ""
            if isinstance(piece, str) and piece:
                chunks.append(piece)
                if DEBUG:
                    print(f"[chunk {len(chunks):02d}] {piece!r}")
        except Exception as e:
            if DEBUG:
                print(f"[sse] JSON parse error: {e}")

    text = "".join(chunks).strip()
    if DEBUG:
        print("\n[streaming] ✅ Stream completed" if saw_done else "\n[streaming] ⚠️ No [DONE] seen")
        print(f"[streaming] ✅ Received {len(chunks)} chunks")
        print(f"[streaming] ✅ Final text: {text!r}\n")

    assert saw_json, "No JSON streaming frames received."
    assert saw_done, "Missing SSE terminator [DONE]."
    assert text, "Reconstructed streamed content is empty."
    assert len(text) >= 5, f"Streamed content too short: '{text}'"

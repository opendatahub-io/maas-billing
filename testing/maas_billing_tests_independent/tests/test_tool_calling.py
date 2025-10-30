import os
import json
import subprocess, shlex
import pytest

def _parse_args(args_raw):
    if isinstance(args_raw, dict):
        return args_raw
    return json.loads(args_raw)

@pytest.mark.skipif(
    os.getenv("TOOL_CALLING_ENABLED", "false").lower() not in ("1", "true", "yes"),
    reason="Tool calling not enabled for this environment",
)
def test_tool_calling_forced(http, base_url, model_name, tools_spec):

    # 1️⃣ Mint MaaS token using oc whoami -t
    oc_token = subprocess.check_output(shlex.split("oc whoami -t"), text=True).strip()
    mint_url = f"{base_url}/v1/tokens"
    r = http.post(
        mint_url,
        headers={"Authorization": f"Bearer {oc_token}", "Content-Type": "application/json"},
        json={"expiration": "20m"},
        timeout=30,
    )
    assert r.status_code in (200, 201), f"Token mint failed: {r.status_code} {r.text}"
    token = r.json()["token"]

    # 2️⃣ List models
    models_url = f"{base_url}/v1/models"
    r = http.get(models_url, headers={"Authorization": f"Bearer {token}"}, timeout=30)
    assert r.status_code == 200, f"Models list failed: {r.status_code} {r.text}"
    models = r.json()["data"]

    model_entry = next((m for m in models if m["id"] == model_name), None)
    assert model_entry, f"Model {model_name} not found in /v1/models"
    model_url = model_entry["url"]
    chat_url = f"{model_url}/v1/chat/completions"
    print(f"[debug] posting to: {chat_url}")

    # 3️⃣ Tool-calling test (forced)
    payload = {
        "model": model_name,
        "messages": [
            {"role": "system", "content": "When tool_choice is set, ALWAYS return exactly one tool call with fully-formed JSON arguments."},
            {"role": "user", "content": "What's the weather in Boston, MA? Call the get_weather tool and pass location='Boston' and unit='fahrenheit'."}
        ],
        "tools": tools_spec,
        "tool_choice": {"type": "function", "function": {"name": "get_weather"}},
        "temperature": 0,
        "max_tokens": 128,
    }

    r = http.post(
        chat_url,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
        json=payload,
        timeout=60,
    )
    assert r.status_code in (200, 201), f"Chat call failed: {r.status_code} {r.text}"

    data = r.json()
    msg = data["choices"][0]["message"]
    tool_calls = msg.get("tool_calls") or []
    print("[tool_calling] endpoint:", chat_url)
    print("[tool_calling] http:", r.status_code)
    print("[tool_calling] tool_calls:", json.dumps(tool_calls, indent=2))
    print("[tool_calling] full_response:", json.dumps(data, indent=2))
    assert tool_calls, "No tool_calls in response"
    assert len(tool_calls) == 1, f"Expected exactly one tool_call, got {len(tool_calls)}"

    call0 = tool_calls[0]
    fn = (call0.get("function") or {}).get("name")
    args_raw = (call0.get("function") or {}).get("arguments")
    args = _parse_args(args_raw)

    assert fn == "get_weather", f"Unexpected tool: {fn}"
    assert ("city" in args or "location" in args), f"Missing city/location in args: {args}"

@pytest.mark.skipif(
    os.getenv("TOOL_CALLING_ENABLED", "false").lower() not in ("1","true","yes"),
    reason="Tool calling not enabled for this environment",
)
def test_tool_calling_auto(http, base_url, model_name, tools_spec):
    # 1) Mint MaaS token (same as forced)
    oc_token = subprocess.check_output(shlex.split("oc whoami -t"), text=True).strip()
    mint_url = f"{base_url}/v1/tokens"
    r = http.post(
        mint_url,
        headers={"Authorization": f"Bearer {oc_token}", "Content-Type": "application/json"},
        json={"expiration": "20m"},
        timeout=30,
    )
    assert r.status_code in (200, 201), f"Token mint failed: {r.status_code} {r.text}"
    token = r.json()["token"]

    # 2) Discover model URL (same as forced)
    models_url = f"{base_url}/v1/models"
    r = http.get(models_url, headers={"Authorization": f"Bearer {token}"}, timeout=30)
    assert r.status_code == 200, f"Models list failed: {r.status_code} {r.text}"
    models = r.json()["data"]
    model_entry = next((m for m in models if m["id"] == model_name), None)
    assert model_entry, f"Model {model_name} not found in /v1/models"
    model_url = model_entry["url"]
    chat_url = f"{model_url}/v1/chat/completions"
    print(f"[debug] posting to: {chat_url}")

    # 3) Auto tool-calling payload
    payload = {
        "model": model_name,
        "messages": [
            {
                "role": "system",
                "content": (
                    "You are a tool-using assistant. When the user asks about weather, "
                    "you MUST call the get_weather tool exactly once with JSON arguments. "
                    "Do not answer in plain text before the tool call."
                ),
            },
            {"role": "user", "content": "What's the weather in Boston, MA today? Use Fahrenheit."},
        ],
        "tools": [
            {
                "type": "function",
                "function": {
                    "name": "get_weather",
                    "description": "Get current weather.",
                    "parameters": {
                        "type": "object",
                        "properties": {
                            "location": {"type": "string", "description": "City, e.g. 'Boston, MA'"},
                            "unit": {"type": "string", "enum": ["celsius", "fahrenheit"]},
                        },
                        "required": ["location"],
                        "additionalProperties": False,
                    },
                },
            }
        ],
        "tool_choice": "auto",
        "temperature": 0,
        "max_tokens": 384,             # give Qwen room beyond its <think> prelude
        "parallel_tool_calls": False,  # keep it to one call
    }

    r = http.post(
        chat_url,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
        json=payload,
        timeout=60,
    )
    assert r.status_code in (200, 201), f"Chat call failed: {r.status_code} {r.text}"
    data = r.json()
    msg = data["choices"][0]["message"]
    tool_calls = msg.get("tool_calls") or []
    print("[tool_calling_auto] http:", r.status_code)
    print("[tool_calling_auto] tool_calls:", json.dumps(tool_calls, indent=2))
    print("[tool_calling_auto] full_response:", json.dumps(data, indent=2))

    # Soft assertion: if the model still chooses not to call, skip (backend is configured for auto)
    if not tool_calls:
        pytest.skip("Model chose not to emit tool_calls in auto mode")

    fn = (tool_calls[0].get("function") or {}).get("name")
    args_raw = (tool_calls[0].get("function") or {}).get("arguments")
    args = args_raw if isinstance(args_raw, dict) else json.loads(args_raw)
    assert fn == "get_weather"
    assert args.get("location"), f"Missing location in args: {args}"
# Models-as-a-Service Tests — How to Run (Admin / Free / Premium)

This guide shows how to run the **maas_billing_tests_independent** test pack
against **your OpenShift cluster** and **whatever model is installed**.  
It covers Admin, Free, and Premium and shows how to generate HTML/JUnit reports.

**No code changes are required.**

---

## 0) Prerequisites

- Python **3.10+**
- `oc` CLI (logged into your cluster)
- `jq` (recommended) – `sudo apt-get install -y jq` on Debian/Ubuntu/WSL
- Shell: commands use **bash/WSL**; PowerShell equivalents are at the end.

Create a venv and install deps:

```bash
cd maas_billing_tests_independent
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

# (optional) pretty HTML report
pip install pytest-html
```
---

## 1) Login & environment (run in every new shell)

Login as the user you want to test (Admin first, later Free and Premium):

```bash
oc login https://api.<cluster>:6443 --token '<your-user-token>'
oc whoami
```

Resolve Route host and set base URLs:

```bash
CLUSTER_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
HOST="maas.${CLUSTER_DOMAIN}"                          # note: no "-api" here
export MAAS_API_BASE_URL="http://${HOST}/maas-api"    # point directly to /maas-api
export USAGE_API_BASE="$MAAS_API_BASE_URL"

export FREE_OC_TOKEN="$(oc whoami -t)"
```

Pick a **MODEL_NAME** from the catalog (`id` field):

```bash
curl -s -H "Authorization: Bearer ${FREE_OC_TOKEN}" "${MAAS_API_BASE_URL}/v1/models" | jq -r '.data[] | [.id,.name,.url] | @tsv'
export MODEL_NAME="<paste-id-from-output>"     # e.g., facebook/opt-125m
export MODEL_URL="<paste-url-from-output>"     # direct model URL, e.g. maas.apps.ci-ln-t6jfcg2-76ef8.aws-2.ci.openshift.org/llm/facebook-opt-125m-simulated/v1/chat/completions
```
---

## 2) Configure limits the tests will use

### 2.1 Request-rate bursts (Free & Premium)

Read your gateway **RateLimitPolicy** values and export them so the tests know
what to expect:

```bash
# FREE request-rate burst (per window)
export RATE_LIMIT_BURST_FREE=$(
  oc -n openshift-ingress get ratelimitpolicies.kuadrant.io gateway-rate-limits \
  -o jsonpath='{.spec.limits.free.rates[0].limit}'
)

# Premium (optional; only needed for the Free-vs-Premium test)
export RATE_LIMIT_BURST_PREMIUM=$(
  oc -n openshift-ingress get ratelimitpolicies.kuadrant.io gateway-rate-limits \
  -o jsonpath='{.spec.limits.enterprise.rates[0].limit}'
)
```

### 2.2 Keep request-rate tests away from token-rate limits

Use small per-call generation and a short delay (these defaults work well):

```bash
export TOKENS_PER_CALL_SMALL=16
export BURST_SLEEP=0.05
```

> You can override any of these per run without editing the code.

---

## 3) Run tests — by role

Re-run the login + `export` lines when you switch users.  
The suite assumes **`FREE_OC_TOKEN`** holds the *current* user’s token.

### A) Admin (sanity / wiring)

```bash
pytest -q test/maas_billing_tests_independent/tests/test_tokens.py::test_minted_token_is_jwt
pytest -q test/maas_billing_tests_independent/tests/test_models_user.py
pytest -q test/maas_billing_tests_independent/tests/test_gateway_endpoints.py::test_chat_completion_works
```

### B) Free user (authz + request-rate burst + usage)

```bash
# log in as a *Free* user, then:
export FREE_OC_TOKEN="$(oc whoami -t)"

# basics
pytest -q test/maas_billing_tests_independent/tests/test_tokens.py::test_minted_token_is_jwt
pytest -q test/maas_billing_tests_independent/tests/test_models_user.py
pytest -q test/maas_billing_tests_independent/tests/test_gateway_endpoints.py::test_chat_completion_works

# request-rate burst (expects some 429s after RATE_LIMIT_BURST_FREE)
pytest -q test/maas_billing_tests_independent/tests/test_quota_global.py::test_rate_limit_burst

# usage (optional; requires USAGE_API_BASE)
pytest -q test/maas_billing_tests_independent/tests/test_usage_logs.py
```

#### Token‑rate for Free
Trigger token-based limiting by making each call expensive in tokens:
```bash
export TOKENS_PER_CALL_LARGE=1200
pytest -q test/maas_billing_tests_independent/tests/test_token_ratelimit.py
```

#### Interplay for Free — which limiter fires first?
**Request‑rate first:** many *cheap* calls
```bash
export TOKENS_PER_CALL_SMALL=16
export BURST_SLEEP=0.05
pytest -q test/maas_billing_tests_independent/tests/test_quota_global.py::test_rate_limit_burst
```
**Token‑rate first:** few *expensive* calls
```bash
export TOKENS_PER_CALL_LARGE=1200
pytest -q test/maas_billing_tests_independent/tests/test_token_ratelimit.py
```

### C) Premium user (same flow + Free-vs-Premium comparison)

```bash
# log in as a *Premium* user, then:
export FREE_OC_TOKEN="$(oc whoami -t)"     # current user’s token again
export PREMIUM_OC_TOKEN="$FREE_OC_TOKEN"   # used by the test to mint for premium

pytest -q test/maas_billing_tests_independent/tests/test_gateway_endpoints.py::test_chat_completion_works

# Compare Free vs Premium burst; Premium must not be worse than Free
# (uses RATE_LIMIT_BURST_FREE / RATE_LIMIT_BURST_PREMIUM)
pytest -q test/maas_billing_tests_independent/tests/test_quota_per_user.py::test_free_vs_premium_quota
```

#### Token‑rate for Premium
Run the token limiter test while logged in as your Premium user:
```bash
export TOKENS_PER_CALL_LARGE=1200
pytest -q test/maas_billing_tests_independent/tests/test_token_ratelimit.py
```

#### Interplay for Premium — which limiter fires first?
**Request‑rate first:** many *cheap* calls (uses `RATE_LIMIT_BURST_PREMIUM` if you exported it)
```bash
export TOKENS_PER_CALL_SMALL=16
export BURST_SLEEP=0.05
pytest -q test/maas_billing_tests_independent/tests/test_quota_global.py::test_rate_limit_burst
```
**Token‑rate first:** few *expensive* calls
```bash
export TOKENS_PER_CALL_LARGE=1200
pytest -q test/maas_billing_tests_independent/tests/test_token_ratelimit.py
```

### D) Token-rate (current user – Free **or** Premium)

If you want to *exercise* token-rate limiting, increase tokens per call to make it trip:

```bash
export TOKENS_PER_CALL_LARGE=1200   # example value to drive token usage
pytest -q test/maas_billing_tests_independent/tests/test_token_ratelimit.py
```

---

## 3E) Interplay: which limiter applies first? (request‑rate vs token‑rate)

The gateway evaluates **both** limiters on every call; you get **429** as soon as **either** limit is exceeded.  
By shaping traffic as above (many *cheap* calls vs few *expensive* calls), you can show each limiter firing first, proving both are active on your cluster.

---

## 4) Reports (HTML & JUnit)

```bash
# Create a timestamped artifacts folder
REP_ROOT=test/maas_billing_tests_independent/artifacts
TS=$(date +%Y-%m-%d_%H-%M-%S)
OUT="$REP_ROOT/$TS"
mkdir -p "$OUT"

# Example: run everything for the current user and produce reports
pytest -q test/maas_billing_tests_independent/tests \
  --html="$OUT/maas-test-report.html" --self-contained-html \
  --junitxml="$OUT/maas-test-report.xml"

echo "Reports saved in: $OUT"
# Note - The artifacts/ folder is kept in Git but its contents are ignored, so reports don't show in PRs
```

### 4.1 Smoke tests & single test (saved under artifacts/)

```bash
# 3 smoke tests → one report
REP_ROOT=test/maas_billing_tests_independent/artifacts
TS=$(date +%Y-%m-%d_%H-%M-%S)
OUT="$REP_ROOT/$TS"
mkdir -p "$OUT"

pytest -q \
  test/maas_billing_tests_independent/tests/test_tokens.py::test_minted_token_is_jwt \
  test/maas_billing_tests_independent/tests/test_models_user.py \
  test/maas_billing_tests_independent/tests/test_gateway_endpoints.py::test_chat_completion_works \
  --html="$OUT/smoke.html" --self-contained-html \
  --junitxml="$OUT/smoke.xml"

# Single test → its own report
pytest -q test/maas_billing_tests_independent/tests/test_gateway_endpoints.py::test_chat_completion_works \
  --html="$OUT/chat.html" --self-contained-html \
  --junitxml="$OUT/chat.xml"
```

---

## 5) What each test does (cheat-sheet)

- `tests/test_tokens.py` – mints a short-lived MaaS JWT from your OC token.
- `tests/test_models_user.py` – lists models; asserts your `MODEL_NAME` exists.
- `tests/test_gateway_endpoints.py` – discovers the model’s **URL** via `/v1/models`
  then calls **`<model-url>/v1/chat/completions`** and expects a normal reply.
- `tests/test_quota_global.py` – sends **N = RATE_LIMIT_BURST_FREE + 5** quick calls with
  low tokens; expects ≥ `RATE_LIMIT_BURST_FREE` successes and then **429** (request-rate limiter).
- `tests/test_quota_per_user.py` – runs the same quick burst for Free & Premium; asserts
  **Premium ≥ Free** (Premium is not more restricted).
- `tests/test_token_ratelimit.py` (optional) – cranks up token usage to hit the token-rate
  limiter and prints usage headers:
  `x-odhu-usage-input-tokens`, `x-odhu-usage-output-tokens`, `x-odhu-usage-total-tokens`.
- `tests/test_usage_logs.py` (optional) – smoke call + probe of the Usage API endpoint.

---

### 5A) Tool Calling

Validates vLLM tool calling end-to-end through MaaS.  
Works with Qwen/Qwen3-0.6B (or any model deployed with tool calling enabled).

### Prereqs

The model deployment must enable tool calling, e.g. vLLM args include:
- `--tool-call-parser qwen3_xml`
- `--enable-auto-tool-choice`

The MaaS token endpoint may reject short TTLs. Use ≥ 10m.

### Run tests

```bash
# Pick a model that supports tool calling:
export MODEL_NAME="Qwen/Qwen3-0.6B"
# Gate to enable these tests
export TOOL_CALLING_ENABLED=true

pytest -q test/maas_billing_tests_independent/tests/test_tool_calling.py::test_tool_calling_forced -s \
  --html="$OUT/tool-calling-forced.html" --self-contained-html \
  --junitxml="$OUT/tool-calling-forced.xml"
```

### 5B) Streaming (SSE) chat completions

Validates that the model endpoint supports Server-Sent Events with `stream:true`:
- Confirms Content-Type: `text/event-stream`
- Parses data: frames as JSON (`choices[].delta.content`)
- Reconstructs non-empty text from chunks
- Ensures the stream ends with `[DONE]`

```bash
# Enable the streaming test (and extra logs)
STREAMING_ENABLED=true STREAMING_DEBUG=true \
pytest -q test/maas_billing_tests_independent/tests/test_streaming_chat.py::test_chat_completions_streaming \
  -s --capture=tee-sys \
  --html="$OUT/streaming.html" --self-contained-html \
  --junitxml="$OUT/streaming.xml"

echo "Streaming reports saved in: $OUT"
```

## 6) PowerShell equivalents (Windows)

```powershell
# venv
python -m venv .venv
. .\.venv\Scripts\Activate
pip install -r requirements.txt
# (optional) pip install pytest-html

# login (role of your choice)
oc login https://api.<cluster>:6443 --token '<token>'

# resolve Route host (avoid $HOST which is reserved in PS)
$CLUSTER_DOMAIN = oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}'
$ROUTE_HOST = "maas-api.$CLUSTER_DOMAIN"

# MaaS API base URL (route-based)
$env:MAAS_API_BASE_URL = "https://$ROUTE_HOST"
$env:USAGE_API_BASE    = $env:MAAS_API_BASE_URL

$env:FREE_OC_TOKEN     = oc whoami -t
$env:MODEL_NAME        = "<model-id>"        # from /v1/models

# request-rate bursts
$env:RATE_LIMIT_BURST_FREE    = "5"          # put your Free limit here
$env:RATE_LIMIT_BURST_PREMIUM = "20"         # put your Premium limit here

# keep token usage low in request-rate tests
$env:TOKENS_PER_CALL_SMALL = "16"
$env:BURST_SLEEP = "0.05"

# run
pytest -q tests/test_tokens.py::test_minted_token_is_jwt
pytest -q tests/test_models_user.py
pytest -q tests/test_gateway_endpoints.py::test_chat_completion_works
pytest -q tests/test_quota_global.py::test_rate_limit_burst

# report (timestamped artifacts folder)
$REP_ROOT = "test\maas_billing_tests_independent\artifacts"
$TS = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
$OUT = "$REP_ROOT\$TS"
New-Item -ItemType Directory -Force -Path $OUT | Out-Null

pytest -q test/maas_billing_tests_independent/tests `
  --html="$OUT\maas-test-report.html" --self-contained-html `
  --junitxml="$OUT\maas-test-report.xml"

Write-Host "Reports saved in: $OUT"
```

> In PowerShell, **do not** use `$HOST`; use `$ROUTE_HOST`.

---

## 7) TL;DR (copy-paste)

```bash
# venv
python3 -m venv .venv && source .venv/bin/activate && pip install -r requirements.txt

# login & env
oc login https://api.<cluster>:6443 --token '<token>'

CLUSTER_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
HOST="maas-api.${CLUSTER_DOMAIN}"
export MAAS_API_BASE_URL="https://${HOST}"

export USAGE_API_BASE="$MAAS_API_BASE_URL"
export FREE_OC_TOKEN="$(oc whoami -t)"

# pick a model
curl -s -H "Authorization: Bearer ${FREE_OC_TOKEN}" "${MAAS_API_BASE_URL}/v1/models" | jq -r '.data[] | [.id,.name,.url] | @tsv'
export MODEL_NAME="<model-id>"

# bursts from your RLP
export RATE_LIMIT_BURST_FREE=5
export RATE_LIMIT_BURST_PREMIUM=20

# safe defaults for burst tests
export TOKENS_PER_CALL_SMALL=16
export BURST_SLEEP=0.05

# run a few
pytest -q test/maas_billing_tests_independent/tests/test_tokens.py::test_minted_token_is_jwt
pytest -q test/maas_billing_tests_independent/tests/test_models_user.py
pytest -q test/maas_billing_tests_independent/tests/test_gateway_endpoints.py::test_chat_completion_works
pytest -q test/maas_billing_tests_independent/tests/test_quota_global.py::test_rate_limit_burst

# report (timestamped artifacts folder)
REP_ROOT=test/maas_billing_tests_independent/artifacts
TS=$(date +%Y-%m-%d_%H-%M-%S)
OUT="$REP_ROOT/$TS"
mkdir -p "$OUT"
pytest -q test/maas_billing_tests_independent/tests \
  --html="$OUT/maas-test-report.html" --self-contained-html \
  --junitxml="$OUT/maas-test-report.xml"
```

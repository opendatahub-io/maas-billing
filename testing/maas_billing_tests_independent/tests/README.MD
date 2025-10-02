# MaaS Billing Tests — How to Run (Admin / Free / Premium)

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
cd maas_billing_tests_independent_v5_full
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

Maas API base URLs:

# Preferred: apps-domain host (TLS + correct Host header)
APPS=$(oc get ingresses.config/cluster -o jsonpath='{.spec.domain}')
export MAAS_API_BASE_URL="https://maas-api.${APPS}"
export USAGE_API_BASE="${MAAS_API_BASE_URL}"   # used by usage tests

# Fallback (commented): ELB endpoint + explicit Host header
# HOST=$(oc -n openshift-ingress get gateway openshift-ai-inference -o jsonpath='{.status.addresses[0].value}')
# curl -H "Host: maas-api.${APPS}" "http://${HOST}/v1/models"


Export the **current user’s** OpenShift token into `FREE_OC_TOKEN`
(the tests use this name for “who you are right now”):

```bash
export FREE_OC_TOKEN="$(oc whoami -t)"
```

Pick a **MODEL_NAME** from the catalog (`id` field):

```bash
curl -s -H "Authorization: Bearer ${FREE_OC_TOKEN}" "${MAAS_API_BASE_URL}/v1/models" | jq -r '.data[] | [.id,.name,.url] | @tsv'
export MODEL_NAME="<paste-id-from-output>"     # e.g., facebook-opt-125m-simulated
```

---

## 2) Configure limits the tests will use

### 2.1 Request-rate bursts (Free & Premium)

Read your gateway **RateLimitPolicy** values and export them so the tests know
what to expect:

```bash
# Free (update the jsonpath if your CR layout differs)
export RATE_LIMIT_BURST_FREE=$(
  oc -n openshift-ingress get ratelimitpolicies.gateway.networking.k8s.io gateway-rate-limits \
  -o jsonpath='{.spec.limits.free.rates[0].limit}'
)

# Premium (optional; only needed for the Free-vs-Premium test)
export RATE_LIMIT_BURST_PREMIUM=$(
  oc -n openshift-ingress get ratelimitpolicies.gateway.networking.k8s.io gateway-rate-limits \
  -o jsonpath='{.spec.limits.premium.rates[0].limit}'
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
pytest -q tests/test_tokens.py::test_minted_token_is_jwt
pytest -q tests/test_models_user.py
pytest -q tests/test_gateway_endpoints.py::test_chat_completion_works
```

### B) Free user (authz + request-rate burst + usage)

```bash
# log in as a *Free* user, then:
export FREE_OC_TOKEN="$(oc whoami -t)"

# basics
pytest -q tests/test_tokens.py::test_minted_token_is_jwt
pytest -q tests/test_models_user.py
pytest -q tests/test_gateway_endpoints.py::test_chat_completion_works

# request-rate burst (expects some 429s after RATE_LIMIT_BURST_FREE)
pytest -q tests/test_quota_global.py::test_rate_limit_burst

# usage (optional; requires USAGE_API_BASE)
pytest -q tests/test_usage_logs.py
```

#### Token‑rate for Free
Trigger token-based limiting by making each call expensive in tokens:
```bash
export TOKENS_PER_CALL_LARGE=1200
pytest -q tests/test_token_ratelimit.py
```

#### Interplay for Free — which limiter fires first?
**Request‑rate first:** many *cheap* calls
```bash
export TOKENS_PER_CALL_SMALL=16
export BURST_SLEEP=0.05
pytest -q tests/test_quota_global.py::test_rate_limit_burst
```
**Token‑rate first:** few *expensive* calls
```bash
export TOKENS_PER_CALL_LARGE=1200
pytest -q tests/test_token_ratelimit.py
```

### C) Premium user (same flow + Free-vs-Premium comparison)

```bash
# log in as a *Premium* user, then:
export FREE_OC_TOKEN="$(oc whoami -t)"     # current user’s token again
export PREMIUM_OC_TOKEN="$FREE_OC_TOKEN"   # used by the test to mint for premium

pytest -q tests/test_gateway_endpoints.py::test_chat_completion_works

# Compare Free vs Premium burst; Premium must not be worse than Free
# (uses RATE_LIMIT_BURST_FREE / RATE_LIMIT_BURST_PREMIUM)
pytest -q tests/test_quota_per_user.py::test_free_vs_premium_quota
```

#### Token‑rate for Premium
Run the token limiter test while logged in as your Premium user:
```bash
export TOKENS_PER_CALL_LARGE=1200
pytest -q tests/test_token_ratelimit.py
```

#### Interplay for Premium — which limiter fires first?
**Request‑rate first:** many *cheap* calls (uses `RATE_LIMIT_BURST_PREMIUM` if you exported it)
```bash
export TOKENS_PER_CALL_SMALL=16
export BURST_SLEEP=0.05
pytest -q tests/test_quota_global.py::test_rate_limit_burst
```
**Token‑rate first:** few *expensive* calls
```bash
export TOKENS_PER_CALL_LARGE=1200
pytest -q tests/test_token_ratelimit.py
```

### D) Token-rate (current user – Free **or** Premium)

If you want to *exercise* token-rate limiting, increase tokens per call to make it trip:

```bash
export TOKENS_PER_CALL_LARGE=1200   # example value to drive token usage
pytest -q tests/test_token_ratelimit.py
```

---

## 3E) Interplay: which limiter applies first? (request‑rate vs token‑rate)

The gateway evaluates **both** limiters on every call; you get **429** as soon as **either** limit is exceeded.  
By shaping traffic as above (many *cheap* calls vs few *expensive* calls), you can show each limiter firing first, proving both are active on your cluster.

---

## 4) Reports (HTML & JUnit)

```bash
mkdir -p reports

# Example: run everything for the current user and produce reports
pytest -q \
  --html=reports/current.html --self-contained-html \
  --junitxml=reports/current.xml
```

Open `reports/current.html` in your browser.

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

## 6) Troubleshooting

- **401 Unauthorized** – ensure you exported `FREE_OC_TOKEN="$(oc whoami -t)"` in this shell.
- **404 on chat** – the test already posts to **`<model-url>/v1/chat/completions`**.
  If you edited anything, make sure you didn’t send to `/maas-api/v1/chat/completions`.
- **Burst test returns 429 too early** – your exported `RATE_LIMIT_BURST_FREE` is higher than
  the actual policy. Re-read the CR and export the real value (or lower `N_BURST` if you set it).
- **Never see 429** – increase `N_BURST` or verify the RateLimitPolicy is **Accepted/Enforced**
  in the `openshift-ingress` project.
- **WSL vs PowerShell** – they’re separate shells; log in and re-export vars in whichever one
  you use to run `pytest`.

---

## 7) PowerShell equivalents (Windows)

```powershell
# venv
python -m venv .venv
. .\.venv\Scripts\Activate
pip install -r requirements.txt
# (optional) pip install pytest-html

# login (role of your choice)
oc login https://api.<cluster>:6443 --token '<token>'

# resolve gateway (avoid $HOST, which is reserved in PS)
$GATEWAY_HOST = oc -n openshift-ingress `
  get gateway openshift-ai-inference -o jsonpath='{.status.addresses[0].value}'

# env
$env:MAAS_API_BASE_URL = "http://$GATEWAY_HOST/maas-api"
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

# report
pytest -q --html=reports\current.html --self-contained-html --junitxml=reports\current.xml
```

> In PowerShell, **do not** use `$HOST`; use `$GATEWAY_HOST`.

---

## 8) TL;DR (copy-paste)

```bash
# venv
python3 -m venv .venv && source .venv/bin/activate && pip install -r requirements.txt

# login & env
oc login https://api.<cluster>:6443 --token '<token>'
HOST=$(oc -n openshift-ingress get gateway openshift-ai-inference -o jsonpath='{.status.addresses[0].value}')
export MAAS_API_BASE_URL="http://${HOST}/maas-api"
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
pytest -q tests/test_tokens.py::test_minted_token_is_jwt
pytest -q tests/test_models_user.py
pytest -q tests/test_gateway_endpoints.py::test_chat_completion_works
pytest -q tests/test_quota_global.py::test_rate_limit_burst

# report
mkdir -p reports && pytest -q --html=reports/current.html --self-contained-html --junitxml=reports/current.xml
```

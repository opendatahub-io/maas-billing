"""
Shared test helpers/fixtures for MaaS billing tests.

What this file provides:
- http(requests.Session)       -> respects REQUESTS_VERIFY env (default: True)
- base_url                     -> from $MAAS_API_BASE_URL
- model_name                   -> from $MODEL_NAME
- bearer(token)                -> {"Authorization": f"Bearer <token>"}
- ensure_free_key/ensure_premium_key:
    Try to mint a MaaS JWT via /v1/tokens or /tokens (with/without body).
    If minting isn't available, fall back to the OC token so tests still run.
- mint_maas_key/revoke_maas_key for explicit control in tests
- parse_usage_headers()        -> reads x-odhu-usage-* headers
- get_limit(env_name, fallback_key, default_val):
    env override -> cluster CR discovery (RLP/TRLP) -> default
- Simple http_get/http_post wrappers (optional convenience)
"""

from __future__ import annotations
import os, json, base64, shutil, subprocess
import pytest, requests

# -------------------------- Env & constants --------------------------

BASE_URL       = os.getenv("MAAS_API_BASE_URL", "").rstrip("/")
MODEL_NAME     = os.getenv("MODEL_NAME")  # no misleading default
FREE_OC_TOKEN  = os.getenv("FREE_OC_TOKEN", "")
PREMIUM_OC_TOKEN = os.getenv("PREMIUM_OC_TOKEN", "")
USAGE_API_BASE = os.getenv("USAGE_API_BASE", BASE_URL)

USAGE_HEADERS = [
    "x-odhu-usage-input-tokens",
    "x-odhu-usage-output-tokens",
    "x-odhu-usage-total-tokens",
]

# -------------------------- Pytest fixtures --------------------------

@pytest.fixture(scope="session")
def base_url():
    assert BASE_URL, "MAAS_API_BASE_URL not set"
    return BASE_URL

@pytest.fixture(scope="session")
def model_name():
    return MODEL_NAME

@pytest.fixture(scope="session")
def http() -> requests.Session:
    s = requests.Session()
    # Default verify=True; allow opt-out via env
    verify_env = os.getenv("REQUESTS_VERIFY", "true").lower()
    s.verify = not (verify_env in ("0", "false", "no"))
    return s

# -------------------------- HTTP helpers -----------------------------

def http_get(http, url, headers=None, timeout=60):
    r = http.get(url, headers=headers or {}, timeout=timeout)
    try:
        body = r.json()
    except Exception:
        body = r.text
    return r.status_code, body, r

def http_post(http, url, headers=None, json=None, data=None, timeout=60):
    r = http.post(url, headers=headers or {}, json=json, data=data, timeout=timeout)
    try:
        body = r.json()
    except Exception:
        body = r.text
    return r.status_code, body, r

def bearer(tok: str) -> dict:
    return {"Authorization": f"Bearer {tok}"} if tok else {}

# -------------------------- Token mint/revoke ------------------------

def _looks_like_jwt(tok: str) -> bool:
    parts = tok.split(".")
    if len(parts) != 3:
        return False
    try:
        base64.urlsafe_b64decode(parts[0] + "===")
        base64.urlsafe_b64decode(parts[1] + "===")
        return True
    except Exception:
        return False

def _try_mint_maas_key(http: requests.Session, base_url: str, oc_user_token: str, minutes=10) -> str | None:
    """
    Try several permutations commonly seen across clusters:
    - POST /v1/tokens and /tokens
    - with bodies: {"ttl": "10m"}, {"expiration": "10m"}, {}, and no body
    Return the minted token or None if not available.
    """
    eps = ["/v1/tokens", "/tokens"]
    bodies = [{"ttl": f"{minutes}m"}, {"expiration": f"{minutes}m"}, {}, None]
    for ep in eps:
        url = f"{base_url.rstrip('/')}{ep}"
        for body in bodies:
            try:
                r = http.post(url, headers=bearer(oc_user_token), json=body, timeout=60)
            except Exception:
                continue
            if r.status_code in (200, 201):
                try:
                    j = r.json()
                    tok = j.get("token") or j.get("access_token")
                    if tok:
                        return tok
                except Exception:
                    pass
            if r.status_code in (404, 405):
                # endpoint not present or method not allowed → try next ep
                break
    return None

def mint_maas_key(http, base_url, oc_user_token, minutes=10):
    tok = _try_mint_maas_key(http, base_url, oc_user_token, minutes=minutes)
    if tok:
        return tok
    raise AssertionError("Could not mint a MaaS key with any common body/endpoint variant")

def revoke_maas_key(http, base_url, oc_user_token, token=None):
    # Some clusters revoke by calling DELETE on the token endpoint (token not always needed)
    for ep in ("/v1/tokens", "/tokens"):
        url = f"{base_url.rstrip('/')}{ep}"
        try:
            r = http.delete(url, headers=bearer(oc_user_token), timeout=60)
        except Exception:
            continue
        if r.status_code in (200, 202, 204):
            return r
    return r  # last response

def ensure_free_key(http: requests.Session) -> str:
    """
    Preferred: a minted MaaS JWT. Fallback: the OC token (if cluster accepts Bearer OC).
    """
    assert FREE_OC_TOKEN, "FREE_OC_TOKEN not set (export your current user's oc token)"
    assert BASE_URL, "MAAS_API_BASE_URL not set"
    minted = _try_mint_maas_key(http, BASE_URL, FREE_OC_TOKEN, minutes=10)
    if minted and _looks_like_jwt(minted):
        return minted
    return FREE_OC_TOKEN

def ensure_premium_key(http: requests.Session) -> str:
    """
    Preferred: a minted MaaS JWT. Fallback: the OC token (if cluster accepts Bearer OC).
    """
    assert PREMIUM_OC_TOKEN, "PREMIUM_OC_TOKEN not set (export your premium user's oc token)"
    assert BASE_URL, "MAAS_API_BASE_URL not set"
    minted = _try_mint_maas_key(http, BASE_URL, PREMIUM_OC_TOKEN, minutes=10)
    if minted and _looks_like_jwt(minted):
        return minted
    return PREMIUM_OC_TOKEN

@pytest.fixture
def maas_key(http):
    return ensure_free_key(http)

# -------------------------- Usage headers helper ---------------------

def parse_usage_headers(resp) -> dict:
    out = {}
    for h in USAGE_HEADERS:
        v = resp.headers.get(h) or resp.headers.get(h.title())
        if v is not None:
            try:
                out[h] = int(v)
            except Exception:
                out[h] = v
    return out

# -------------------------- Cluster policy discovery -----------------

def _get_json(ns, kind, name):
    try:
        out = subprocess.run(
            ["oc", "-n", ns, "get", kind, name, "-o", "json"],
            capture_output=True, text=True, check=True
        ).stdout
        return json.loads(out)
    except Exception:
        return {}

def _first_existing(ns, kinds, name):
    for k in kinds:
        d = _get_json(ns, k, name)
        if d:
            return d
    return {}

def policy_from_cluster():
    # Try both API groups for each CRD
    rlp = _first_existing(
        "openshift-ingress",
        ["ratelimitpolicies.gateway.networking.k8s.io",
         "ratelimitpolicies.kuadrant.io"],
        "gateway-rate-limits",
    )
    trlp = _first_existing(
        "maas-api",
        ["tokenratelimitpolicies.gateway.networking.k8s.io",
         "tokenratelimitpolicies.kuadrant.io"],
        "gateway-token-rate-limits",
    )
    return {
        "free_burst":     (rlp or {}).get("spec", {}).get("limits", {}).get("free", {}).get("rates", [{}])[0].get("limit"),
        "premium_burst":  (rlp or {}).get("spec", {}).get("limits", {}).get("premium", {}).get("rates", [{}])[0].get("limit"),
        "free_tokens":    (trlp or {}).get("spec", {}).get("limits", {}).get("free-user-tokens", {}).get("rates", [{}])[0].get("limit"),
        "premium_tokens": (trlp or {}).get("spec", {}).get("limits", {}).get("premium-user-tokens", {}).get("rates", [{}])[0].get("limit"),
    }

POLICY = policy_from_cluster()

def get_limit(env_name: str, fallback_key: str, default_val):
    """
    Prefer env override → then cluster policy (POLICY[fallback_key]) → default.
    Examples:
      get_limit("RATE_LIMIT_BURST_FREE", "free_burst", 16)
      get_limit("RATE_LIMIT_BURST_PREMIUM", "premium_burst", 32)
      get_limit("TOKEN_LIMIT_FREE", "free_tokens", 1000)
      get_limit("TOKEN_LIMIT_PREMIUM", "premium_tokens", 2000)
    """
    v = os.getenv(env_name)
    if v:
        try:
            return int(v)
        except Exception:
            return default_val
    v = POLICY.get(fallback_key)
    return int(v) if isinstance(v, (int, float)) else default_val

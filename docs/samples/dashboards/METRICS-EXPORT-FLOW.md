# 🔄 Metrics Export and Enrichment Flow

## Who Exports and Enriches Metrics?

### Current Flow (All Working ✅)

| Component | Role | What It Does | Status |
| --------- | ---- | ------------ | ------ |
| **TelemetryPolicy** | Configuration | Defines which labels to extract (`user`, `tier`, `model`) | ✅ Working |
| **Kuadrant Operator** | Processor | Reads TelemetryPolicy and configures the gateway | ✅ Working |
| **Envoy WasmPlugin** | Extractor | Extracts `user`, `tier`, `model` from request/response | ✅ Working |
| **Authorino** | Identity Provider | Provides `auth.identity.userid` and `auth.identity.tier` | ✅ Working |
| **Limitador** | Rate Limiter & Metrics Exporter | Receives metadata, uses for rate limiting, **exports metrics with labels** | ✅ Working |
| **Prometheus** | Metrics Collector | Scrapes metrics from Limitador | ✅ Working |
| **HAProxy** | Ingress Router | Exports latency metrics for routes | ✅ Working |

---

## Detailed Flow

### Step 1: Label Extraction (✅ Working)

**Component**: Envoy WasmPlugin (configured by TelemetryPolicy)

**What it extracts:**

- `model`: From `request.path.split("/")[2]` - ✅ Works
- `user`: From `auth.identity.userid` - ✅ Works
- `tier`: From `auth.identity.tier` - ✅ Works

**Status**: ✅ All labels extracted correctly

---

### Step 2: Metadata Transmission (✅ Working)

**Component**: Envoy WasmPlugin → Limitador

**What happens:**

- Envoy WasmPlugin sends extracted labels as **dynamic metadata** to Limitador
- Metadata is sent via gRPC to Limitador service

**Status**: ✅ Metadata is transmitted successfully

---

### Step 3: Rate Limiting (✅ Working)

**Component**: Limitador

**What happens:**

- Limitador receives the dynamic metadata (`user`, `tier`, `model`)
- Uses metadata for rate limiting decisions
- Tracks counters per `user`/`tier`/`model` combination

**Status**: ✅ Rate limiting works with custom labels

---

### Step 4: Metrics Export (✅ Working)

**Component**: Limitador → Prometheus

**What happens:**

- Limitador exports metrics WITH custom labels: `authorized_hits{user="...", tier="...", model="..."}`

**Verified Output:**

```
authorized_hits{model="facebook-opt-125m-simulated",tier="free",user="tgitelma-redhat-com-dd264a84",limitador_namespace="llm/facebook-opt-125m-simulated-kserve-route"} 376
authorized_calls{user="ahadas-redhat-com-1e8bdd56",tier="free",model="facebook-opt-125m-simulated",limitador_namespace="llm/facebook-opt-125m-simulated-kserve-route"} 19
limited_calls{model="facebook-opt-125m-simulated",user="tgitelma-redhat-com-dd264a84",tier="free",limitador_namespace="llm/facebook-opt-125m-simulated-kserve-route"} 20
```

**Status**: ✅ All custom labels exported to Prometheus

---

### Step 5: Latency Metrics (✅ Working)

**Component**: HAProxy → Prometheus

**What happens:**

- HAProxy exports latency metrics for each route
- Filtered to MaaS routes: `haproxy_backend_http_average_response_latency_milliseconds{route=~"maas.*"}`

**Status**: ✅ Latency metrics available for MaaS gateway route

---

## Component Summary

| Component | Exports Metrics? | Enriches with Custom Labels? | Status |
| --------- | ---------------- | ---------------------------- | ------ |
| **TelemetryPolicy** | ❌ No | ✅ Configures extraction | ✅ Working |
| **Envoy WasmPlugin** | ❌ No | ✅ Extracts labels | ✅ Working |
| **Authorino** | ✅ Yes (own metrics) | ❌ No | ✅ Working |
| **Limitador** | ✅ Yes | ✅ Yes - exports all labels | ✅ Working |
| **HAProxy** | ✅ Yes (latency) | ❌ No (route-level only) | ✅ Working |
| **Prometheus** | ❌ No | ❌ No | ✅ Working |

---

## Available Labels

| Label | Source | Example Value |
| ----- | ------ | ------------- |
| `user` | `auth.identity.userid` | `tgitelma-redhat-com-dd264a84` |
| `tier` | `auth.identity.tier` | `free`, `premium`, `enterprise` |
| `model` | `request.path` | `facebook-opt-125m-simulated` |
| `limitador_namespace` | HTTPRoute | `llm/facebook-opt-125m-simulated-kserve-route` |
| `route` | HAProxy | `maas-gateway-route` |

---

## What's Missing (Blocked by Dependencies)

| Missing Feature | Why | Blocked By |
|-----------------|-----|------------|
| **P50/P99 Latency** | HAProxy only provides averages, not histograms | Need Istio/Envoy histogram metrics |
| **Latency per API Key** | HAProxy metrics don't include user/API key labels | Would need tracing or custom instrumentation |
| **Model Deployment Status** | Can't show if model is "Ready/NotReady" | RHOAIENG-25355 - KServe metrics integration |
| **Model Resource Allocation** | CPU/GPU/Memory per model | RHOAIENG-12528 - Resource metrics |
| **Actual Token Consumption** | LLM tokens (input/output), not just request hits | RHOAIENG-28166 - Token metrics |
| **Model Inference Latency** | Time spent in model inference vs routing | Requires KServe metrics |

---

## Verification Command

```bash
# Check Limitador metrics directly
oc exec -n kuadrant-system deploy/limitador-limitador -- curl -s localhost:8080/metrics | grep -E "^(authorized_hits|authorized_calls|limited_calls)"

# Check HAProxy latency metrics
oc exec -n openshift-monitoring -c prometheus prometheus-k8s-0 -- curl -s 'http://localhost:9090/api/v1/query?query=haproxy_backend_http_average_response_latency_milliseconds{route=~"maas.*"}'
```

# 📊 MaaS Metrics Summary

## ✅ AVAILABLE METRICS (Currently Working in Prometheus)

### Limitador Metrics

| Metric | Source | Labels Available | Notes |
| ------ | ------ | ---------------- | ----- |
| `authorized_hits` | Limitador | ✅ `user`, `tier`, `model`, `limitador_namespace` | ✅ Working - Counts successful API calls |
| `authorized_calls` | Limitador | ✅ `user`, `tier`, `model`, `limitador_namespace` | ✅ Working - Rate limiting success counter |
| `limited_calls` | Limitador | ✅ `user`, `tier`, `model`, `limitador_namespace` | ✅ Working - Rate limiting block counter |
| `limitador_up` | Limitador | Standard Prometheus labels | ✅ Working - Health check metric |

**Example metrics (verified on cluster):**

```
authorized_hits{model="facebook-opt-125m-simulated",tier="free",user="tgitelma-redhat-com-dd264a84",limitador_namespace="llm/facebook-opt-125m-simulated-kserve-route"} 376
authorized_calls{user="ahadas-redhat-com-1e8bdd56",tier="free",model="facebook-opt-125m-simulated",limitador_namespace="llm/facebook-opt-125m-simulated-kserve-route"} 19
limited_calls{model="facebook-opt-125m-simulated",user="tgitelma-redhat-com-dd264a84",tier="free",limitador_namespace="llm/facebook-opt-125m-simulated-kserve-route"} 20
```

**Note**:

- `limitador_namespace` identifies the HTTPRoute (e.g., `llm/facebook-opt-125m-simulated-kserve-route`)
- **TelemetryPolicy** (`deployment/base/observability/telemetry-policy.yaml`) configures extraction of `user`, `tier`, `model` labels
- All custom labels are now exported correctly by Limitador

---

### Authorino Metrics

| Metric | Source | Labels Available | Missing Labels | Notes |
| ------ | ------ | ---------------- | -------------- | ----- |
| `auth_server_authconfig_total` | Authorino | `authconfig`, `namespace`, `evaluator_type` | N/A | ✅ Working - Auth config evaluation count |
| `auth_server_response_status_total` | Authorino | `authconfig`, `namespace`, `status` | N/A | ✅ Working - Auth response status codes |
| `grpc_server_handled_total` | Authorino | `grpc_code`, `grpc_method`, `grpc_service`, `grpc_type` | N/A | ✅ Working - gRPC request handling |
| `grpc_server_started_total` | Authorino | `grpc_code`, `grpc_method`, `grpc_service`, `grpc_type` | N/A | ✅ Working - gRPC request start counter |

---

### Envoy Gateway Metrics

| Metric | Source | Labels Available | Notes |
| ------ | ------ | ---------------- | ----- |
| `envoy_http_downstream_rq_xx` | Envoy | `envoy_response_code_class`, `namespace` | ✅ Working - HTTP requests by response code class (2xx, 4xx, 5xx) |

**Useful Queries:**

```promql
# Successful requests (2xx)
sum(envoy_http_downstream_rq_xx{envoy_response_code_class="2",namespace="llm"})

# Client errors (4xx) - includes rate limited and auth denied
sum(envoy_http_downstream_rq_xx{envoy_response_code_class="4",namespace="llm"})

# Server errors (5xx)
sum(envoy_http_downstream_rq_xx{envoy_response_code_class="5",namespace="llm"})
```

---

### HAProxy Ingress Metrics

| Metric | Source | Labels Available | Notes |
| ------ | ------ | ---------------- | ----- |
| `haproxy_backend_http_responses_total` | HAProxy | `code` (2xx, 4xx, 5xx) | ✅ Working - Cluster ingress traffic |
| `haproxy_backend_http_average_response_latency_milliseconds` | HAProxy | `route` | ✅ Working - Average latency per route |

**Useful Queries:**

```promql
# Total 2xx responses through cluster ingress
sum(haproxy_backend_http_responses_total{code="2xx"})

# 4xx errors in last hour
sum(increase(haproxy_backend_http_responses_total{code="4xx"}[1h]))

# Average latency for MaaS routes
avg(haproxy_backend_http_average_response_latency_milliseconds{route=~"maas.*"})

# Latency per route (MaaS only)
avg by (route) (haproxy_backend_http_average_response_latency_milliseconds{route=~"maas.*"})
```

---

### Kubernetes Metrics (kube-state-metrics)

| Metric | Source | Labels Available | Notes |
| ------ | ------ | ---------------- | ----- |
| `kube_pod_status_phase` | kube-state-metrics | `namespace`, `pod`, `phase` | ✅ Working - Pod health status |

**Useful Queries:**

```promql
# Running pods in maas-api namespace
count(kube_pod_status_phase{namespace="maas-api", phase="Running"} == 1)

# Gateway pods running
count(kube_pod_status_phase{namespace="openshift-ingress", pod=~"maas.*", phase="Running"} == 1)
```

---

### vLLM Model Metrics (Potential)

| Metric | Source | Labels Available | Notes |
| ------ | ------ | ---------------- | ----- |
| `vllm:num_requests_running` | vLLM | `model_name` | ⚠️ Available if vLLM model deployed |
| `vllm:num_requests_waiting` | vLLM | `model_name` | ⚠️ Available if vLLM model deployed |
| `vllm:request_latency_seconds` | vLLM | `model_name` | ⚠️ Histogram for latency tracking |
| `vllm:prompt_tokens_total` | vLLM | `model_name` | ⚠️ Token consumption tracking |
| `vllm:generation_tokens_total` | vLLM | `model_name` | ⚠️ Token generation tracking |

**Note**: vLLM metrics are scraped via ServiceMonitor (`vllm-simulator-monitor`, `qwen3-model-monitor`) but require vLLM-based model deployment.

---

### TelemetryPolicy Configuration Summary

**Policy**: `user-group` (targets `maas-default-gateway`)  
**File**: `deployment/base/observability/telemetry-policy.yaml`

| Configured Label | Extraction Method | Status |
| ---------------- | ----------------- | ------ |
| `model` | `request.path.split("/")[2]` - Extracts from URL path `/llm/{model}/v1/...` | ✅ Working |
| `tier` | `auth.identity.tier` - From Authorino identity context | ✅ Working |
| `user` | `auth.identity.userid` - From Authorino identity context | ✅ Working |

**How it works:**

1. ✅ Envoy WasmPlugin extracts `model`, `tier`, `user` from request context
2. ✅ Sends this data as **dynamic metadata** to Limitador for rate limiting decisions
3. ✅ **Limitador exports all labels to Prometheus metrics**

**Verified Output:**

```
authorized_hits{model="facebook-opt-125m-simulated", tier="free", user="tgitelma-redhat-com-dd264a84", limitador_namespace="llm/..."}
```

---

## 📋 Dashboard Queries

### ✅ Working Queries with Custom Labels

All queries using `user`, `tier`, `model` labels are now working!

#### Per-User Queries

```promql
# Requests per user
sum by (user) (authorized_hits)

# Rate limited requests per user
sum by (user) (limited_calls)

# User throughput
rate(authorized_hits{user="tgitelma-redhat-com-dd264a84"}[5m])
```

#### Per-Model Queries

```promql
# Requests per model
sum by (model) (authorized_hits)

# Model error rates
sum by (model) (limited_calls)

# Model throughput
rate(authorized_hits{model="facebook-opt-125m-simulated"}[5m])
```

#### Per-Tier Queries

```promql
# Requests per tier
sum by (tier) (authorized_hits)

# Tier distribution
sum by (tier) (rate(authorized_hits[5m]))
```

#### Combined Queries

```promql
# User activity by model
sum by (user, model) (authorized_hits)

# Top users by requests
topk(10, sum by (user) (authorized_hits))

# Success rate per user
sum by (user) (authorized_calls) / (sum by (user) (authorized_calls) + sum by (user) (limited_calls))
```

---

### Dashboard Query Summary

| Dashboard | Status | Capabilities |
| --------- | ------ | ------------ |
| **Platform Admin** | ✅ Fully Working | Component health, per-model metrics, per-user traffic, tier analysis, latency by route |
| **AI Engineer** | ✅ Fully Working | Per-user filtering, model usage, rate limit tracking, hourly patterns |
| **Token Metrics** | ✅ Fully Working | Revenue calculations, cost per user, billing tables |

---

## ❌ MISSING METRICS (Blocked by Dependencies)

### What's NOT Available Yet

| Missing Feature | Why It's Needed | What's Blocking It | Jira |
|-----------------|-----------------|-------------------|------|
| **P50/P99 Latency** | Better latency analysis than averages | HAProxy only exports averages, need histogram metrics | - |
| **Latency per API Key/User** | Track performance per customer | HAProxy metrics don't include user labels | - |
| **Token Consumption** | Actual LLM tokens (input/output), not request counts | Requires LLM-level instrumentation | **RHOAIENG-28166** |
| **Model Status (Ready/Not Ready)** | Show model health in dashboard | KServe doesn't expose model status metrics | **RHOAIENG-25355** |
| **Model Resource Allocation** | Show CPU/GPU/Memory per model | KServe resource metrics not available | **RHOAIENG-12528** |
| **Model Inference Latency** | Time spent in LLM inference (not routing) | Requires KServe/vLLM metrics integration | - |
| **Per-Request Token Counts** | Tokens per request for accurate billing | Not exposed by current LLM simulators | **RHOAIENG-28166** |

### Workarounds Currently Used

| Missing Feature | Current Workaround |
|-----------------|-------------------|
| P50/P99 Latency | Using average latency from HAProxy |
| Model Status | Using `kube_pod_status_phase` to show running pod counts |
| Token Consumption | Using request counts as proxy for usage |

---

## ✅ Current Status

### Custom Labels Are Working!

**Verified on cluster** - All custom labels (`user`, `tier`, `model`) are now being exported by Limitador.

| Component | Status |
| --------- | ------ |
| **TelemetryPolicy** | ✅ Correctly configured |
| **Limitador Export** | ✅ All labels exported |
| **Prometheus Scraping** | ✅ Working |
| **HAProxy Latency** | ✅ Working (route-level) |
| **kube-state-metrics** | ✅ Working (pod status) |

---

## 📊 Summary Table

| Category | Metrics Available | Custom Labels | Status |
| -------- | ----------------- | ------------- | ------ |
| **Limitador** | ✅ 4 metrics | ✅ `user`, `tier`, `model`, `limitador_namespace` | ✅ Fully working |
| **Authorino** | ✅ 4+ metrics | ✅ Standard labels | ✅ Fully working |
| **Envoy** | ✅ HTTP metrics | ✅ `response_code_class` | ✅ Fully working |
| **HAProxy** | ✅ Latency + HTTP | ✅ `route` | ✅ Fully working |
| **kube-state-metrics** | ✅ Pod status | ✅ `namespace`, `pod`, `phase` | ✅ Fully working |
| **TelemetryPolicy** | ✅ Configured | ✅ All labels exported | ✅ Fully working |

---

## 📋 Complete Metrics List

### ✅ Limitador Metrics (All Custom Labels Working)

| Metric Name | Available Labels | Filtering Capability |
| ----------- | ---------------- | -------------------- |
| `authorized_hits` | `user`, `tier`, `model`, `limitador_namespace` | ✅ By user, tier, model, route |
| `authorized_calls` | `user`, `tier`, `model`, `limitador_namespace` | ✅ By user, tier, model, route |
| `limited_calls` | `user`, `tier`, `model`, `limitador_namespace` | ✅ By user, tier, model, route |
| `limitador_up` | Standard labels | ✅ Health check |

### ✅ Authorino Metrics

| Metric Name | Available Labels | Filtering Capability |
| ----------- | ---------------- | -------------------- |
| `auth_server_authconfig_total` | `authconfig`, `namespace`, `evaluator_type` | ✅ By authconfig |
| `auth_server_response_status_total` | `authconfig`, `namespace`, `status` | ✅ By status |
| `grpc_server_handled_total` | `grpc_code`, `grpc_method`, `grpc_service` | ✅ By gRPC method |

### ✅ HAProxy Metrics

| Metric Name | Available Labels | Filtering Capability |
| ----------- | ---------------- | -------------------- |
| `haproxy_backend_http_average_response_latency_milliseconds` | `route` | ✅ By route (filter `route=~"maas.*"`) |
| `haproxy_backend_http_responses_total` | `code` | ✅ By HTTP status code class |

### ✅ Kubernetes Metrics

| Metric Name | Available Labels | Filtering Capability |
| ----------- | ---------------- | -------------------- |
| `kube_pod_status_phase` | `namespace`, `pod`, `phase` | ✅ By namespace, pod, phase |

### 📍 Metrics from Other Components (Future)

| Metric Type | Source | Blocked By |
| ----------- | ------ | ---------- |
| Token consumption | LLM (vLLM/KServe) | RHOAIENG-28166 |
| Model status | KServe | RHOAIENG-25355 |
| Resource allocation | KServe | RHOAIENG-12528 |
| Latency histograms | Gateway (Envoy/Istio) | Custom instrumentation |

---

## 🔗 Related Files

- **TelemetryPolicy**: `deployment/base/observability/telemetry-policy.yaml`
- **ServiceMonitor**: `deployment/base/observability/servicemonitor.yaml`
- **Platform Admin Dashboard JSON**: `docs/samples/dashboards/platform-admin-dashboard.json`
- **AI Engineer Dashboard JSON**: `docs/samples/dashboards/ai-engineer-dashboard.json`
- **Token Metrics Dashboard JSON**: `docs/samples/dashboards/maas-token-metrics-dashboard.json`
- **Deployment Script**: `deployment/scripts/observability/deploy-openshift-observability.sh`

### GitOps Dashboard Installation (Persistent)

- **Dashboard Kustomization**: `deployment/components/observability/dashboards/kustomization.yaml`
- **Platform Admin CRD**: `deployment/components/observability/dashboards/dashboard-platform-admin.yaml`
- **AI Engineer CRD**: `deployment/components/observability/dashboards/dashboard-ai-engineer.yaml`

**Deploy persistent dashboards:**
```bash
# Ensure Grafana instance has the label
oc label grafana grafana -n llm-observability app=grafana

# Apply dashboard CRDs
oc apply -k deployment/components/observability/dashboards
```

**Dashboards installed via CRDs:**
- ✅ Platform Admin Dashboard → `MaaS v1.0` folder
- ✅ AI Engineer Dashboard → `MaaS v1.0` folder
- ⚠️ Token Metrics Dashboard → Manual import only (source in `docs/samples/dashboards/`)

---

## 📝 Notes

1. **Custom Labels Working**: All custom labels (`user`, `tier`, `model`) are now exported by Limitador and available for dashboard queries.

2. **TelemetryPolicy**: The policy correctly extracts labels from request context and Limitador exports them to Prometheus.

3. **Dashboard Compatibility**: All dashboards can now use full filtering by user, tier, and model.

4. **Latency Metrics**: Available at route level via HAProxy. For per-user latency, additional instrumentation would be needed.

5. **Blocked Features**: P50/P99 latency, token consumption, and model status metrics are blocked by pending Jira tickets (RHOAIENG-25355, RHOAIENG-12528, RHOAIENG-28166).

6. **Verified Users**:
   - `tgitelma-redhat-com-dd264a84`
   - `ahadas-redhat-com-1e8bdd56`

7. **Verified Models**:
   - `facebook-opt-125m-simulated`

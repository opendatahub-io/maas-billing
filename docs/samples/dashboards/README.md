# 📊 MaaS Grafana Dashboards

This directory contains Grafana dashboard samples for the MaaS platform.

## 📁 Dashboard Files

| File | Description |
| ---- | ----------- |
| `platform-admin-dashboard.json` | Unified view for platform administrators |
| `ai-engineer-dashboard.json` | API key-filtered view for AI engineers |
| `maas-token-metrics-dashboard.json` | Legacy token metrics dashboard |

## 📖 Documentation Files

| File | Description |
| ---- | ----------- |
| `METRICS-SUMMARY.md` | **Main reference** - Complete metrics documentation, queries, and limitations |
| `METRICS-EXPORT-FLOW.md` | Architecture flow showing how metrics are exported |
| `PROMETHEUS-COUNTER-BEHAVIOR.md` | Educational guide on Prometheus counter behavior |

## 🎯 Available Metrics

### ✅ All Metrics Working!

| Category | Metrics | Labels |
| -------- | ------- | ------ |
| **Limitador** | `authorized_hits`, `authorized_calls`, `limited_calls`, `limitador_up` | ✅ `user`, `tier`, `model`, `limitador_namespace` |
| **Istio Gateway** | `istio_requests_total`, `istio_request_duration_milliseconds_bucket` | ✅ `response_code`, `destination_service_name` |
| **vLLM/KServe** | `vllm:num_requests_running`, `vllm:num_requests_waiting`, `vllm:gpu_cache_usage_perc` | ✅ `model_name` |
| **Kubernetes** | `kube_pod_status_phase`, `ALERTS` | ✅ `namespace`, `pod`, `alertname` |
| **Authorino** | `controller_runtime_reconcile_*` | ⚠️ Operator metrics only |

**Verified on cluster:**

```
authorized_hits{model="facebook-opt-125m-simulated",tier="free",user="tgitelma-redhat-com-dd264a84",...} 376
istio_requests_total{response_code="200",destination_service_name="facebook-opt-125m-simulated-kserve-workload-svc",...} 55
```

See `METRICS-SUMMARY.md` for full details and query examples.

## 🔧 How to Use

1. **Automated Deployment (Recommended):**
   ```bash
   ./scripts/install-observability.sh
   ```
   This script installs Grafana, configures Prometheus datasource, and deploys all dashboards.

2. **Manual Import:**
   - Go to Grafana → Dashboards → Import
   - Upload the desired dashboard JSON file
   - Configure Prometheus datasource

3. **Prerequisites:**
   - User-workload-monitoring enabled in OpenShift
   - ServiceMonitors deployed for Limitador, Istio Gateway, and KServe models
   - Kuadrant policies configured with TelemetryPolicy

## 📈 Working Queries

```promql
# Requests per user
sum by (user) (authorized_hits)

# Requests per model
sum by (model) (authorized_hits)

# Top 10 users
topk(10, sum by (user) (authorized_hits))

# Success rate per user
sum by (user) (authorized_calls) / (sum by (user) (authorized_calls) + sum by (user) (limited_calls))

# P95 latency by service (Istio)
histogram_quantile(0.95, sum by (destination_service_name, le) (rate(istio_request_duration_milliseconds_bucket[5m])))

# Unauthorized requests (401)
sum(rate(istio_requests_total{response_code="401"}[5m]))

# Overall error rate (4xx + 5xx)
sum(rate(istio_requests_total{response_code=~"4.."}[5m])) + sum(rate(istio_requests_total{response_code=~"5.."}[5m]))

# Firing alerts in MaaS namespaces
count(ALERTS{alertstate="firing", namespace=~"llm|kuadrant-system|maas-api"})
```

## 🔗 Related Files

- **TelemetryPolicy**: `deployment/base/observability/telemetry-policy.yaml`
- **ServiceMonitors**: `deployment/components/observability/prometheus/`

## 📊 Dashboard Panels

### Platform Admin Dashboard
- **Overview**: MaaS API pods, Gateway pods, Model pods, Success rate, P50 latency
- **Alerts**: Firing alerts count, Active alerts table (filtered to MaaS namespaces)
- **Rate Limiting**: Request rate by user/model/tier, Rate limited requests
- **Errors**: Overall error rate (4xx + 5xx from Istio + Limitador)
- **Latency**: P95 latency by service (from Istio histograms)
- **Model Metrics**: Requests running/waiting, GPU cache usage, Token throughput
- **Top Users**: Top 10 by hits, Top 10 by declined requests

### AI Engineer Dashboard
- **User-filtered views**: Per-user request volumes and rate limiting

## 📝 Notes

- Dashboards are compatible with Kuadrant v1.2.0+ (with custom Limitador build)
- ✅ Per-user, per-model, per-tier filtering is fully working
- ✅ P50/P95/P99 latency from Istio gateway histograms
- ✅ Error tracking (401, 429, 5xx) from Istio + Limitador
- ✅ Alert integration (MaaS-filtered firing alerts)
- ✅ vLLM/KServe model metrics (queue depth, GPU cache)
- Requires Prometheus Operator for ServiceMonitor support
- Dashboard auto-refreshes every 30 seconds

To customize the dashboard:
1. Import into Grafana
2. Edit panels as needed
3. Export updated JSON
4. Replace this file with your custom version

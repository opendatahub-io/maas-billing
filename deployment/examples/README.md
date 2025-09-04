# MaaS Deployment Examples

Complete deployment examples for different Models-as-a-Service scenarios.

## Prerequisites

Deploy [core-infrastructure](../core-infrastructure/) first:

```bash
# 1. Install Istio
./scripts/installers/install-istio.sh

# 2. Install Cert Manager
./scripts/installers/install-cert-manager.sh

# 3. Install KServe
./scripts/installers/install-kserve.sh

# 4. Install Prometheus
./scripts/installers/install-prometheus.sh

# Deploy core infrastructure
export CLUSTER_DOMAIN="apps.your-cluster.com"
export CLUSTER_DOMAIN="apps.your-cluster.com"
cd ../core-infrastructure   # from deployment/examples/
kustomize build . | envsubst | kubectl apply -f -

## Platform-Specific Deployments

For platform-specific deployments with external access, use the overlays:

- **OpenShift**: `kustomize build ../overlays/openshift`
- **Kubernetes**: `kustomize build ../overlays/kubernetes`  
- **Internal only**: Use the examples directly without overlays

See [overlays/README.md](../overlays/README.md) for more details.

## Available Examples

### Basic Deployment
Minimal setup with simulator model and API key authentication:

```bash
cd basic-deployment
export CLUSTER_DOMAIN="apps.your-cluster.com"
kustomize build . | envsubst | kubectl apply -f -
```

**Includes:**
- vLLM Simulator model
- API key authentication
- Basic gateway routing

### Simulator Deployment  
Full-featured setup with authentication, rate limiting, and observability:

**For OpenShift:**
```bash
export CLUSTER_DOMAIN="apps.your-cluster.com"
kustomize build ../overlays/openshift | envsubst | kubectl apply -f -
```

**For Kubernetes:**
```bash
export CLUSTER_DOMAIN="your-kubernetes-domain.com"
kustomize build ../overlays/kubernetes | envsubst | kubectl apply -f -
```

**Internal access only (no external routes):**
```bash
cd simulator-deployment
export CLUSTER_DOMAIN="internal.cluster.local"
kustomize build . | envsubst | kubectl apply -f -

**Includes:**
- vLLM Simulator model
- API key authentication
- Token-based rate limiting
- Prometheus ServiceMonitors
- Token usage metrics

### GPU Deployment
Production setup with GPU-accelerated models:

**For OpenShift:**
```bash
export CLUSTER_DOMAIN="apps.your-cluster.com"
# Note: Currently uses simulator-deployment base. For GPU models, use:
cd gpu-deployment
kustomize build . | envsubst | kubectl apply -f -
```

**For Kubernetes:**
```bash
export CLUSTER_DOMAIN="your-kubernetes-domain.com"
cd gpu-deployment
kustomize build . | envsubst | kubectl apply -f -
```

**Includes:**
- vLLM Simulator model
- Qwen3-0.6B GPU model
- API key authentication  
- Token-based rate limiting
- Prometheus ServiceMonitors

## Testing Your Deployment

### Basic Connectivity
```bash
# Test simulator model
curl -H 'Authorization: APIKEY freeuser1_key' \
     -H 'Content-Type: application/json' \
     -d '{"model":"simulator-model","messages":[{"role":"user","content":"Hello!"}]}' \
     http://simulator-llm.${CLUSTER_DOMAIN}/v1/chat/completions
```

### Rate Limiting
```bash
# Test rate limiting (Free tier: expect 429 after 5 requests in 2min)
for i in {1..10}; do
  printf "Request #%-2s -> " "$i"
  curl -s -o /dev/null -w "%{http_code}\n" \
       -H 'Authorization: APIKEY freeuser1_key' \
       -H 'Content-Type: application/json' \
       -d '{"model":"simulator-model","messages":[{"role":"user","content":"Test"}],"max_tokens":10}' \
       http://simulator-llm.${CLUSTER_DOMAIN}/v1/chat/completions
done
```

### GPU Models (GPU deployment only)
```bash
# Test Qwen3 model
curl -H 'Authorization: APIKEY premiumuser1_key' \
     -H 'Content-Type: application/json' \
     -d '{"model":"qwen3-0-6b-instruct","messages":[{"role":"user","content":"Hello!"}]}' \
     http://qwen3-llm.${CLUSTER_DOMAIN}/v1/chat/completions
```

## Available API Keys

| Tier     | API Keys                               | Rate Limits (Token-based) |
|----------|----------------------------------------|---------------------------|
| Free     | `freeuser1_key`, `freeuser2_key`       | 100 tokens per 1min       |
| Premium  | `premiumuser1_key`, `premiumuser2_key` | 500 tokens per 1min       |

## Component Details

### Models (`kustomize-templates/models/`)
- **simulator/**: Lightweight vLLM simulator for testing
- **qwen3/**: GPU-accelerated Qwen3-0.6B model + vLLM runtime

### Authentication (`kustomize-templates/auth/`)  
- **api-keys/**: API key secrets and AuthPolicy
- **token-rate-limiting/**: TokenRateLimitPolicy for usage-based limits

### Observability (`kustomize-templates/observability/`)
- **service-monitors.yaml**: Kuadrant component monitoring
- **token-metrics.yaml**: Token usage metrics from custom wasm-shim

## Customization

### Adding New Models
1. Create new directory under `kustomize-templates/models/`
2. Add InferenceService manifest
3. Update HTTPRoute in `core-infrastructure/kustomize-templates/gateway/`

### Custom Rate Limits
Edit `kustomize-templates/auth/token-rate-limiting/token-policy.yaml`:

```yaml
spec:
  limits:
    "free-tier":
      rates:
      - limit: 200  # Increase from 100
        duration: 1m
        unit: token
```

## Troubleshooting

### Model Not Ready
```bash
# Check InferenceService status
kubectl get inferenceservice -n llm

# Check pod logs
kubectl logs -n llm -l serving.kserve.io/inferenceservice=vllm-simulator
```

### Authentication Failures
```bash
# Test without auth (should return 401)
curl -w "%{http_code}\n" http://simulator-llm.${CLUSTER_DOMAIN}/v1/chat/completions

# Check AuthPolicy status
kubectl get authpolicy -n llm
```

### Rate Limiting Issues
```bash
# Check TokenRateLimitPolicy
kubectl get tokenratelimitpolicy -n llm

# Check WasmPlugin configuration
kubectl get wasmplugin -n llm -o yaml | grep url
```

### No Metrics Data
```bash
# Check ServiceMonitors
kubectl get servicemonitor -n llm
kubectl get servicemonitor -n kuadrant-system
# or
kubectl get servicemonitor -A | egrep '^(llm|kuadrant-system)\s'

# Check if custom wasm-shim is loaded
kubectl logs -n llm deployment/inference-gateway-istio | grep nerdalert

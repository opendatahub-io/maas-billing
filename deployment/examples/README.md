# MaaS Deployment Examples

Complete deployment examples for different Models-as-a-Service scenarios.

## Automated Installation (Recommended)

Use the install script to ensure proper installation sequence:

```bash
# Show available deployment types and options
deployment/scripts/install.sh --help

# Deploy with default settings (simulator deployment)
deployment/scripts/install.sh

# Deploy specific deployment type
deployment/scripts/install.sh gpu
deployment/scripts/install.sh basic

# Add a new deployment type

Any subdirectory under deployment/examples/ named as {name}-deployment will be picked up as a deployment type by deployment/scripts/insall.sh

e.g. deployment/examples/gpu-deployment will display as the deployment option --gpu in the installer.

## Installation Sequence

The install script enforces this critical sequence for reliable deployment:

1. **Install Dependencies** - Install all required operators and tools
   ```bash
   scripts/install-dependencies.sh --all
   ```

2. **Set Cluster Domain** - Configure domain for external access
   ```bash
   # For OpenShift clusters (auto-detected by install script)
   export CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
   
   # For non-OpenShift clusters, set manually
   export CLUSTER_DOMAIN="your-kubernetes-domain.com"
   ```

3. **Clean Conflicting Operators** - Remove any conflicting Istio installations
   ```bash
   kubectl -n gateway-system delete subscription sailoperator --ignore-not-found
   kubectl -n gateway-system delete csv sailoperator.v0.1.0 --ignore-not-found
   kubectl -n gateway-system delete deployment sail-operator --ignore-not-found
   kubectl -n gateway-system delete deployment istiod --ignore-not-found
   ```

4. **Deploy Kuadrant Operators** - Install core infrastructure operators
   ```bash
   kustomize build core-infrastructure/kustomize-templates/kuadrant | envsubst | kubectl apply -f -
   ```

5. **Wait for Operators** - Ensure all operators are ready before proceeding
   ```bash
   kubectl wait --for=condition=available deployment/kuadrant-operator-controller-manager -n kuadrant-system --timeout=300s
   kubectl wait --for=condition=available deployment/limitador-operator-controller-manager -n kuadrant-system --timeout=300s
   kubectl wait --for=condition=available deployment/authorino-operator -n kuadrant-system --timeout=300s
   ```

6. **Deploy with External Access** - Deploy the selected example with OpenShift overlay
   ```bash
   kustomize build overlays/openshift | envsubst | kubectl apply -f -
   ```

**Important**: This sequence prevents common issues like operator conflicts and ensures dependencies are ready before deployment.

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

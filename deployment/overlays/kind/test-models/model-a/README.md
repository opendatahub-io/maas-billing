# llm-katan Test Model for Kind

This directory contains Kubernetes manifests for deploying [llm-katan](https://github.com/vllm-project/semantic-router/tree/main/e2e-tests/llm-katan) as a test model in the Kind local development environment.

## What is llm-katan?

llm-katan is a lightweight LLM serving package designed for **testing and development**:
- Ultra-small model (Qwen2.5-0.5B - 500M parameters)
- CPU-only inference (no GPU needed)
- OpenAI-compatible API
- Fast startup (~30 seconds)
- Minimal resources (2-3GB RAM)

Perfect for testing the MaaS platform without heavy models!

## Quick Deploy

```bash
# 1. Create namespace (if not exists)
kubectl create namespace llm --dry-run=client -o yaml | kubectl apply -f -

# 2. (Optional) Create HuggingFace token secret for private models
kubectl create secret generic huggingface-token -n llm \
  --from-literal=token=YOUR_HUGGINGFACE_TOKEN

# 3. Deploy llm-katan
kubectl apply -k deployment/overlays/kind/test-models/llm-katan/

# 4. Wait for pod to be ready (may take 30-60 seconds for model download)
kubectl wait --for=condition=Ready pod -l app=llm-katan -n llm --timeout=300s

# 5. Check status
kubectl get pods -n llm
kubectl logs -n llm -l app=llm-katan
```

## Test the Model

### Via Port-Forward

```bash
# Port-forward to local machine
kubectl port-forward -n llm svc/llm-katan 8000:8000

# Test health endpoint
curl http://localhost:8000/health

# List models
curl http://localhost:8000/v1/models

# Send chat completion request
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llm-katan",
    "messages": [{"role": "user", "content": "Hello! What is 2+2?"}],
    "max_tokens": 50
  }'
```

### Via Gateway (with MaaS Platform)

```bash
# Get auth token
TOKEN=$(kubectl create token default -n maas-api --duration=10m)

# Test via gateway
kubectl port-forward -n maas-api svc/maas-gateway-istio 8080:80

curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llm-katan",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Resource Requirements

- **CPU**: 500m (request), 2 cores (limit)
- **Memory**: 1Gi (request), 3Gi (limit)
- **Disk**: ~2GB for model cache
- **Startup Time**: 30-60 seconds (first time, includes model download)

## Troubleshooting

### Pod Not Starting

```bash
# Check pod status
kubectl describe pod -n llm -l app=llm-katan

# Check logs
kubectl logs -n llm -l app=llm-katan --tail=50

# Common issues:
# 1. Model download timeout - increase startupProbe failureThreshold
# 2. Out of memory - increase memory limits
# 3. HuggingFace token - check secret or use public models
```

### Model Download Issues

If using a private model or hitting rate limits:
```bash
# Create HuggingFace token secret
kubectl create secret generic huggingface-token -n llm \
  --from-literal=token=hf_YOUR_TOKEN_HERE

# Delete and recreate pod to pick up secret
kubectl delete pod -n llm -l app=llm-katan
```

### Test Different Models

Edit `deployment.yaml` to use different models:
```yaml
args:
- "--model"
- "Qwen/Qwen2.5-0.5B-Instruct"  # Change this
- "--served-model-name"
- "my-model-name"  # Change this
```

Recommended lightweight models:
- `Qwen/Qwen2.5-0.5B-Instruct` (500M - fastest)
- `Qwen/Qwen2.5-1.5B-Instruct` (1.5B - better quality)
- `facebook/opt-125m` (125M - smallest)

## Use Cases for Testing

### 1. Policy Enforcement
Test AuthPolicy and RateLimitPolicy against real model endpoint

### 2. Multi-Model Routing
Deploy multiple llm-katan instances with different names to test routing

### 3. Load Testing
Lightweight enough to run multiple instances for load testing

### 4. Integration Testing
Real AI responses for realistic end-to-end testing

## Cleanup

```bash
kubectl delete -k deployment/overlays/kind/test-models/llm-katan/
```

## References

- GitHub: https://github.com/vllm-project/semantic-router/tree/main/e2e-tests/llm-katan
- PyPI: https://pypi.org/project/llm-katan/
- Docker: ghcr.io/vllm-project/semantic-router/llm-katan

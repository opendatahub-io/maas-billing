# Core Infrastructure Deployment

Base platform components required for Models-as-a-Service (MaaS) deployment.

## Quick Start

```bash
# 1. Install required dependencies (recommended)
./scripts/install-dependencies.sh --all

# Alternative: Install dependencies individually
# ./scripts/installers/install-istio.sh
# ./scripts/installers/install-cert-manager.sh
# ./scripts/installers/install-kserve.sh           # Add --ocp flag for OpenShift clusters
# ./scripts/installers/install-prometheus.sh       # Optional: for observability

# 2. Set your cluster domain
export CLUSTER_DOMAIN="apps.your-cluster.com"

# 3. Deploy core infrastructure
cd core-infrastructure
kustomize build . | envsubst | kubectl apply -f -
```

## Dependency Installation

The `install-dependencies.sh` script provides a convenient way to install all required platform components:

```bash
# Interactive mode - prompts for confirmation
./scripts/install-dependencies.sh

# Install all components without prompts
./scripts/install-dependencies.sh --all

# Install specific components
./scripts/install-dependencies.sh --istio --cert-manager
./scripts/install-dependencies.sh --kserve
./scripts/install-dependencies.sh --prometheus

# Show available options
./scripts/install-dependencies.sh --help
```

**Components installed in order:**
- **Istio**: Service mesh and Gateway API configuration
- **cert-manager**: Certificate management for TLS and webhooks  
- **KServe**: Model serving platform
- **Prometheus**: Observability and metrics collection (optional)

## Components

- **namespaces/**: Required Kubernetes namespaces (`llm`, `llm-observability`, `kuadrant-system`)
- **istio/**: Service mesh and Gateway API configuration  
- **kserve/**: Model serving platform with OpenShift integration
- **kuadrant/**: API gateway policies and operators (via OLM)
- **gateway/**: Traffic routing and external access (parameterized domains)

## Prerequisites

- **OpenShift 4.14+** or Kubernetes 1.28+ with admin access
- **kustomize v4.0+** 
- **envsubst** (for domain parameterization)

## Deployment Order

The kustomization deploys components in the correct order:

1. **namespaces** - Creates required namespaces
2. **istio** - Gateway API Gateway configuration
3. **kserve** - Model serving platform + OpenShift SecurityContextConstraints
4. **kuadrant** - OLM operators (Kuadrant, Authorino, Limitador) + custom wasm-shim
5. **gateway** - Model routing and OpenShift Routes

## Configuration

### Domain Parameterization

Set the `CLUSTER_DOMAIN` environment variable to match your cluster's ingress domain:

```bash
# For OpenShift clusters
export CLUSTER_DOMAIN="apps.my-cluster.example.com"

# For local clusters  
export CLUSTER_DOMAIN="maas.local"
```

### Custom Wasm-Shim

The deployment automatically configures Kuadrant to use the custom wasm-shim (`ghcr.io/nerdalert/wasm-shim:latest`) required for token-based rate limiting metrics.

## Verification

After deployment, verify components are ready:

```bash
# Check namespaces
kubectl get namespaces llm llm-observability kuadrant-system

# Check Kuadrant operators
kubectl get csv -n kuadrant-system

# Check Gateway
kubectl get gateway inference-gateway -n llm

# Check KServe
kubectl get configmap inferenceservice-config -n kserve
```

## Next Steps

After core infrastructure is deployed, proceed to the [examples](../examples/) directory to deploy models, authentication, and observability components.

## Troubleshooting

### Common Issues

**Domain Resolution**
```bash
# Check Gateway hostname
kubectl get gateway inference-gateway -n llm -o yaml | grep hostname
```

**Kuadrant Not Ready**
```bash
# Check operator status
kubectl get kuadrant kuadrant -n kuadrant-system
kubectl get csv -n kuadrant-system
```

**KServe Configuration**
```bash
# Verify domain in KServe config
kubectl get configmap inferenceservice-config -n kserve -o yaml | grep ingressDomain
```

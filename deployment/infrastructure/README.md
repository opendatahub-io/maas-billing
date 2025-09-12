# Core Infrastructure Deployment

Base platform components required for Models-as-a-Service (MaaS) deployment.

## Quick Start

```bash
# 1. Install required dependencies (recommended)
./deployment/scripts/install-dependencies.sh --all

# Alternative: Install dependencies individually
# ./scripts/installers/install-istio.sh
# ./scripts/installers/install-cert-manager.sh
# ./scripts/installers/install-kserve.sh           # Add --ocp flag for OpenShift clusters
# ./scripts/installers/install-prometheus.sh       # Optional: for observability (Add --ocp flag for OpenShift clusters)
# ./scripts/installers/install-grafana.sh         # Optional: for observability requires Grafana operator to be pre-installed

# 2. Set your cluster domain
export CLUSTER_DOMAIN="apps.your-cluster.com"

# 3. Deploy core infrastructure
kustomize build deployment/infrastructure | envsubst | kubectl apply -f -

# 4. Deploy custom limitador image(this should be pushed into the main product soon so this can be removed)
kubectl patch limitador limitador   -n kuadrant-system   --type merge -p '{"spec":{"image":"ghcr.io/redhat-et/limitador:metrics","version":""}}'
```

Move to [next steps](../examples/) to deploy examples.

## Dependency Installation

The `install-dependencies.sh` script provides a convenient way to install all required platform components:

```bash
# Interactive mode - prompts for confirmation
./deployment/scripts/install-dependencies.sh

# Install all components without prompts
./deployment/scripts/install-dependencies.sh --all

# Install specific components
./deployment/scripts/install-dependencies.sh --istio --cert-manager
./deployment/scripts/install-dependencies.sh --kserve
./deployment/scripts/install-dependencies.sh --prometheus

# Show available options
./deployment/scripts/install-dependencies.sh --help
```

**Components installed in order:**
- **Istio**: Service mesh and Gateway API configuration
- **cert-manager**: Certificate management for TLS and webhooks  
- **KServe**: Model serving platform
- **Prometheus**: Observability and metrics collection (optional)
- **Kuadrant**: API management and security for Kubernetes, extending Gateway API with policies. Using Authorino for auth(z) and Limitador for rate limiting.

## Components

- **namespaces/**: Required Kubernetes namespaces (`llm`, `llm-observability`, `kuadrant-system`)
- **istio/**: Service mesh and Gateway API configuration  
- **kserve/**: Model serving platform with OpenShift integration
- **kuadrant/**: API gateway policies and operators (via Helm)
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
4. **kuadrant** - Operators (Kuadrant, Authorino, Limitador) installed via Helm
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

### Kuadrant Installation

Kuadrant operators are installed via Helm by the installer script, then configured via kustomize manifests under `kustomize-templates/kuadrant/kuadrant-configure`.

## Verification

After deployment, verify components are ready:

```bash
# Check namespaces
kubectl get namespaces llm llm-observability kuadrant-system

# Check Kuadrant operators
kubectl get deployments -n kuadrant-system
kubectl get pods -n kuadrant-system

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
kubectl get deployments -n kuadrant-system
```

**KServe Configuration**
```bash
# Verify domain in KServe config
kubectl get configmap inferenceservice-config -n kserve -o yaml | grep ingressDomain
```

# Platform-Specific Overlays

This directory contains platform-specific overlays for the MaaS deployment.

## Available Overlays

### OpenShift (`openshift/`)
For OpenShift clusters, includes OpenShift Routes for external access.

```bash
# Deploy on OpenShift
export CLUSTER_DOMAIN="apps.your-openshift-cluster.com"
kustomize build overlays/openshift | envsubst | kubectl apply -f -
```

### Kubernetes (`kubernetes/`)
For standard Kubernetes clusters, includes Ingress resources for external access.

```bash
# Deploy on Kubernetes with NGINX Ingress Controller
export CLUSTER_DOMAIN="your-kubernetes-domain.com"
kustomize build overlays/kubernetes | envsubst | kubectl apply -f -
```

## Base Deployment

To deploy without external routes/ingress (internal access only):

```bash
# Deploy base configuration
export CLUSTER_DOMAIN="internal.cluster.local"
kustomize build examples/simulator-deployment | envsubst | kubectl apply -f -
```

## Choosing the Right Overlay

- **OpenShift**: Use `overlays/openshift` if running on OpenShift
- **Kubernetes**: Use `overlays/kubernetes` if running on standard Kubernetes with an ingress controller
- **Internal Only**: Use `examples/simulator-deployment` directly for cluster-internal access only
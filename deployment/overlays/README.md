# Platform-Specific Overlays

This directory contains platform-specific overlays for the MaaS deployment.

## Available Overlays

### OpenShift (`openshift/`)
For OpenShift clusters, includes OpenShift Routes for external access.

```bash
export CLUSTER_DOMAIN="apps.your-openshift-cluster.com"
kustomize build overlays/openshift | envsubst | kubectl apply -f -
```

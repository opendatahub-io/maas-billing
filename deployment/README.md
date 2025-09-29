# Deployment Guide

This guide provides instructions for deploying the MaaS Platform infrastructure and applications.

## Prerequisites

- **Kubernetes cluster** (1.25+) or **OpenShift** (4.19+)
- **kubectl** configured to access your cluster
- **kustomize** (3.10+)
- **jq** for JSON processing
- Optional: **oc** CLI for OpenShift clusters
- **For Kubernetes**: Istio will be installed via Helm by the dependency script

## Deployment Structure

```
deployment/
├── base/                    # Core infrastructure components
│   ├── gateway-routes.yaml  # Base HTTP routes
│   ├── kserve/              # KServe configuration
│   ├── maas-api/            # MaaS API deployment
│   ├── networking/          # Gateway API and Kuadrant
│   ├── policies/            # Authentication policies
│   └── token-rate-limiting/ # Rate limiting configuration
├── overlays/                # Platform-specific configurations
│   ├── openshift/           # OpenShift deployment
│   └── kubernetes/          # Standard Kubernetes deployment
├── samples/                 # Example model deployments
│   └── models/
│       ├── simulator/       # CPU-based test model
│       └── qwen3/           # GPU-based Qwen3 model
└── scripts/                 # Installation utilities
```

## Quick Start

### Automated OpenShift Deployment (Recommended)

For OpenShift clusters, use the automated deployment script:
```bash
./deployment/scripts/deploy-openshift.sh
```

This script handles all steps including feature gates, dependencies, and OpenShift-specific configurations.

### Manual Deployment Steps

### Step 0: Enable Gateway API Features (OpenShift Only)

If you're on OpenShift, you need to enable the Gateway API features first:

```bash
oc patch featuregate/cluster --type='merge' \
  -p '{"spec":{"featureSet":"CustomNoUpgrade","customNoUpgrade":{"enabled":["GatewayAPI","GatewayAPIController"]}}}'
```

Wait for the cluster operators to reconcile (this may take a few minutes).

### Step 1: Create Namespaces

```bash
for ns in kserve kuadrant-system llm maas-api; do 
  kubectl create namespace $ns || true
done
```

### Step 2: Install Dependencies

```bash
# Install required operators and CRDs
./deployment/scripts/install-dependencies.sh \
  --cert-manager \
  --kserve \
  --kuadrant
```

### Step 3: Deploy Core Infrastructure

Choose your platform:

#### OpenShift Deployment
```bash
export CLUSTER_DOMAIN="apps.your-openshift-cluster.com"
kustomize build deployment/overlays/openshift | envsubst | kubectl apply -f -
```

#### Kubernetes Deployment
```bash
export CLUSTER_DOMAIN="your-kubernetes-domain.com"
kustomize build deployment/overlays/kubernetes | envsubst | kubectl apply -f -
```

### Step 4: Deploy Sample Models (Optional)

#### Simulator Model (CPU)
```bash
kustomize build deployment/samples/models/simulator | kubectl apply -f -
```

#### Qwen3 Model (GPU Required)
```bash
# Note: Requires GPU nodes with nvidia.com/gpu resources
kustomize build deployment/samples/models/qwen3 | kubectl apply -f -
```

## Platform-Specific Configuration

### OpenShift Configuration

#### Patch Kuadrant for OpenShift Gateway Controller

If installed via Helm:
```bash
kubectl -n kuadrant-system patch deployment kuadrant-operator-controller-manager \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ISTIO_GATEWAY_CONTROLLER_NAMES","value":"openshift.io/gateway-controller/v1"}}]'
```

**Important**: After the Gateway becomes ready, restart the Kuadrant operators to ensure policies are properly enforced:
```bash
# Wait for Gateway to be ready
kubectl wait --for=condition=Programmed gateway openshift-ai-inference -n openshift-ingress --timeout=300s

# Restart Kuadrant operators
kubectl rollout restart deployment/kuadrant-operator-controller-manager -n kuadrant-system
kubectl rollout restart deployment/authorino-operator -n kuadrant-system
kubectl rollout restart deployment/limitador-operator-controller-manager -n kuadrant-system
```

If installed via OLM:
```bash
kubectl patch csv kuadrant-operator.v0.0.0 -n kuadrant-system --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/0/env/-",
    "value": {
      "name": "ISTIO_GATEWAY_CONTROLLER_NAMES",
      "value": "openshift.io/gateway-controller/v1"
    }
  }
]'
```

#### Update KServe Ingress Domain
```bash
kubectl -n kserve patch configmap inferenceservice-config \
  --type='json' \
  -p="[{
    \"op\": \"replace\",
    \"path\": \"/data/ingress\",
    \"value\": \"{\\\"enableGatewayApi\\\": true, \\\"kserveIngressGateway\\\": \\\"openshift-ingress/openshift-ai-inference\\\", \\\"ingressGateway\\\": \\\"istio-system/istio-ingressgateway\\\", \\\"ingressDomain\\\": \\\"$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')\\\"}\"
  }]"
```

#### Update Limitador Image for Metrics (Optional but Recommended)
```bash
# Update Limitador to expose metrics properly
kubectl -n kuadrant-system patch limitador limitador --type merge \
  -p '{"spec":{"image":"quay.io/kuadrant/limitador:1a28eac1b42c63658a291056a62b5d940596fd4c","version":""}}'
```

#### Configure AuthPolicy Audience
```bash
# Get the correct audience for OpenShift identities
AUD="$(kubectl create token default --duration=10m \
  | jwt decode --json - \
  | jq -r '.payload.aud[0]')"

# Apply the auth policy with correct audience
kubectl patch -f deployment/base/policies/auth-policy.yaml \
  --type='json' \
  -p "$(jq -nc --arg aud "$AUD" '[{
    op:"replace",
    path:"/spec/rules/authentication/openshift-identities/kubernetesTokenReview/audiences",
    value:[$aud]
  }]')" \
  -o yaml | kubectl apply -f -
```

### Kubernetes Configuration

For standard Kubernetes clusters, ensure you have an Ingress controller installed (e.g., NGINX):

```bash
# Install NGINX Ingress Controller (if not present)
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml
```

## Storage Initializer Configuration

The KServe storage initializer requires sufficient resources for downloading large models. Default settings in `deployment/base/kserve/kserve-config-openshift.yaml`:

- Memory Request: 4Gi
- Memory Limit: 8Gi
- CPU Request: 2
- CPU Limit: 4

To adjust for larger models:
```bash
kubectl edit configmap inferenceservice-config -n kserve
```

## Testing the Deployment

### 1. Get Gateway Endpoint
```bash
# OpenShift
HOST="$(kubectl get gateway openshift-ai-inference -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"

# Kubernetes with LoadBalancer
HOST="$(kubectl get gateway openshift-ai-inference -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"
```

### 2. Get Authentication Token
```bash
# OpenShift
TOKEN_RESPONSE=$(curl -sSk \
  -H "Authorization: Bearer $(oc whoami -t)" \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{"expiration": "10m"}' \
  "${HOST}/maas-api/v1/tokens")

TOKEN=$(echo $TOKEN_RESPONSE | jq -r .token)
```

### 3. Test Model Endpoints

For OpenShift deployments, use the gateway route:

```bash
# Get gateway route
GATEWAY_HOST="gateway.${CLUSTER_DOMAIN}"

# Test Simulator through gateway (with rate limiting)
curl -ks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -X POST \
  "https://${GATEWAY_HOST}/simulator/v1/chat/completions" \
  -d '{
    "model": "vllm-simulator",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 10
  }' | jq .

# Test Qwen3 through gateway (if deployed)
curl -ks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -X POST \
  "https://${GATEWAY_HOST}/qwen3/v1/chat/completions" \
  -d '{
    "model": "qwen3-0-6b-instruct",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }' | jq .
```

### 4. Test Rate Limiting
```bash
# Send multiple requests to trigger rate limit
for i in {1..16}; do
  curl -ks -o /dev/null -w "%{http_code}\n" \
    -H "Authorization: Bearer $TOKEN" \
    "${HOST}/simulator/health"
done
```

### 5. Verify Complete Deployment
```bash
# Check all components are running
kubectl get pods -n maas-api
kubectl get pods -n kuadrant-system
kubectl get pods -n kserve
kubectl get pods -n llm

# Check Gateway status
kubectl get gateway -n openshift-ingress openshift-ai-inference

# Check policies are enforced
kubectl get authpolicy -A
kubectl get tokenratelimitpolicy -A

# Check InferenceServices are ready
kubectl get inferenceservice -n llm
```

## Services Exposed

After deployment, the following services are available:

### OpenShift Access (with Rate Limiting)

Access models through the gateway route for proper token rate limiting:

1. **MaaS API**: `https://maas-api.${CLUSTER_DOMAIN}`
   - For token generation and management
   - Direct route to MaaS API service

2. **Gateway (for Models)**: `https://gateway.${CLUSTER_DOMAIN}`
   - **Simulator**: `https://gateway.${CLUSTER_DOMAIN}/simulator/v1/chat/completions`
   - **Qwen3**: `https://gateway.${CLUSTER_DOMAIN}/qwen3/v1/chat/completions`
   - All model access MUST go through the gateway for rate limiting

**⚠️ IMPORTANT**: Direct routes to models bypass TokenRateLimitPolicy. Always use the gateway route for production.

## Troubleshooting

### Check Component Status
```bash
# Check pods
kubectl get pods -A | grep -E "maas-api|kserve|kuadrant|simulator|qwen"

# Check services
kubectl get svc -A | grep -E "maas-api|simulator|qwen"

# Check HTTPRoutes
kubectl get httproute -A

# Check Gateway
kubectl get gateway -A
```

### View Logs
```bash
# MaaS API logs
kubectl logs -n maas-api -l app=maas-api --tail=50

# Kuadrant logs
kubectl logs -n kuadrant-system -l app=kuadrant --tail=50

# Model logs
kubectl logs -n llm -l component=predictor --tail=50
```

### Common Issues

1. **OOMKilled during model download**: Increase storage initializer memory limits
2. **GPU models not scheduling**: Ensure nodes have `nvidia.com/gpu` resources
3. **Rate limiting not working**: Verify AuthPolicy and TokenRateLimitPolicy are applied
4. **Routes not accessible**: Check Gateway status and HTTPRoute configuration
5. **TokenRateLimitPolicy MissingDependency error**: 
   - **Symptom**: TokenRateLimitPolicy shows status "token rate limit policy validation has not finished"
   - **Fix**: Run `./scripts/fix-token-rate-limit-policy.sh` or manually restart:
     ```bash
     kubectl rollout restart deployment kuadrant-operator-controller-manager -n kuadrant-system
     kubectl rollout restart deployment/authorino -n kuadrant-system
     ```
   - **Note**: This is a known Kuadrant issue that may occur after initial deployment
6. **Gateway stuck in "Waiting for controller" on OpenShift**:
   - **Symptom**: Gateway shows "Waiting for controller" indefinitely
   - **Expected behavior**: Creating the GatewayClass should automatically trigger Service Mesh installation
   - **If automatic installation doesn't work**:
     1. Install Red Hat OpenShift Service Mesh operator from OperatorHub manually
     2. Create an Istio instance:
        ```bash
        cat <<EOF | kubectl apply -f -
        apiVersion: sailoperator.io/v1
        kind: Istio
        metadata:
          name: openshift-gateway
        spec:
          version: v1.26.4
          namespace: openshift-ingress
        EOF
        ```
   - **Note**: This is typically only needed on non-RHOAI OpenShift clusters

## Next Steps

After deploying the infrastructure:

1. **Start the development environment**: See the main [README](../README.md) for frontend/backend setup
2. **Deploy additional models**: Check [samples/models](samples/models/) for more examples
3. **Configure monitoring**: Enable observability components in overlays 
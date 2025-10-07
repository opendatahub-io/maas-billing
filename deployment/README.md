# Deployment Guide

This guide provides instructions for deploying the MaaS Platform infrastructure and applications.

## Prerequisites

- **ODH/RHOAI requirements**:
  - KServe enabled in DataScienceCluster
  - Service Mesh installed (automatically installed with ODH/RHOAI)

## Important Notes

- This project assumes OpenDataHub (ODH) or Red Hat OpenShift AI (RHOAI) as the base platform
- KServe components are expected to be provided by ODH/RHOAI, not installed separately
- For non-ODH/RHOAI deployments, KServe can be optionally installed from `deployment/components/kserve`

## Quick Start

### Automated OpenShift Deployment (Recommended)

For OpenShift clusters, use the automated deployment script:
```bash
./deployment/scripts/deploy-openshift.sh
```

This script handles all steps including feature gates, dependencies, and OpenShift-specific configurations.

### Manual Deployment Steps

### Step 0: Enable Gateway API Features (OpenShift Only)

#### For OpenShift 4.19.9+
On newer OpenShift versions (4.19.9+), Gateway API is enabled by creating the GatewayClass resource. Skip to Step 1.

#### For OpenShift < 4.19.9
Enable Gateway API features manually:

```bash
oc patch featuregate/cluster --type='merge' \
  -p '{"spec":{"featureSet":"CustomNoUpgrade","customNoUpgrade":{"enabled":["GatewayAPI","GatewayAPIController"]}}}'
```

Wait for the cluster operators to reconcile (this may take a few minutes).

### Step 1: Create Namespaces

> [!NOTE]
> The `kserve` namespace is managed by ODH/RHOAI and should not be created manually.

```bash
for ns in kuadrant-system llm maas-api; do 
  kubectl create namespace $ns || true
done
```

### Step 2: Install Dependencies

Install required operators and CRDs. Note that KServe is provided by ODH/RHOAI on OpenShift.

```bash
./deployment/scripts/install-dependencies.sh \
  --cert-manager \
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

> [!NOTE]
> These models use KServe's `LLMInferenceService` custom resource, which requires ODH/RHOAI with KServe enabled.

#### Simulator Model (CPU)
```bash
PROJECT_DIR=$(git rev-parse --show-toplevel)
kustomize build ${PROJECT_DIR}/docs/samples/models/simulator/ | kubectl apply -f -
```

#### Facebook OPT-125M Model (CPU)
```bash
PROJECT_DIR=$(git rev-parse --show-toplevel)
kustomize build ${PROJECT_DIR}/docs/samples/models/facebook-opt-125m-cpu/  | kubectl apply -f -
```

#### Qwen3 Model (GPU Required)

> [!WARNING]
> This model requires GPU nodes with `nvidia.com/gpu` resources available in your cluster.

```bash
PROJECT_DIR=$(git rev-parse --show-toplevel)
kustomize build ${PROJECT_DIR}/docs/samples/models/qwen3/ | kubectl apply -f -
```

#### Verify Model Deployment
```bash
# Check LLMInferenceService status
kubectl get llminferenceservices -n llm

# Check pods
kubectl get pods -n llm
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

> [!IMPORTANT]
> After the Gateway becomes ready, restart the Kuadrant operators to ensure policies are properly enforced.

Wait for Gateway to be ready:

```bash
kubectl wait --for=condition=Programmed gateway maas-default-gateway -n openshift-ingress --timeout=300s
```

Then restart Kuadrant operators:

```bash
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

#### Update Limitador Image for Metrics (Optional but Recommended)

Update Limitador to expose metrics properly:

```bash
kubectl -n kuadrant-system patch limitador limitador --type merge \
  -p '{"spec":{"image":"quay.io/kuadrant/limitador:1a28eac1b42c63658a291056a62b5d940596fd4c","version":""}}'
```

#### Ensure the correct audience is set for AuthPolicy

Patch `AuthPolicy` with the correct audience for Openshift Identities:

```shell
AUD="$(kubectl create token default --duration=10m 2>/dev/null | cut -d. -f2 | base64 -d 2>/dev/null | jq -r '.aud[0]' 2>/dev/null)"

echo "Patching AuthPolicy with audience: $AUD"

kubectl patch authpolicy maas-api-auth-policy -n maas-api \
  --type='json' \
  -p "$(jq -nc --arg aud "$AUD" '[{
    op:"replace",
    path:"/spec/rules/authentication/openshift-identities/kubernetesTokenReview/audiences/0",
    value:$aud
  }]')"

```
## Testing the Deployment

### 1. Get Gateway Endpoint

For OpenShift:
```bash
HOST="$(kubectl get gateway maas-default-gateway -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"
```

For Kubernetes with LoadBalancer:
```bash
HOST="$(kubectl get gateway maas-default-gateway -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"
```

### 2. Get Authentication Token

For OpenShift:
```bash
TOKEN_RESPONSE=$(curl -sSk \
  -H "Authorization: Bearer $(oc whoami -t)" \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{"expiration": "10m"}' \
  "${HOST}/maas-api/v1/tokens")

TOKEN=$(echo $TOKEN_RESPONSE | jq -r .token)
```

### 3. Test Model Endpoints

For OpenShift deployments, first get the gateway route:

```bash
MODELS=$(curl ${HOST}/maas-api/v1/models  \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" | jq . -r)

echo $MODELS | jq .
MODEL_URL=$(echo $MODELS | jq -r '.data[0].url')
MODEL_NAME=$(echo $MODELS | jq -r '.data[0].id')

echo $MODEL_URL
```

### 4. Test Rate Limiting

Send multiple requests to trigger rate limit:

```bash
for i in {1..16}
do
curl -sSk -o /dev/null -w "%{http_code}\n" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
        \"model\": \"${MODEL_NAME}\",
        \"prompt\": \"Not really understood prompt\",
        \"max_prompts\": 40
    }" \
  "${MODEL_URL}/v1/chat/completions";
done
```

### 5. Verify Complete Deployment

Check that all components are running:

```bash
kubectl get pods -n maas-api
kubectl get pods -n kuadrant-system
kubectl get pods -n kserve
kubectl get pods -n llm
```

Check Gateway status:

```bash
kubectl get gateway -n openshift-ingress maas-default-gateway
```

Check that policies are enforced:

```bash
kubectl get authpolicy -A
kubectl get tokenratelimitpolicy -A

# Check LLMInferenceServices are ready
kubectl get llminferenceservices -n llm
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

Check all relevant pods:

```bash
kubectl get pods -A | grep -E "maas-api|kserve|kuadrant|simulator|qwen"
```

Check services:

```bash
kubectl get svc -A | grep -E "maas-api|simulator|qwen"
```

Check HTTPRoutes and Gateway:

```bash
kubectl get httproute -A
kubectl get gateway -A
```

### View Logs

View MaaS API logs:

```bash
kubectl logs -n maas-api -l app=maas-api --tail=50
```

View Kuadrant logs:

```bash
kubectl logs -n kuadrant-system -l app=kuadrant --tail=50
```

View Model logs:

```bash
kubectl logs -n llm -l component=predictor --tail=50
```

### Common Issues

1. **OOMKilled during model download**: Increase storage initializer memory limits
2. **GPU models not scheduling**: Ensure nodes have `nvidia.com/gpu` resources
3. **Rate limiting not working**: Verify AuthPolicy and TokenRateLimitPolicy are applied
4. **Routes not accessible**: Check Gateway status and HTTPRoute configuration
5. **Kuadrant installation fails with CRD errors**: The deployment script now automatically cleans up leftover CRDs from previous installations
6. **TokenRateLimitPolicy MissingDependency error**: 
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
     2. Create a Service Mesh control plane (Istio instance):
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

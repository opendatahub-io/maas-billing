# Installation Guide

This guide provides complete instructions for deploying the MaaS Platform infrastructure.

## Prerequisites

- **OpenShift cluster** (4.19.9+) with kubectl/oc access
  - **Reccomended** 16 vCPUs, 32GB RAM, 100GB storage
- **ODH/RHOAI requirements**:
    - KServe enabled in DataScienceCluster (RawDeployment mode enabled)
    - Service Mesh installed (automatically installed with ODH/RHOAI)
- **Cluster admin** or equivalent permissions
- **Required tools**:
  - `oc` (OpenShift CLI)
  - `kubectl`
  - `jq`
  - `kustomize` (v5.7.0+)

## Quick Start

### Automated OpenShift Deployment (Recommended)

For OpenShift clusters, use the automated deployment script:

```bash
./deployment/scripts/deploy-openshift.sh
```

### Verify Deployment

The deployment script creates the following core resources:

- **Namespaces**: `maas-api`, `kuadrant-system`, `kserve`, `opendatahub`, `llm`
- **Gateway**: `maas-default-gateway` in `openshift-ingress` namespace
- **HTTPRoutes**: `maas-api-route` in the `openshift-ingress` namespace
- **Policies**: `AuthPolicy`, `TokenRateLimitPolicy`, `RateLimitPolicy`, `TelemetryPolicy`
- **MaaS API**: Deployment and service in `maas-api` namespace
- **Operators**: Kuadrant, Authorino, Limitador in `kuadrant-system` namespace

Check deployment status:

```bash
# Check all namespaces
kubectl get ns | grep -E "maas-api|kuadrant|kserve|opendatahub|llm"

# Check Gateway status
kubectl get gateway -n openshift-ingress maas-default-gateway

# Check policies
kubectl get authpolicy -A
kubectl get tokenratelimitpolicy -A
kubectl get ratelimitpolicy -A

# Check MaaS API
kubectl get pods -n maas-api
kubectl get svc -n maas-api

# Check Kuadrant operators
kubectl get pods -n kuadrant-system

# Check KServe (if deployed)
kubectl get pods -n kserve
kubectl get pods -n opendatahub
```

### Model Setup (Optional)

#### Deploy Sample Models (Optional)

!!! note
    These models use KServe's `LLMInferenceService` custom resource, which requires ODH/RHOAI with KServe enabled.

**Simulator Model (CPU)**
```bash
PROJECT_DIR=$(git rev-parse --show-toplevel)
kustomize build ${PROJECT_DIR}/docs/samples/models/simulator/ | kubectl apply -f -
```

**Facebook OPT-125M Model (CPU)**
```bash
PROJECT_DIR=$(git rev-parse --show-toplevel)
kustomize build ${PROJECT_DIR}/docs/samples/models/facebook-opt-125m-cpu/ | kubectl apply -f -
```

**Qwen3 Model (GPU Required)**

!!! warning
    This model requires GPU nodes with `nvidia.com/gpu` resources available in your cluster.

```bash
PROJECT_DIR=$(git rev-parse --show-toplevel)
kustomize build ${PROJECT_DIR}/docs/samples/models/qwen3/ | kubectl apply -f -
```

**Verify Model Deployment**
```bash
# Check LLMInferenceService status
kubectl get llminferenceservices -n llm

# Check pods
kubectl get pods -n llm
```

#### Update Existing Models (Optional)

To update an existing model, modify the `LLMInferenceService` to use the newly created `maas-default-gateway` gateway.

```bash
kubectl patch llminferenceservice my-production-model -n llm --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/gateway/refs/-",
    "value": {
      "name": "maas-default-gateway",
      "namespace": "openshift-ingress"
    }
  }
]'
```

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: LLMInferenceService
metadata:
  name: my-production-model
spec:
  gateway:
    refs:
      - name: maas-default-gateway
        namespace: openshift-ingress
```

## Testing the Deployment

### Automated Validation (Recommended)

The easiest way to validate your deployment is to use the automated validation script (assumes model is deployed):

```bash
./deployment/scripts/validate-deployment.sh
```

The script provides detailed feedback with specific suggestions for fixing any issues found.

### Manual Testing Steps

If you prefer to test manually or troubleshoot specific components, follow these steps:

#### 1. Get Gateway Endpoint

For OpenShift:
```bash
HOST="$(kubectl get gateway maas-default-gateway -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"
```

#### 2. Get Authentication Token

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

#### 3. Test Model Endpoints

Get available models:
```bash
MODELS=$(curl ${HOST}/maas-api/v1/models  \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" | jq . -r)

echo $MODELS | jq .
MODEL_URL=$(echo $MODELS | jq -r '.data[0].url')
MODEL_NAME=$(echo $MODELS | jq -r '.data[0].id')
```

#### 4. Test Rate Limiting

Send multiple requests to trigger rate limit (should get 200 OK followed by 429 Rate Limit Exceeded):
```bash
for i in {1..16}; do
  curl -sSk -o /dev/null -w "%{http_code}\n" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"model\": \"${MODEL_NAME}\", \"prompt\": \"Hello\", \"max_tokens\": 50}" \
    "${MODEL_URL}/v1/chat/completions"
done
```

#### 5. Verify Complete Deployment

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
kubectl get llminferenceservices -n llm
```

See the [deployment scripts documentation](../../deployment/scripts/README.md) for more information about validation and troubleshooting.

## Troubleshooting

### Common Issues

1. **Getting `501` Not Implemented errors**: Traffic is not making it to the Gateway.
      - [ ] Verify Gateway status and HTTPRoute configuration
2. **Getting `401` Unauthorized errors when trying to get a token**: Authentication maas-api is not working.
      - [ ] Verify `maas-api-auth-policy` AuthPolicy is applied
      - [ ] Validate the AuthPolicy audience matches the token audience (audiences: ["https://kubernetes.default.svc", "maas-default-gateway-sa"])
3. **Getting `401` errors when trying to get models**: Authentication is not working for the models endpoint.
      - [ ] Create a new token (default expiration is 10 minutes)
      - [ ] Verify `gateway-auth-policy` AuthPolicy is applied
      - [ ] Validate that `system:serviceaccounts:maas-default-gateway-tier-{TIER}` has `post` access to the `llminferenceservices` resource
        - Note: this should be automated by the ODH Controller
4. **Getting `404` errors when trying to get models**: The models endpoint is not working.
      - [ ] Verify `model-route` HTTPRoute exist and is applied
      - [ ] Verify the model is deployed and the `LLMInferenceService` has the `maas-default-gateway` gateway specified
      - [ ] Verify that the model is recognized by maas-api by checking the `maas-api/v1/models` endpoint ([here](#3-test-model-endpoints))
5. **Rate limiting not working**: Verify AuthPolicy and TokenRateLimitPolicy are applied
      - [ ] Verify `gateway-rate-limits` RateLimitPolicy is applied
      - [ ] Verify `gateway-token-rate-limits` TokenRateLimitPolicy is applied
      - [ ] Verify the model is deployed and the `LLMInferenceService` has the `maas-default-gateway` gateway specified
      - [ ] Verify that the model is rate limited by checking the `maas-api/v1/models` endpoint ([here](#4-test-rate-limiting))
      - [ ] Verify that the model is token rate limited by checking the `maas-api/v1/models` endpoint ([here](#4-test-rate-limiting))

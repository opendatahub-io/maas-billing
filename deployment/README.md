# Deployment Guide

This guide provides instructions for deploying the MaaS Platform infrastructure and applications.

## Prerequisites

- **OpenShift cluster (4.19.9+)** with **kubectl** or **oc** access

- **Required CLI Tools**:
  - **jq**: Command-line JSON processor
  - **kustomize**: Kubernetes manifests customization tool (v5.7.0+)
  - **git**: Version control system

- **ODH/RHOAI requirements**:
  - KServe enabled in DataScienceCluster
  - Service Mesh installed (automatically installed with ODH/RHOAI)

## Important Notes

- This project assumes OpenDataHub (ODH) or Red Hat OpenShift AI (RHOAI) as the base platform
- KServe components are expected to be provided by ODH/RHOAI, not installed separately
- For non-ODH/RHOAI deployments, KServe can be optionally installed from `deployment/components/kserve`

> [!NOTE]
> **Important:** For the KServe section of the ODH operator, you must set `defaultDeploymentMode: RawDeployment` and ensure that the serving management state is set to `Removed`.  
>  
> Example configuration:
> 
> ```yaml
> kserve:
>   nim:
>     managementState: Managed
>   rawDeploymentServiceConfig: Headless
>   serving:
>     ingressGateway:
>       certificate:
>         type: OpenshiftDefaultIngress
>     managementState: Removed
>     name: knative-serving
>   managementState: Managed
>   defaultDeploymentMode: RawDeployment
> ```

## Quick Start

### Automated OpenShift Deployment (Recommended)

For OpenShift clusters, use the automated deployment script:
```bash
./deployment/scripts/deploy-openshift.sh
```

This script handles all steps including feature gates, dependencies, and OpenShift-specific configurations.

### 📊 Monitoring Dashboard

After deployment, you can import the Grafana dashboard for monitoring:

1. **Dashboard Location:** `docs/samples/dashboards/maas-token-metrics-dashboard.json`
2. **Import into Grafana:** Upload the JSON file to your Grafana instance
3. **Configure Prometheus:** Ensure your Prometheus datasource is configured
4. **View Metrics:** Monitor token usage, rate limiting, and tier-based analytics

See [Dashboard Documentation](../../docs/samples/dashboards/README.md) for detailed setup instructions.

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

## Validation Steps

### Automated Validation (Recommended)

The easiest way to validate your deployment is to use the automated validation script:

```bash
./deployment/scripts/validate-deployment.sh
```

This script will automatically check:
- ✅ All component pods are running
- ✅ Gateway is configured and ready
- ✅ Policies are enforced (AuthPolicy, TokenRateLimitPolicy)
- ✅ API endpoints are accessible
- ✅ Authentication is working
- ✅ Rate limiting is enforced
- ✅ Authorization is enforced (401 without token)

The script provides detailed feedback with specific suggestions for fixing any issues found.

### Manual Validation Steps

If you prefer to validate manually or troubleshoot specific components, follow these steps:

#### 1. Get Gateway Endpoint

```bash
CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
HOST="https://maas.${CLUSTER_DOMAIN}"
```

**Note:** If you haven't created the `maas-default-gateway` yet, you can use the fallback:
```bash
HOST="https://gateway.${CLUSTER_DOMAIN}"
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

#### 3. List Available Models

```bash
MODELS=$(curl -sSk ${HOST}/maas-api/v1/models \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" | jq -r .)

echo $MODELS | jq .
MODEL_NAME=$(echo $MODELS | jq -r '.data[0].id')
# Get the full URL which includes the LLMInferenceService resource name in the path
MODEL_URL=$(echo $MODELS | jq -r '.data[0].url')

echo "Model URL: $MODEL_URL"
```

#### 4. Test Model Inference Endpoint

Send a request to the model endpoint (should get a 200 OK response):

```bash
curl -sSk -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"model\": \"${MODEL_NAME}\", \"prompt\": \"Hello\", \"max_tokens\": 50}" \
  "${MODEL_URL}"
```

#### 5. Test Authorization Enforcement

Send a request to the model endpoint without a token (should get a 401 Unauthorized response):

```bash
curl -sSk -H "Content-Type: application/json" \
  -d "{\"model\": \"${MODEL_NAME}\", \"prompt\": \"Hello\", \"max_tokens\": 50}" \
  "${MODEL_URL}" -v
```

#### 6. Test Rate Limiting

Send multiple requests to trigger rate limit (should get 200 OK followed by 429 Rate Limit Exceeded after about 4 requests):

```bash
for i in {1..16}; do
  curl -sSk -o /dev/null -w "%{http_code}\n" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"model\": \"${MODEL_NAME}\", \"prompt\": \"Hello\", \"max_tokens\": 50}" \
    "${MODEL_URL}"
done
```

#### 7. Verify Component Status

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

---

**💡 Tip:** Instead of running these manual validation steps, you can use the automated validation script which performs all these checks and more:

```bash
./deployment/scripts/validate-deployment.sh
```

The script provides detailed feedback with color-coded results and specific suggestions for fixing any issues found. See [deployment/scripts/README.md](scripts/README.md) for more information.

---

### Common Issues

1. **Rate limiting not working**: Verify AuthPolicy and TokenRateLimitPolicy exist and are enforce
2. **Routes not accessible (503 errors)**: Check Maas Default Gateway status and HTTPRoute configuration
3. **Kuadrant installation fails with CRD errors**: The deployment script now automatically cleans up leftover CRDs from previous installations

## Next Steps

After deploying the infrastructure:

1. **Start the development environment**: See the main [README](../README.md) for frontend/backend setup
2. **Deploy additional models**: Check [samples/models](samples/models/) for more examples
3. **Configure monitoring**: Enable observability components in overlays 

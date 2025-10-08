# Installation Guide

This guide provides complete instructions for deploying the MaaS Platform infrastructure and applications on OpenShift.

## 📹 Video Walkthrough

> [!TIP]
> **Watch the installation process in action!**
> 
> For a visual guide to the installation process, watch our step-by-step video walkthrough:
> 
> <!-- TODO: Add video embed once uploaded -->
> **[Installation Video Walkthrough]** _(Coming Soon)_
> 
> The video covers:
> - Prerequisites verification
> - Automated deployment using the deploy-openshift.sh script
> - Manual deployment steps
> - Testing and verification
> - Common troubleshooting scenarios

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
  - `kustomize`

## Quick Start

### Automated OpenShift Deployment (Recommended)

For OpenShift clusters, use the automated deployment script:

```bash
./deployment/scripts/deploy-openshift.sh
```

This script handles all steps including feature gates, dependencies, and OpenShift-specific configurations.

> [!NOTE]
> **If you encounter authentication errors when testing the deployment, you may need to patch the `AuthPolicy` with the correct audience for OpenShift Identities.**
> 
> Run the following commands to retrieve the correct audience and patch the `AuthPolicy`:
> 
> ```bash
> PROJECT_DIR=$(git rev-parse --show-toplevel)
> AUD="$(kubectl create token default --duration=10m \
>   | jwt decode --json - \
>   | jq -r '.payload.aud[0]')"
> 
> echo "Patching AuthPolicy with audience: $AUD"
> 
> kubectl patch authpolicy maas-api-auth-policy -n maas-api \
>   --type='json' \
>   -p "$(jq -nc --arg aud "$AUD" '[{
>     op:"replace",
>     path:"/spec/rules/authentication/openshift-identities/kubernetesTokenReview/audiences/0",
>     value:$aud
>   }]')"
> ```

For manual deployment, see the [Manual Deployment Steps](deployment/README.md#manual-deployment-steps) in the deployment README.

#### Deploy Sample Models (Optional)

> [!NOTE]
> These models use KServe's `LLMInferenceService` custom resource, which requires ODH/RHOAI with KServe enabled.

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

> [!WARNING]
> This model requires GPU nodes with `nvidia.com/gpu` resources available in your cluster.

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

## Testing the Deployment

### 1. Get Gateway Endpoint

For OpenShift:
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

Get available models:
```bash
MODELS=$(curl ${HOST}/maas-api/v1/models  \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" | jq . -r)

echo $MODELS | jq .
MODEL_URL=$(echo $MODELS | jq -r '.data[0].url')
MODEL_NAME=$(echo $MODELS | jq -r '.data[0].id')
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
7. **Gateway stuck in "Waiting for controller" on OpenShift**:
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

## Next Steps

After deploying the infrastructure:

1. **Configure tiers**: See [Tier Management](tier-management.md) for access control setup
2. **Set up monitoring**: Enable observability components in overlays
3. **Start development**: See the main [README](https://github.com/redhat-ai-ml/maas-billing/blob/main/README.md) github page setup

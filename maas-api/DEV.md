## Environment Setup

### Prerequisites

- kubectl
- jq
- kustomize 5.7
- OCP 4.19.9+ (for GW API)
- [jwt](https://github.com/mike-engel/jwt-cli) CLI tool (for inspecting tokens)

### Setup

### Core Infrastructure

First, we need to deploy the core infrastructure. That includes:
- Kuadrant
- Cert Manager

> [!IMPORTANT]
> If you are running RHOAI, both Kuadrant and Cert Manager should be already installed.

```shell
PROJECT_DIR=$(git rev-parse --show-toplevel) 
for ns in opendatahub kuadrant-system llm maas-api; do kubectl create ns $ns || true; done
"${PROJECT_DIR}/deployment/scripts/install-dependencies.sh" --cert-manager --kuadrant 
```
#### Enabling GW API

> [!IMPORTANT]
> For enabling Gateway API on OCP 4.19.9+, only GatewayClass creation is needed.

```shell
PROJECT_DIR=$(git rev-parse --show-toplevel)
kustomize build ${PROJECT_DIR}/maas-api/deploy/infra/openshift-gateway-api | kubectl apply --server-side=true --force-conflicts -f -
```

### Deploying Opendatahub KServe

```shell
PROJECT_DIR=$(git rev-parse --show-toplevel)
kustomize build ${PROJECT_DIR}/maas-api/deploy/infra/odh | kubectl apply --server-side=true --force-conflicts -f -
```

> [!NOTE]
> If it fails the first time, simply re-run. CRDs or Webhooks might not be established timely.
> This approach is aligned with how odh-operator would process (requeue reconciliation).

### Deploying MaaS API for development

```shell
make deploy-dev
```

This will:
- Deploy MaaS API component with Service Account Token provider
- Set up demo policies (see `deploy/policies`)

#### Patch Kuadrant deployment

> [!IMPORTANT]
> See https://docs.redhat.com/en/documentation/red_hat_connectivity_link/1.1/html/release_notes_for_connectivity_link_1.1/prodname-relnotes_rhcl#connectivity_link_known_issues

If you installed Kuadrant using Helm chats (i.e. by calling `./install-dependencies.sh --kuadrant` like in the example above),
you need to patch the Kuadrant deployment to add the correct environment variable.

```shell
kubectl -n kuadrant-system patch deployment kuadrant-operator-controller-manager \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ISTIO_GATEWAY_CONTROLLER_NAMES","value":"openshift.io/gateway-controller/v1"}}]'
```

If you installed Kuadrant using OLM, you have to patch `ClusterServiceVersion` instead, to add the correct environment variable.

```shell
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

#### Ensure the correct audience is set for AuthPolicy

Patch `AuthPolicy` with the correct audience for Openshift Identities:

```shell
PROJECT_DIR=$(git rev-parse --show-toplevel)
AUD="$(kubectl create token default --duration=10m \
  | jwt decode --json - \
  | jq -r '.payload.aud[0]')"

echo "Patching AuthPolicy with audience: $AUD"

kubectl patch --local -f ${PROJECT_DIR}/maas-api/deploy/policies/maas-api/auth-policy.yaml \
  --type='json' \
  -p "$(jq -nc --arg aud "$AUD" '[{
    op:"replace",
    path:"/spec/rules/authentication/openshift-identities/kubernetesTokenReview/audiences/0",
    value:$aud
  }]')" \
  -o yaml | kubectl apply -f -
```

#### Update Limitador image to expose metrics

Update the Limitador deployment to use the latest image that exposes metrics:

```shell
NS=kuadrant-system
kubectl -n $NS patch limitador limitador --type merge \
  -p '{"spec":{"image":"quay.io/kuadrant/limitador:1a28eac1b42c63658a291056a62b5d940596fd4c","version":""}}'
```

### Testing

#### Deploying the demo model

```shell
PROJECT_DIR=$(git rev-parse --show-toplevel)
kustomize build ${PROJECT_DIR}/maas-api/deploy/models/simulator | kubectl apply --server-side=true --force-conflicts -f -
```

#### Getting the token

To see the token, you can use the following commands:

```shell
HOST="$(kubectl get gateway openshift-ai-inference -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"

TOKEN_RESPONSE=$(curl -sSk \
  -H "Authorization: Bearer $(oc whoami -t)" \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{
    "expiration": "4h"
  }' \
  "${HOST}/maas-api/v1/tokens")

echo $TOKEN_RESPONSE | jq -r .
echo $TOKEN_RESPONSE | jq -r .token | jwt decode --json -

TOKEN=$(echo $TOKEN_RESPONSE | jq -r .token)
```
> [!NOTE]
> This is a self-service endpoint that issues ephemeral tokens. Openshift Identity (`$(oc whoami -t)`) is used as a refresh token.

#### Calling the model and hitting the rate limit

Using model discovery:

```shell
HOST="$(kubectl get gateway openshift-ai-inference -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"

MODELS=$(curl ${HOST}/maas-api/v1/models  \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" | jq . -r)

echo $MODELS | jq .
MODEL_URL=$(echo $MODELS | jq -r '.data[0].url')
MODEL_NAME=$(echo $MODELS | jq -r '.data[0].id')

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

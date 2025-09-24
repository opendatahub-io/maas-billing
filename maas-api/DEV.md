## Setting up dev environment

### Prerequisites

- kubectl
- jq
- kustomize
- OCP 4.19.9+ (for GW API)
- [jwt](https://github.com/mike-engel/jwt-cli) CLI tool (for inspecting tokens)

### Setup

First, we need to deploy the core infrastructure:

```shell
ROOT=$(git rev-parse --show-toplevel)
for ns in kserve kuadrant llm maas-api; do kubectl create ns $ns || true; done
kustomize build ${ROOT}/deployment/default/odh | kubectl apply -f -
cd ${ROOT} && ./deployment/scripts/install-dependencies.sh --cert-manager --kserve --kuadrant && cd -
kustomize build ${ROOT}/deployment/base/maas-api | kubectl apply -f -
kustomize build --load-restrictor LoadRestrictionsNone ${ROOT}/deployment/samples/models/simulator | kubectl apply -f -
```

For GPU-based model deployment (requires GPU nodes):

```shell
kustomize build --load-restrictor LoadRestrictionsNone ${ROOT}/deployment/samples/models/qwen3 | kubectl apply -f -
```

> [!IMPORTANT]
> The model YAML files in `deploy/overlays/dev/models/` are symlinks, therefore, they need to be built with --load-restrictor LoadRestrictionsNone.
> For more details see this [issue](https://github.com/kubernetes-sigs/kustomize/issues/4420).
> 
> [!NOTE]
> The Qwen3 model requires:
> - GPU resources (nvidia.com/gpu) and will only schedule on nodes with GPU support
> - Sufficient storage initializer resources (4Gi memory minimum) to download model weights
> - The KServe inferenceservice-config ConfigMap is pre-configured with appropriate storage initializer resources

#### Storage Initializer Configuration

The KServe storage initializer requires sufficient resources to download large models. The default configuration in `deployment/default/odh/kserve-config-openshift.yaml` is set to:

- Memory Request: 4Gi
- Memory Limit: 8Gi
- CPU Request: 2
- CPU Limit: 4

These values are sufficient for most models including Qwen3. If you encounter OOMKilled errors during model download, you may need to increase these limits by editing the ConfigMap:

```shell
kubectl edit configmap inferenceservice-config -n kserve
```

#### Patch Kuadrant deployment

If you installed Kuadrant using Helm chats (i.e. by calling `./install-dependencies.sh --kuadrant` like in the example above), you need to patch the Kuadrant deployment to add the correct environment variable.

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

#####
#### Update KServe configmap with the actual ingress domain

```shell
kubectl -n kserve patch configmap inferenceservice-config \
  --type='json' \
  -p="$(cat <<EOF
[
  {
    "op":"replace",
    "path":"/data/ingress",
    "value":"{
  \"enableGatewayApi\": true,
  \"kserveIngressGateway\": \"openshift-ingress/openshift-ai-inference\",
  \"ingressGateway\": \"istio-system/istio-ingressgateway\",
  \"ingressDomain\": \"$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')\"
}"
  }
]
EOF
)"
```

#### Ensure the correct audience is set for AuthPolicy

Patch `AuthPolicy` with the correct audience for Openshift Identities

```shell
AUD="$(kubectl create token default --duration=10m \
  | jwt decode --json - \
  | jq -r '.payload.aud[0]')"

kubectl patch --local -f ${ROOT}/deployment/base/policies/auth-policy.yaml \
  --type='json' \
  -p "$(jq -nc --arg aud "$AUD" '[{
    op:"replace",
    path:"/spec/rules/authentication/openshift-identities/kubernetesTokenReview/audiences",
    value:[$aud]
  }]')" \
  -o yaml | kubectl apply -f -
```

### Testing

#### Getting the token

To see the token, you can use the following commands:

```shell
HOST="$(kubectl get gateway openshift-ai-inference -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"

TOKEN_RESPONSE=$(curl -sSk \
  -H "Authorization: Bearer $(oc whoami -t)" \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{
    "expiration": "10m"
  }' \
  "${HOST}/maas-api/v1/tokens")

echo $TOKEN_RESPONSE | jq -r .
echo $TOKEN_RESPONSE | jq -r .token | jwt decode --json -

TOKEN=$(echo $TOKEN_RESPONSE | jq -r .token)
```
> [!NOTE]
> This is a self-service endpoint that issues ephemeral tokens. Openshift Identity (`$(oc whoami -t)`) is used as a refresh token.

#### Calling the model and hitting the rate limit

```shell
for i in {1..16}
do
curl -ks -o /dev/null -w "%{http_code}\n" \
  -H "Authorization: Bearer $TOKEN" \
  "${HOST}/simulator/health";
done;
```

#### Testing GPU Models (Qwen3)

If you have deployed the Qwen3 model (requires GPU nodes), you can test it:

```shell
# Test health endpoint
curl -ks -o /dev/null -w "%{http_code}\n" \
  -H "Authorization: Bearer $TOKEN" \
  "${HOST}/qwen3/health"

# Test completion endpoint
curl -ks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -X POST \
  "${HOST}/qwen3/v1/completions" \
  -d '{
    "model": "qwen3-0-6b-instruct",
    "prompt": "Hello, how are you?",
    "max_tokens": 50
  }' | jq .
```
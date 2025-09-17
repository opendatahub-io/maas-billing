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
kustomize build ${ROOT}/deployment/infrastructure/kustomize-templates/kserve | kubectl apply -f -
cd ${ROOT} && ./deployment/scripts/install-dependencies.sh --cert-manager --kserve --kuadrant && cd -
make deploy-dev \
  -e REPO=quay.io/bmajsak/maas-api \
  -e TAG=sa-token-provider \
  -e PRE_DEPLOY_STEP='kustomize edit add patch --group apps --kind Deployment --path patches/sa-token-provider.yaml'
kustomize build --load-restrictor LoadRestrictionsNone deploy/overlays/dev/models/simulator | kubectl apply -f -
```

> [!IMPORTANT]
> `vllm-simulator.yaml` in `deploy/overlays/dev/models/simulator` is a symlink, therefore, it needs to be built with --load-restrictor LoadRestrictionsNone.
> For more details see this [issue](https://github.com/kubernetes-sigs/kustomize/issues/4420).

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

kubectl patch --local -f deploy/overlays/dev/policies/auth-policy.yaml \
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
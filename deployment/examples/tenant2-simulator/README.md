# Tenant onboarding example — connect an existing model/serving runtime to the MaaS Gateway (API-key auth + rate limits)

This walkthrough shows how to hook up an **existing** model/serving runtime (e.g., KServe) in a tenant namespace to the centralized MaaS platform: attach a tenant host to the **shared Gateway** via an `HTTPRoute`, then apply **AuthPolicy** and **TokenRateLimitPolicy**.

> [!NOTE]
> This flow will be automated in future releases based on annotations. The manual steps below are kept for clarity and troubleshooting.

## What’s shared (cluster-wide)
- Istio (and `GatewayClass istio`)
- Gateway API CRDs
- cert-manager
- KServe controllers/CRDs
- Kuadrant controllers/CRDs (Authorino + Limitador)

> These are installed once and reused by all tenants. You don’t reinstall them here.

> **Prerequisite – existing Serving Runtime**
> A model/serving runtime (e.g., KServe) is already running in `${TENANT_NS}` and exposes a
> Service `${BACKEND_SVC}` (Service port 80). This example only wires that Service to the
> shared MaaS Gateway and applies Auth/RateLimit policies.

## What this example creates (tenant-owned)
- Namespace `${TENANT_NS}` (created if needed)
- An **HTTPRoute** in `${TENANT_NS}` that attaches host `${HOST}` to the **shared Gateway**
  `${GATEWAY_NAME}` in `${GATEWAY_NS}` and routes to `${BACKEND_SVC}`
- A public **OpenShift Route** (`simulator-route`) in `${GATEWAY_NS}` publishing the shared gateway


> **Policies**
> AuthN and rate-limit policies are attached to the **shared Gateway** `${GATEWAY_NAME}` in
> `${GATEWAY_NS}`. In **Step 3** we render the repo’s Kuadrant resources and retarget them to
> that Gateway (namespace/name), then `oc apply -f -`.

---

## Step 0: Set environment variables (once per terminal)
```bash
export GATEWAY_NS=${GATEWAY_NS:=llm-tenant}
export GATEWAY_NAME=${GATEWAY_NAME:=inference-gateway}
export TENANT_NS=${TENANT_NS:=llm-tenant2}
export CLUSTER_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
export HOST="simulator-${TENANT_NS}.${CLUSTER_DOMAIN}"
export BACKEND_SVC=${BACKEND_SVC:=vllm-simulator-predictor}

echo "TENANT_NS=$TENANT_NS"
echo "HOST=$HOST"
echo "GATEWAY_NS=$GATEWAY_NS"
echo "GATEWAY_NAME=$GATEWAY_NAME"
echo "BACKEND_SVC=$BACKEND_SVC"
```

## Step 1: Apply tenant resources (no envsubst; use sed on the fly)
```bash
cd deployment/examples/tenant2-simulator

# Create the namespace
oc create ns "$TENANT_NS" || true

# HTTPRoute: attach tenant host to the shared Gateway and route to an existing Service
sed -e "s|\${HOST}|$HOST|g" \
    -e "s|\${GATEWAY_NS}|$GATEWAY_NS|g" \
    -e "s|\${GATEWAY_NAME}|$GATEWAY_NAME|g" \
    -e "s|\${BACKEND_SVC}|$BACKEND_SVC|g" \
    4-httproute.yaml | oc -n "$TENANT_NS" apply -f -

# Public exposure via OpenShift Route (apply in GATEWAY_NS)
sed -e "s|\${HOST}|$HOST|g" 5-route.yaml | oc -n "$GATEWAY_NS" apply -f -
```
## Step 2: Check what got created
```bash
oc -n "$TENANT_NS"  get httproute simulator-domain-route -o wide
oc -n "$GATEWAY_NS" get route      simulator-route       -o wide
oc -n "$TENANT_NS"  get svc        "$BACKEND_SVC"        -o wide
```

## Step 3: Attach AuthPolicy + TokenRateLimitPolicy (retargeted)
```bash
cd "$(git rev-parse --show-toplevel)"

kustomize build deployment/overlays/openshift | yq '
  select(
    (.kind == "AuthPolicy" and .spec.targetRef.kind == "Gateway") or
    (.kind == "TokenRateLimitPolicy")
  )
  | .metadata.namespace       = env(GATEWAY_NS)
  | .spec.targetRef.namespace = env(GATEWAY_NS)
  | .spec.targetRef.name      = env(GATEWAY_NAME)
' | oc apply -f -

oc -n "$GATEWAY_NS" get authpolicy -o wide
oc -n "$GATEWAY_NS" get tokenratelimitpolicy -o wide

```

## Step 4: Call with API keys
```bash
SIM_HOST=$(oc -n "$GATEWAY_NS" get route simulator-route -o jsonpath='{.spec.host}')

# Tip: if you're not sure of the secret name, list available API-key secrets on the shared gateway:
# oc -n "$GATEWAY_NS" get secret -l 'kuadrant.io/auth-secret=true' -o name
# (Optional) auto-pick the first one:
# KEY=$(oc -n "$GATEWAY_NS" get secret -l 'kuadrant.io/auth-secret=true' -o jsonpath='{.items[0].metadata.name}')
# FREE=$(oc -n "$GATEWAY_NS" get secret "$KEY" -o jsonpath='{.data.api_key}' | base64 -d)

# If you know the secret name:
FREE=$(oc -n "$GATEWAY_NS" get secret freeuser1-apikey -o jsonpath='{.data.api_key}' | base64 -d)

curl -i \
  -H "Authorization: APIKEY ${FREE}" \
  -H "Content-Type: application/json" \
  --data '{"model":"simulator-model","messages":[{"role":"user","content":"Hello!"}]}' \
  "http://${SIM_HOST}/v1/chat/completions"

# Burst to see rate limit (expect some 429 on free tier)
for i in {1..15}; do
  code=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: APIKEY ${FREE}" \
    -H "Content-Type: application/json" \
    --data '{"model":"simulator-model","messages":[{"role":"user","content":"Hello"}],"max_tokens":10}' \
    "http://${SIM_HOST}/v1/chat/completions"); echo "req #$i -> $code"
done
```

## Step 5: Clean up
```bash
# Route lives in the shared gateway namespace
oc -n "$GATEWAY_NS" delete route simulator-route || true

# HTTPRoute lives in the tenant namespace
oc -n "$TENANT_NS" delete httproute simulator-domain-route || true

# No tenant secrets/gateway to delete
oc delete ns "$TENANT_NS" || true
```
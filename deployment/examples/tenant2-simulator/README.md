# Tenant example: `llm-tenant2` — simulator model + tenant Gateway + API-key auth + rate limits

This example shows how to stand up a **second tenant** (`llm-tenant2`) on a cluster that already
has the shared MaaS platform bits installed (**Istio/Gateway API, cert-manager, KServe, Kuadrant**).

## What’s shared (cluster-wide)
- Istio (and `GatewayClass istio`)
- Gateway API CRDs
- cert-manager
- KServe controllers/CRDs
- Kuadrant controllers/CRDs (Authorino + Limitador)

> These are installed once and reused by all tenants. You don’t reinstall them here.

## What this example creates (tenant-owned)
- Namespace `llm-tenant2`
- A tiny **simulator** model via KServe (custom container)
- A **Gateway + HTTPRoute** scoped to the tenant
- A public **OpenShift Route** pointing to the tenant gateway service
- **API-key** Secrets for `free` and `premium` users

> AuthN / rate-limit **policies** are attached at the tenant Gateway. We reuse the repo’s policy
manifests by retargeting them to `llm-tenant2` (see Step 4).

---

## Step 0: Set environment variables (once per terminal)
```bash
export TENANT_NS=${TENANT_NS:=llm-tenant2}
export CLUSTER_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
export HOST="simulator-${TENANT_NS}.${CLUSTER_DOMAIN}"

echo "TENANT_NS=$TENANT_NS"
echo "HOST=$HOST"
```

## Step 1: Apply tenant resources (no envsubst; use sed on the fly)
```bash
cd deployment/examples/tenant2-simulator

# Create the namespace
oc create ns "$TENANT_NS" || true

# App: tiny simulator (ConfigMap + KServe InferenceService)
oc -n "$TENANT_NS" apply -f 1-sim-app-configmap.yaml
oc -n "$TENANT_NS" apply -f 2-simulator-isvc.yaml

# Tenant Gateway + HTTPRoute (replace ${HOST} while applying)
sed -e "s|\${HOST}|$HOST|g" 3-gateway.yaml   | oc -n "$TENANT_NS" apply -f -
sed -e "s|\${HOST}|$HOST|g" 4-httproute.yaml | oc -n "$TENANT_NS" apply -f -

# Public exposure via OpenShift Route
sed -e "s|\${HOST}|$HOST|g" 5-route.yaml     | oc -n "$TENANT_NS" apply -f -
```

## Step 2: (Optional) Create sample API keys
```bash
oc -n "$TENANT_NS" apply -f 6-sample-keys.yaml
```

## Step 3: Check what got created
```bash
oc -n "$TENANT_NS" get pods,svc,isvc,gateway,httproute,route
```

## Step 4: Attach AuthPolicy + TokenRateLimitPolicy (retargeted)
```bash
# from repo root (where `deployment/` exists)
cd ../../..

TENANT_NS=${TENANT_NS:=llm-tenant2}
GW_NAME=inference-gateway

kustomize build deployment/overlays/openshift | yq '
  select(
    (.kind == "AuthPolicy" and .spec.targetRef.kind == "Gateway") or
    (.kind == "TokenRateLimitPolicy")
  )
  | .metadata.namespace       = strenv(TENANT_NS)
  | .spec.targetRef.namespace = strenv(TENANT_NS)
  | .spec.targetRef.name      = strenv(GW_NAME)
' | oc apply -f -

oc -n "$TENANT_NS" get authpolicy -o wide
oc -n "$TENANT_NS" get tokenratelimitpolicy -o wide
```

## Step 5: Call with API keys
```bash
SIM_URL=$(oc -n "$TENANT_NS" get route simulator-route -o jsonpath='{.spec.host}')
FREE=$(oc -n "$TENANT_NS" get secret freeuser1-apikey -o jsonpath='{.data.api_key}' | base64 -d)

curl -i \
  -H "Authorization: APIKEY ${FREE}" \
  -H "Content-Type: application/json" \
  --data '{"model":"simulator-model","messages":[{"role":"user","content":"Hello!"}]}' \
  "http://${SIM_URL}/v1/chat/completions"

# Burst to see rate limit (expect some 429 on free tier)
for i in {1..15}; do
  code=$(curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: APIKEY ${FREE}" \
    -H "Content-Type: application/json" \
    --data '{"model":"simulator-model","messages":[{"role":"user","content":"Hello"}],"max_tokens":10}' \
    "http://${SIM_URL}/v1/chat/completions"); echo "req #$i -> $code"
done
```

## Step 6: Clean up
```bash
oc -n "$TENANT_NS" delete route simulator-route || true
oc -n "$TENANT_NS" delete httproute simulator-domain-route || true
oc -n "$TENANT_NS" delete gateway inference-gateway || true
oc -n "$TENANT_NS" delete isvc vllm-simulator || true
oc -n "$TENANT_NS" delete cm sim-app || true
oc -n "$TENANT_NS" delete secret freeuser1-apikey freeuser2-apikey premiumuser1-apikey premiumuser2-apikey || true
oc delete ns "$TENANT_NS" || true
```
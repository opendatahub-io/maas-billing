#!/bin/bash

# OpenShift MaaS Platform Deployment Script
# This script automates the complete deployment of the MaaS platform on OpenShift

set -e

# Helper function to wait for CRD to be established
wait_for_crd() {
  local crd="$1"
  local timeout="${2:-60s}"

  echo "⏳ Waiting for CRD ${crd} to appear (timeout: ${timeout})…"
  if ! timeout "$timeout" bash -c 'until kubectl get crd "$1" &>/dev/null; do sleep 2; done' _ "$crd"; then
    echo "❌ Timed out after $timeout waiting for CRD $crd to appear." >&2
    return 1
  fi

  echo "⏳ CRD ${crd} detected — waiting for it to become Established (timeout: ${timeout})…"
  kubectl wait --for=condition=Established --timeout="$timeout" "crd/$crd"
}

# Helper function to wait for pods in a namespace to be ready
wait_for_pods() {
  local namespace="$1"
  local timeout="${2:-120}"
  
  kubectl get namespace "$namespace" &>/dev/null || return 0
  
  echo "⏳ Waiting for pods in $namespace to be ready..."
  local end=$((SECONDS + timeout))
  while [ $SECONDS -lt $end ]; do
    local not_ready=$(kubectl get pods -n "$namespace" --no-headers 2>/dev/null | grep -v -E 'Running|Completed|Succeeded' | wc -l)
    [ "$not_ready" -eq 0 ] && return 0
    sleep 5
  done
  echo "⚠️  Timeout waiting for pods in $namespace" >&2
  return 1
}

# version_compare <version1> <version2>
#   Compares two version strings in semantic version format (e.g., "4.19.9")
#   Returns 0 if version1 >= version2, 1 otherwise
version_compare() {
  local version1="$1"
  local version2="$2"
  
  local v1=$(echo "$version1" | awk -F. '{printf "%d%03d%03d", $1, $2, $3}')
  local v2=$(echo "$version2" | awk -F. '{printf "%d%03d%03d", $1, $2, $3}')
  
  [ "$v1" -ge "$v2" ]
}

wait_for_validating_webhooks() {
    local namespace="$1"
    local timeout="${2:-60}"
    local interval=2
    local end=$((SECONDS+timeout))

    echo "⏳ Waiting for validating webhooks in namespace $namespace (timeout: $timeout sec)..."

    while [ $SECONDS -lt $end ]; do
        local not_ready=0

        local services
        services=$(kubectl get validatingwebhookconfigurations \
          -o jsonpath='{range .items[*].webhooks[*].clientConfig.service}{.namespace}/{.name}{"\n"}{end}' \
          | grep "^$namespace/" | sort -u)

        if [ -z "$services" ]; then
            echo "⚠️  No validating webhooks found in namespace $namespace"
            return 0
        fi

        for svc in $services; do
            local ns name ready
            ns=$(echo "$svc" | cut -d/ -f1)
            name=$(echo "$svc" | cut -d/ -f2)

            ready=$(kubectl get endpoints -n "$ns" "$name" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true)
            if [ -z "$ready" ]; then
                echo "🔴 Webhook service $ns/$name not ready"
                not_ready=1
            else
                echo "✅ Webhook service $ns/$name has ready endpoints"
            fi
        done

        if [ "$not_ready" -eq 0 ]; then
            echo "🎉 All validating webhook services in $namespace are ready"
            return 0
        fi

        sleep $interval
    done

    echo "❌ Timed out waiting for validating webhooks in $namespace"
    return 1
}

echo "========================================="
echo "🚀 MaaS Platform OpenShift Deployment"
echo "========================================="
echo ""

# Check if running on OpenShift
if ! kubectl api-resources | grep -q "route.openshift.io"; then
    echo "❌ This script is for OpenShift clusters only."
    exit 1
fi

# Check prerequisites
echo "📋 Checking prerequisites..."
echo ""
echo "Required tools:"
echo "  - oc: $(oc version --client --short 2>/dev/null | head -n1 || echo 'not found')"
echo "  - jq: $(jq --version 2>/dev/null || echo 'not found')"
echo "  - kustomize: $(kustomize version --short 2>/dev/null || echo 'not found')"
echo ""
echo "ℹ️  Note: OpenShift Service Mesh should be automatically installed when GatewayClass is created."
echo "   If the Gateway gets stuck in 'Waiting for controller', you may need to manually"
echo "   install the Red Hat OpenShift Service Mesh operator from OperatorHub."

echo ""
echo "1️⃣ Checking OpenShift version and Gateway API requirements..."

# Get OpenShift version
OCP_VERSION=$(oc get clusterversion version -o jsonpath='{.status.desired.version}' 2>/dev/null || echo "unknown")
echo "   OpenShift version: $OCP_VERSION"

# Check if version is 4.19.9 or higher
if [[ "$OCP_VERSION" == "unknown" ]]; then
    echo "   ⚠️  Could not determine OpenShift version, applying feature gates to be safe"
    oc patch featuregate/cluster --type='merge' \
      -p '{"spec":{"featureSet":"CustomNoUpgrade","customNoUpgrade":{"enabled":["GatewayAPI","GatewayAPIController"]}}}' || true
    echo "   Waiting for feature gates to reconcile (30 seconds)..."
    sleep 30
elif version_compare "$OCP_VERSION" "4.19.9"; then
    echo "   ✅ OpenShift $OCP_VERSION supports Gateway API via GatewayClass (no feature gates needed)"
else
    echo "   Applying Gateway API feature gates for OpenShift < 4.19.9"
    oc patch featuregate/cluster --type='merge' \
      -p '{"spec":{"featureSet":"CustomNoUpgrade","customNoUpgrade":{"enabled":["GatewayAPI","GatewayAPIController"]}}}' || true
    echo "   Waiting for feature gates to reconcile (30 seconds)..."
    sleep 30
fi

echo ""
echo "2️⃣ Creating namespaces..."
echo "   ℹ️  Note: If ODH/RHOAI is already installed, some namespaces may already exist"
for ns in opendatahub kserve kuadrant-system llm maas-api; do
    kubectl create namespace $ns 2>/dev/null || echo "   Namespace $ns already exists"
done

echo ""
echo "3️⃣ Installing dependencies..."

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "   Installing cert-manager..."
"$SCRIPT_DIR/install-dependencies.sh" --cert-manager

# Wait for cert-manager CRDs to be ready
echo "   Waiting for cert-manager CRDs to be established..."
wait_for_crd "certificates.cert-manager.io" "120s" || \
    echo "   ⚠️  Certificate CRD not yet available"

# Clean up any leftover Kuadrant CRDs from previous installations
echo "   Checking for leftover Kuadrant CRDs..."
LEFTOVER_CRDS=$(kubectl get crd 2>/dev/null | grep -E "kuadrant|authorino|limitador" | awk '{print $1}')
if [ -n "$LEFTOVER_CRDS" ]; then
    echo "   Found leftover CRDs, cleaning up..."
    echo "$LEFTOVER_CRDS" | xargs -r kubectl delete crd --timeout=30s 2>/dev/null || true
fi

echo "   Installing Kuadrant..."
"$SCRIPT_DIR/install-dependencies.sh" --kuadrant

# Wait for Kuadrant CRDs to be ready
echo "   Waiting for Kuadrant CRDs to be established..."
wait_for_crd "authpolicies.kuadrant.io" "120s" || \
    echo "   ⚠️  AuthPolicy CRD not yet available"
wait_for_crd "ratelimitpolicies.kuadrant.io" "120s" || \
    echo "   ⚠️  RateLimitPolicy CRD not yet available"

echo ""
echo "4️⃣ Deploying Gateway and networking infrastructure..."
CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
if [ -z "$CLUSTER_DOMAIN" ]; then
    echo "❌ Failed to retrieve cluster domain from OpenShift"
    exit 1
fi
export CLUSTER_DOMAIN
echo "   Cluster domain: $CLUSTER_DOMAIN"

echo "   Deploying Gateway API and Kuadrant configuration..."
cd "$PROJECT_ROOT"
kustomize build deployment/base/networking | envsubst | kubectl apply --server-side=true --force-conflicts -f -

# Wait for Gateway API CRDs if not already present
if ! kubectl get crd gateways.gateway.networking.k8s.io &>/dev/null 2>&1; then
    echo "   Waiting for Gateway API CRDs..."
    wait_for_crd "gateways.gateway.networking.k8s.io" "120s" || \
        echo "   ⚠️  Gateway API CRDs not yet available"
fi

echo ""
echo "5️⃣ Checking for OpenDataHub/RHOAI KServe..."
if kubectl get crd llminferenceservices.serving.kserve.io &>/dev/null 2>&1; then
    echo "   ✅ KServe CRDs already present (ODH/RHOAI detected)"
else
    echo "   ⚠️  KServe not detected. Deploying ODH KServe components..."
    echo "   Note: This may require multiple attempts as CRDs need to be established first."
    
    # First attempt
    echo "   Attempting ODH KServe deployment (attempt 1/2)..."
    if kustomize build "$PROJECT_ROOT/deployment/components/odh/kserve" | kubectl apply --server-side=true --force-conflicts -f - 2>/dev/null; then
        echo "   ✅ Initial deployment successful"
    else
        echo "   ⚠️  First attempt failed (expected if CRDs not yet ready)"
    fi
    
    # Wait for CRDs and operator pods, then retry
    echo "   Waiting for KServe CRDs to be established..."
    if wait_for_crd "llminferenceservices.serving.kserve.io" "120s"; then
        
        wait_for_pods "opendatahub" 120 || true
        wait_for_validating_webhooks opendatahub 90 || true
        
        echo "   Retrying deployment (attempt 2/2)..."
        kustomize build "$PROJECT_ROOT/deployment/components/odh/kserve" | kubectl apply --server-side=true --force-conflicts -f - && \
            echo "   ✅ ODH KServe components deployed successfully" || \
            echo "   ⚠️  ODH KServe deployment failed. This may be expected if ODH operator manages these resources."
    else
        echo "   ⚠️  CRDs did not become ready in time. Continuing anyway..."
        echo "   Run: kustomize build $PROJECT_ROOT/deployment/components/odh/kserve | kubectl apply --server-side=true --force-conflicts -f -"
    fi
fi

echo ""
echo "6️⃣ Deploying MaaS API..."
cd "$PROJECT_ROOT"
kustomize build deployment/base/maas-api | envsubst | kubectl apply -f -

echo ""
echo "7️⃣ Applying OpenShift-specific configurations..."

# Patch Kuadrant for OpenShift Gateway Controller
echo "   Patching Kuadrant operator..."
if ! kubectl -n kuadrant-system get deployment kuadrant-operator-controller-manager -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ISTIO_GATEWAY_CONTROLLER_NAMES")]}' | grep -q "ISTIO_GATEWAY_CONTROLLER_NAMES"; then
  kubectl -n kuadrant-system patch deployment kuadrant-operator-controller-manager \
    --type='json' \
    -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ISTIO_GATEWAY_CONTROLLER_NAMES","value":"openshift.io/gateway-controller/v1"}}]'
  echo "   ✅ Kuadrant operator patched"
else
  echo "   ✅ Kuadrant operator already configured"
fi

echo ""
echo "8️⃣ Waiting for Gateway to be ready..."
echo "   Note: This may take a few minutes if Service Mesh is being automatically installed..."

# Wait for Service Mesh CRDs to be established
if kubectl get crd istios.sailoperator.io &>/dev/null 2>&1; then
    echo "   ✅ Service Mesh operator already detected"
else
    echo "   Waiting for automatic Service Mesh installation..."
    if wait_for_crd "istios.sailoperator.io" "300s"; then
        echo "   ✅ Service Mesh operator installed"
    else
        echo "   ⚠️  Service Mesh CRD not detected within timeout"
        echo "      Gateway may take longer to become ready or require manual Service Mesh installation"
    fi
fi

echo "   Waiting for Gateway to become ready..."
kubectl wait --for=condition=Programmed gateway maas-default-gateway -n openshift-ingress --timeout=300s || \
  echo "   ⚠️  Gateway is taking longer than expected, continuing..."

echo ""
echo "9️⃣ Applying Gateway Policies..."
cd "$PROJECT_ROOT"
kustomize build deployment/base/policies | kubectl apply --server-side=true --force-conflicts -f -

echo ""
echo "🔟 Restarting Kuadrant operators for policy enforcement..."
kubectl rollout restart deployment/kuadrant-operator-controller-manager -n kuadrant-system
kubectl rollout restart deployment/authorino-operator -n kuadrant-system
kubectl rollout restart deployment/limitador-operator-controller-manager -n kuadrant-system

# Wait for rollouts to complete
echo "   Waiting for operators to restart..."
kubectl rollout status deployment/kuadrant-operator-controller-manager -n kuadrant-system --timeout=120s
kubectl rollout status deployment/authorino-operator -n kuadrant-system --timeout=120s
kubectl rollout status deployment/limitador-operator-controller-manager -n kuadrant-system --timeout=120s

echo ""
echo "1️⃣1️⃣ Patching AuthPolicy with correct audience..."
AUD="$(kubectl create token default --duration=10m 2>/dev/null | cut -d. -f2 | base64 -d 2>/dev/null | jq -r '.aud[0]' 2>/dev/null)"
if [ -n "$AUD" ] && [ "$AUD" != "null" ]; then
    echo "   Detected audience: $AUD"
    kubectl patch authpolicy maas-api-auth-policy -n maas-api \
      --type='json' \
      -p "$(jq -nc --arg aud "$AUD" '[{
        op:"replace",
        path:"/spec/rules/authentication/openshift-identities/kubernetesTokenReview/audiences/0",
        value:$aud
      }]')" 2>/dev/null && echo "   ✅ AuthPolicy patched" || echo "   ⚠️  Failed to patch AuthPolicy (may need manual configuration)"
else
    echo "   ⚠️  Could not detect audience, skipping AuthPolicy patch"
    echo "      You may need to manually configure the audience later"
fi

echo ""
echo "1️⃣2️⃣ Updating Limitador image for metrics exposure..."
kubectl -n kuadrant-system patch limitador limitador --type merge \
  -p '{"spec":{"image":"quay.io/kuadrant/limitador:1a28eac1b42c63658a291056a62b5d940596fd4c","version":""}}' 2>/dev/null && \
  echo "   ✅ Limitador image updated" || \
  echo "   ⚠️  Could not update Limitador image (may not be critical)"

# Verification
echo ""
echo "========================================="
echo "✅ Deployment Complete!"
echo "========================================="
echo ""
echo "📊 Status Check:"
echo ""

# Check component status
echo "Component Status:"
kubectl get pods -n maas-api --no-headers | grep Running | wc -l | xargs echo "  MaaS API pods running:"
kubectl get pods -n kuadrant-system --no-headers | grep Running | wc -l | xargs echo "  Kuadrant pods running:"
kubectl get pods -n opendatahub --no-headers | grep Running | wc -l | xargs echo "  KServe pods running:"

echo ""
echo "Gateway Status:"
kubectl get gateway -n openshift-ingress maas-default-gateway -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' | xargs echo "  Accepted:"
kubectl get gateway -n openshift-ingress maas-default-gateway -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' | xargs echo "  Programmed:"

echo ""
echo "Policy Status:"
kubectl get authpolicy -n openshift-ingress gateway-auth-policy -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' 2>/dev/null | xargs echo "  AuthPolicy:"
kubectl get tokenratelimitpolicy -n openshift-ingress gateway-token-rate-limits -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' 2>/dev/null | xargs echo "  TokenRateLimitPolicy:"

echo ""
echo "========================================="
echo "📝 Next Steps:"
echo "========================================="
echo ""
echo "1. Deploy a sample model:"
echo "   kustomize build docs/samples/models/simulator | kubectl apply -f -"
echo ""
echo "2. Test the API:"
echo "   Access the MaaS API at: https://maas-api.$CLUSTER_DOMAIN"
echo "   Access models through: https://gateway.$CLUSTER_DOMAIN"
echo ""
echo "3. Get a token:"
echo "   curl -sSk -H \"Authorization: Bearer \$(oc whoami -t)\" \\"
echo "     -H \"Content-Type: application/json\" -X POST \\"
echo "     -d '{\"expiration\": \"10m\"}' \\"
echo "     \"https://maas-api.$CLUSTER_DOMAIN/v1/tokens\""
echo ""
echo "For troubleshooting, check the deployment guide at deployment/README.md" 
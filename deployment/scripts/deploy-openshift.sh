#!/bin/bash

# OpenShift MaaS Platform Deployment Script
# This script automates the complete deployment of the MaaS platform on OpenShift

set -e

echo "========================================="
echo "🚀 MaaS Platform OpenShift Deployment"
echo "========================================="
echo ""

# Check if running on OpenShift
if ! kubectl api-resources | grep -q "route.openshift.io"; then
    echo "❌ This script is for OpenShift clusters only."
    echo "   Use 'deploy-kubernetes.sh' for standard Kubernetes clusters."
    exit 1
fi

# Note about Service Mesh
echo "📋 Checking prerequisites..."
echo "ℹ️  Note: OpenShift Service Mesh should be automatically installed when GatewayClass is created."
echo "   If the Gateway gets stuck in 'Waiting for controller', you may need to manually"
echo "   install the Red Hat OpenShift Service Mesh operator from OperatorHub."

# Step 1: Enable Gateway API features (if needed)
echo ""
echo "1️⃣ Checking OpenShift version and Gateway API requirements..."

# Get OpenShift version
OCP_VERSION=$(oc version -o json | jq -r '.openshiftVersion' 2>/dev/null || echo "unknown")
echo "   OpenShift version: $OCP_VERSION"

# Check if version is 4.19.9 or higher
if [[ "$OCP_VERSION" == "unknown" ]]; then
    echo "   ⚠️  Could not determine OpenShift version, applying feature gates to be safe"
    oc patch featuregate/cluster --type='merge' \
      -p '{"spec":{"featureSet":"CustomNoUpgrade","customNoUpgrade":{"enabled":["GatewayAPI","GatewayAPIController"]}}}' || true
    echo "   Waiting for feature gates to reconcile (30 seconds)..."
    sleep 30
else
    # Extract major.minor.patch version numbers
    VERSION_REGEX="^[v]?([0-9]+)\.([0-9]+)\.([0-9]+)"
    if [[ "$OCP_VERSION" =~ $VERSION_REGEX ]]; then
        MAJOR="${BASH_REMATCH[1]}"
        MINOR="${BASH_REMATCH[2]}"
        PATCH="${BASH_REMATCH[3]}"
        
        # Check if version is 4.19.9 or higher
        if [[ $MAJOR -gt 4 ]] || \
           [[ $MAJOR -eq 4 && $MINOR -gt 19 ]] || \
           [[ $MAJOR -eq 4 && $MINOR -eq 19 && $PATCH -ge 9 ]]; then
            echo "   ✅ OpenShift $OCP_VERSION supports Gateway API via GatewayClass (no feature gates needed)"
        else
            echo "   Applying Gateway API feature gates for OpenShift < 4.19.9"
            oc patch featuregate/cluster --type='merge' \
              -p '{"spec":{"featureSet":"CustomNoUpgrade","customNoUpgrade":{"enabled":["GatewayAPI","GatewayAPIController"]}}}' || true
            echo "   Waiting for feature gates to reconcile (30 seconds)..."
            sleep 30
        fi
    else
        echo "   ⚠️  Could not parse version, applying feature gates to be safe"
        oc patch featuregate/cluster --type='merge' \
          -p '{"spec":{"featureSet":"CustomNoUpgrade","customNoUpgrade":{"enabled":["GatewayAPI","GatewayAPIController"]}}}' || true
        echo "   Waiting for feature gates to reconcile (30 seconds)..."
        sleep 30
    fi
fi

# Step 2: Create namespaces
echo ""
echo "2️⃣ Creating namespaces..."
for ns in kserve kuadrant-system llm maas-api; do
    kubectl create namespace $ns 2>/dev/null || echo "   Namespace $ns already exists"
done

# Step 3: Install dependencies
echo ""
echo "3️⃣ Installing dependencies..."

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "   Installing cert-manager..."
"$SCRIPT_DIR/install-dependencies.sh" --cert-manager

# Note: KServe should be installed as part of ODH/RHOAI, not separately
# If ODH/RHOAI is not installed, uncomment the following line:
# "$SCRIPT_DIR/install-dependencies.sh" --kserve

# Clean up any leftover Kuadrant CRDs from previous installations
echo "   Checking for leftover Kuadrant CRDs..."
LEFTOVER_CRDS=$(kubectl get crd 2>/dev/null | grep -E "kuadrant|authorino|limitador" | awk '{print $1}')
if [ -n "$LEFTOVER_CRDS" ]; then
    echo "   Found leftover CRDs, cleaning up..."
    echo "$LEFTOVER_CRDS" | xargs -r kubectl delete crd --timeout=30s 2>/dev/null || true
fi

echo "   Installing Kuadrant..."
"$SCRIPT_DIR/install-dependencies.sh" --kuadrant

# Step 4: Deploy core infrastructure
echo ""
echo "4️⃣ Deploying core infrastructure..."
CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
if [ -z "$CLUSTER_DOMAIN" ]; then
    echo "❌ Failed to retrieve cluster domain from OpenShift"
    exit 1
fi
export CLUSTER_DOMAIN
echo "   Cluster domain: $CLUSTER_DOMAIN"

cd "$PROJECT_ROOT"
kustomize build deployment/overlays/openshift | envsubst | kubectl apply -f -

# Step 5: Apply OpenShift-specific patches
echo ""
echo "5️⃣ Applying OpenShift-specific configurations..."

# Patch Kuadrant for OpenShift Gateway Controller
echo "   Patching Kuadrant operator..."
kubectl -n kuadrant-system patch deployment kuadrant-operator-controller-manager \
  --type='json' \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/env/-","value":{"name":"ISTIO_GATEWAY_CONTROLLER_NAMES","value":"openshift.io/gateway-controller/v1"}}]' 2>/dev/null || \
  echo "   Kuadrant operator already patched"

# Update KServe Ingress Domain
echo "   Updating KServe configuration..."
kubectl -n kserve patch configmap inferenceservice-config \
  --type='json' \
  -p="[{\"op\": \"replace\", \"path\": \"/data/ingress\", \"value\": \"{\\\"enableGatewayApi\\\": true, \\\"kserveIngressGateway\\\": \\\"openshift-ingress/openshift-ai-inference\\\", \\\"ingressGateway\\\": \\\"istio-system/istio-ingressgateway\\\", \\\"ingressDomain\\\": \\\"$CLUSTER_DOMAIN\\\"}\" }]" 2>/dev/null || \
  echo "   KServe already configured"

# Step 6: Wait for Gateway to be ready
echo ""
echo "6️⃣ Waiting for Gateway to be ready..."
echo "   Note: This may take a few minutes if Service Mesh is being automatically installed..."

# Check if Service Mesh is being installed
if kubectl get crd istios.sailoperator.io &>/dev/null 2>&1; then
    echo "   Service Mesh operator detected"
else
    echo "   Waiting for automatic Service Mesh installation (up to 5 minutes)..."
    for i in {1..30}; do
        if kubectl get crd istios.sailoperator.io &>/dev/null 2>&1; then
            echo "   Service Mesh operator installed!"
            break
        fi
        sleep 10
    done
fi

kubectl wait --for=condition=Programmed gateway openshift-ai-inference -n openshift-ingress --timeout=300s || \
  echo "   Gateway is taking longer than expected, continuing..."

# Step 7: Restart Kuadrant operators for policy enforcement
echo ""
echo "7️⃣ Restarting Kuadrant operators for policy enforcement..."
kubectl rollout restart deployment/kuadrant-operator-controller-manager -n kuadrant-system
kubectl rollout restart deployment/authorino-operator -n kuadrant-system
kubectl rollout restart deployment/limitador-operator-controller-manager -n kuadrant-system

# Wait for rollouts to complete
echo "   Waiting for operators to restart..."
kubectl rollout status deployment/kuadrant-operator-controller-manager -n kuadrant-system --timeout=120s
kubectl rollout status deployment/authorino-operator -n kuadrant-system --timeout=120s
kubectl rollout status deployment/limitador-operator-controller-manager -n kuadrant-system --timeout=120s

# Step 8: Restart KServe controller
# echo ""
# echo "8️⃣ Restarting KServe controller..."
# kubectl rollout restart deployment kserve-controller-manager -n kserve
# kubectl rollout status deployment/kserve-controller-manager -n kserve --timeout=120s

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
kubectl get pods -n kserve --no-headers | grep Running | wc -l | xargs echo "  KServe pods running:"

echo ""
echo "Gateway Status:"
kubectl get gateway -n openshift-ingress openshift-ai-inference -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' | xargs echo "  Accepted:"
kubectl get gateway -n openshift-ingress openshift-ai-inference -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' | xargs echo "  Programmed:"

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
echo "   kustomize build deployment/samples/models/simulator | kubectl apply -f -"
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
#!/bin/bash

# OpenShift MaaS Platform Deployment Script
# This script automates the complete deployment of the MaaS platform on OpenShift

set -e

# Helper function to wait for CRD to be established
wait_for_crd() {
  local crd="$1"
  local timeout="${2:-60}"  # timeout in seconds
  local interval=2
  local elapsed=0

  echo "⏳ Waiting for CRD ${crd} to appear (timeout: ${timeout}s)…"
  while [ $elapsed -lt $timeout ]; do
    if kubectl get crd "$crd" &>/dev/null; then
      echo "✅ CRD ${crd} detected, waiting for it to become Established..."
      kubectl wait --for=condition=Established --timeout="${timeout}s" "crd/$crd" 2>/dev/null
      return 0
    fi
    sleep $interval
    elapsed=$((elapsed + interval))
  done

  echo "❌ Timed out after ${timeout}s waiting for CRD $crd to appear." >&2
  return 1
}

# Helper function to extract version from CSV name (e.g., "operator.v1.2.3" -> "1.2.3")
extract_version_from_csv() {
  local csv_name="$1"
  echo "$csv_name" | sed -n 's/.*\.v\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\).*/\1/p'
}

# Helper function to compare semantic versions (returns 0 if version1 >= version2)
version_compare() {
  local version1="$1"
  local version2="$2"
  
  # Convert versions to comparable numbers (e.g., "1.2.3" -> "001002003")
  local v1=$(echo "$version1" | awk -F. '{printf "%03d%03d%03d", $1, $2, $3}')
  local v2=$(echo "$version2" | awk -F. '{printf "%03d%03d%03d", $1, $2, $3}')
  
  [ "$v1" -ge "$v2" ]
}

# Helper function to find CSV by operator name and check minimum version
find_csv_with_min_version() {
  local operator_prefix="$1"
  local min_version="$2"
  local namespace="${3:-kuadrant-system}"
  
  local csv_name=$(kubectl get csv -n "$namespace" --no-headers 2>/dev/null | grep "^${operator_prefix}" | head -n1 | awk '{print $1}' || echo "")
  
  if [ -z "$csv_name" ]; then
    echo ""
    return 1
  fi
  
  local installed_version=$(extract_version_from_csv "$csv_name")
  if version_compare "$installed_version" "$min_version"; then
    echo "$csv_name"
    return 0
  else
    echo ""
    return 1
  fi
}

# Helper function to wait for CSV with minimum version requirement
wait_for_csv_with_min_version() {
  local operator_prefix="$1"
  local min_version="$2"
  local namespace="${3:-kuadrant-system}"
  local timeout="${4:-180}"
  
  echo "⏳ Looking for ${operator_prefix} (minimum version: ${min_version})..."
  
  local csv_name=$(find_csv_with_min_version "$operator_prefix" "$min_version" "$namespace")
  if [ -z "$csv_name" ]; then
    # Check if any version exists (for better error message)
    local any_csv=$(kubectl get csv -n "$namespace" --no-headers 2>/dev/null | grep "^${operator_prefix}" | head -n1 | awk '{print $1}' || echo "")
    if [ -n "$any_csv" ]; then
      local installed_version=$(extract_version_from_csv "$any_csv")
      echo "❌ Found ${any_csv} with version ${installed_version}, but minimum required is ${min_version}"
      return 1
    else
      echo "❌ No CSV found for operator ${operator_prefix} in namespace ${namespace}"
      return 1
    fi
  fi
  
  local installed_version=$(extract_version_from_csv "$csv_name")
  echo "✅ Found CSV: ${csv_name} (version: ${installed_version} >= ${min_version})"
  wait_for_csv "$csv_name" "$namespace" "$timeout"
}

# Helper function to wait for CSV to reach Succeeded state
wait_for_csv() {
  local csv_name="$1"
  local namespace="${2:-kuadrant-system}"
  local timeout="${3:-180}"  # timeout in seconds
  local interval=5
  local elapsed=0
  local last_status_print=0

  echo "⏳ Waiting for CSV ${csv_name} to succeed (timeout: ${timeout}s)..."
  while [ $elapsed -lt $timeout ]; do
    local phase=$(kubectl get csv -n "$namespace" "$csv_name" -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")

    case "$phase" in
      "Succeeded")
        echo "✅ CSV ${csv_name} succeeded"
        return 0
        ;;
      "Failed")
        echo "❌ CSV ${csv_name} failed" >&2
        kubectl get csv -n "$namespace" "$csv_name" -o jsonpath='{.status.message}' 2>/dev/null
        return 1
        ;;
      *)
        if [ $((elapsed - last_status_print)) -ge 30 ]; then
          echo "   CSV ${csv_name} status: ${phase} (${elapsed}s elapsed)"
          last_status_print=$elapsed
        fi
        ;;
    esac

    sleep $interval
    elapsed=$((elapsed + interval))
  done

  echo "❌ Timed out after ${timeout}s waiting for CSV ${csv_name}" >&2
  return 1
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
echo "  - git: $(git --version 2>/dev/null || echo 'not found')"
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

# Only clean up leftover CRDs if Kuadrant operators are NOT already installed
echo "   Checking for existing Kuadrant installation..."
EXISTING_KUADRANT_CSV=$(find_csv_with_min_version "kuadrant-operator" "1.3.0" "kuadrant-system")
if [ -z "$EXISTING_KUADRANT_CSV" ]; then
    echo "   No existing installation found, checking for leftover CRDs..."
    LEFTOVER_CRDS=$(kubectl get crd 2>/dev/null | grep -E "kuadrant|authorino|limitador" | awk '{print $1}')
    if [ -n "$LEFTOVER_CRDS" ]; then
        echo "   Found leftover CRDs, cleaning up before installation..."
        echo "$LEFTOVER_CRDS" | xargs -r kubectl delete crd --timeout=30s 2>/dev/null || true
        sleep 5  # Brief wait for cleanup to complete
    fi
else
    echo "   ✅ Kuadrant operator already installed ($EXISTING_KUADRANT_CSV), skipping CRD cleanup"
fi

echo "   Installing Kuadrant..."
"$SCRIPT_DIR/install-dependencies.sh" --kuadrant

echo ""
echo "4️⃣ Deploying Gateway infrastructure..."
CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
if [ -z "$CLUSTER_DOMAIN" ]; then
    echo "❌ Failed to retrieve cluster domain from OpenShift"
    exit 1
fi
export CLUSTER_DOMAIN
echo "   Cluster domain: $CLUSTER_DOMAIN"

echo "   Deploying Gateway and GatewayClass..."
cd "$PROJECT_ROOT"
kubectl apply --server-side=true --force-conflicts -f deployment/base/networking/odh/odh-gateway-api.yaml
kubectl apply --server-side=true --force-conflicts -f <(envsubst '$CLUSTER_DOMAIN' < deployment/base/networking/maas/maas-gateway-api.yaml)

echo ""
echo "5️⃣ Checking for OpenDataHub/RHOAI KServe..."
if kubectl get crd llminferenceservices.serving.kserve.io &>/dev/null 2>&1; then
    echo "   ✅ KServe CRDs already present (ODH/RHOAI detected)"
else
    echo "   ⚠️  KServe not detected. Deploying ODH KServe components..."
    "$SCRIPT_DIR/install-dependencies.sh" --ocp --odh
fi

echo ""
echo "6️⃣ Waiting for Kuadrant operators to be installed by OLM..."
# Wait for CSVs to reach Succeeded state with minimum version requirements
wait_for_csv_with_min_version "kuadrant-operator" "1.3.0" "kuadrant-system" 300 || \
    echo "   ⚠️  Kuadrant operator CSV did not succeed, continuing anyway..."

wait_for_csv_with_min_version "authorino-operator" "0.22.0" "kuadrant-system" 60 || \
    echo "   ⚠️  Authorino operator CSV did not succeed"

wait_for_csv_with_min_version "limitador-operator" "0.16.0" "kuadrant-system" 60 || \
    echo "   ⚠️  Limitador operator CSV did not succeed"

wait_for_csv_with_min_version "dns-operator" "0.15.0" "kuadrant-system" 60 || \
    echo "   ⚠️  DNS operator CSV did not succeed"

# Verify CRDs are present
echo "   Verifying Kuadrant CRDs are available..."
wait_for_crd "kuadrants.kuadrant.io" 30 || echo "   ⚠️  kuadrants.kuadrant.io CRD not found"
wait_for_crd "authpolicies.kuadrant.io" 10 || echo "   ⚠️  authpolicies.kuadrant.io CRD not found"
wait_for_crd "ratelimitpolicies.kuadrant.io" 10 || echo "   ⚠️  ratelimitpolicies.kuadrant.io CRD not found"
wait_for_crd "tokenratelimitpolicies.kuadrant.io" 10 || echo "   ⚠️  tokenratelimitpolicies.kuadrant.io CRD not found"

echo ""
echo "7️⃣ Deploying Kuadrant configuration (now that CRDs exist)..."
cd "$PROJECT_ROOT"
kubectl apply -f deployment/base/networking/odh/kuadrant.yaml

echo ""
echo "8️⃣ Deploying MaaS API..."
cd "$PROJECT_ROOT"
kustomize build deployment/base/maas-api | envsubst | kubectl apply -f -

# Restart Kuadrant operator to pick up the new configuration
echo "   Restarting Kuadrant operator to apply Gateway API provider recognition..."
kubectl rollout restart deployment/kuadrant-operator-controller-manager -n kuadrant-system
echo "   Waiting for Kuadrant operator to be ready..."
kubectl rollout status deployment/kuadrant-operator-controller-manager -n kuadrant-system --timeout=60s || \
  echo "   ⚠️  Kuadrant operator taking longer than expected, continuing..."

echo ""
echo "🔟 Waiting for Gateway to be ready..."
echo "   Note: This may take a few minutes if Service Mesh is being automatically installed..."

# Wait for Service Mesh CRDs to be established
if kubectl get crd istios.sailoperator.io &>/dev/null 2>&1; then
    echo "   ✅ Service Mesh operator already detected"
else
    echo "   Waiting for automatic Service Mesh installation..."
    if wait_for_crd "istios.sailoperator.io" 300; then
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
echo "1️⃣1️⃣ Applying Gateway Policies..."
cd "$PROJECT_ROOT"
kustomize build deployment/base/policies | kubectl apply --server-side=true --force-conflicts -f -

echo ""
echo "1️⃣3️⃣ Patching AuthPolicy with correct audience..."
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
echo "1️⃣4️⃣ Updating Limitador image for metrics exposure..."
kubectl -n kuadrant-system patch limitador limitador --type merge \
  -p '{"spec":{"image":"quay.io/kuadrant/limitador:1a28eac1b42c63658a291056a62b5d940596fd4c","version":""}}' 2>/dev/null && \
  echo "   ✅ Limitador image updated" || \
  echo "   ⚠️  Could not update Limitador image (may not be critical)"

echo ""
echo "========================================="
echo "⚠️  TEMPORARY WORKAROUNDS (TO BE REMOVED)"
echo "========================================="
echo ""
echo "Applying temporary workarounds for known issues..."

echo "   🔧 Restarting Kuadrant, Authorino, and Limitador operators to refresh webhook configurations..."
kubectl delete pod -n kuadrant-system -l control-plane=controller-manager 2>/dev/null && \
  echo "   ✅ Kuadrant operator restarted" || \
  echo "   ⚠️  Could not restart Kuadrant operator"

# Find and restart Authorino deployment dynamically
AUTHORINO_DEPLOYMENT=$(kubectl get deployments -n kuadrant-system --no-headers 2>/dev/null | grep authorino | head -n1 | awk '{print $1}' || echo "")
if [ -n "$AUTHORINO_DEPLOYMENT" ]; then
  kubectl rollout restart deployment "$AUTHORINO_DEPLOYMENT" -n kuadrant-system 2>/dev/null && \
    echo "   ✅ Authorino operator ($AUTHORINO_DEPLOYMENT) restarted" || \
    echo "   ⚠️  Could not restart Authorino operator ($AUTHORINO_DEPLOYMENT)"
else
  echo "   ⚠️  No Authorino deployment found, skipping restart"
fi

kubectl rollout restart deployment limitador-operator-controller-manager -n kuadrant-system 2>/dev/null && \
  echo "   ✅ Limitador operator restarted" || \
  echo "   ⚠️  Could not restart Limitador operator"

echo "   Waiting for operators to be ready..."
kubectl rollout status deployment kuadrant-operator-controller-manager -n kuadrant-system --timeout=60s 2>/dev/null || \
  echo "   ⚠️  Kuadrant operator taking longer than expected"
# Check status of Authorino deployment dynamically
if [ -n "$AUTHORINO_DEPLOYMENT" ]; then
  kubectl rollout status deployment "$AUTHORINO_DEPLOYMENT" -n kuadrant-system --timeout=60s 2>/dev/null || \
    echo "   ⚠️  Authorino operator taking longer than expected"
fi
kubectl rollout status deployment limitador-operator-controller-manager -n kuadrant-system --timeout=60s 2>/dev/null || \
  echo "   ⚠️  Limitador operator taking longer than expected"

echo ""
echo "========================================="
# Deploy observability components (ServiceMonitor and TelemetryPolicy)
echo "   Deploying observability components..."
kustomize build deployment/base/observability | kubectl apply -f -
echo "   ✅ Observability components deployed"

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
echo "Policy Enforcement Status:"
kubectl get authpolicy -n openshift-ingress gateway-auth-policy -o jsonpath='{.status.conditions[?(@.type=="Enforced")].status}' 2>/dev/null | xargs echo "  AuthPolicy Enforced:"
kubectl get ratelimitpolicy -n openshift-ingress gateway-rate-limits -o jsonpath='{.status.conditions[?(@.type=="Enforced")].status}' 2>/dev/null | xargs echo "  RateLimitPolicy Enforced:"
kubectl get tokenratelimitpolicy -n openshift-ingress gateway-token-rate-limits -o jsonpath='{.status.conditions[?(@.type=="Enforced")].status}' 2>/dev/null | xargs echo "  TokenRateLimitPolicy Enforced:"
kubectl get telemetrypolicy -n openshift-ingress user-group -o jsonpath='{.status.conditions[?(@.type=="Enforced")].status}' 2>/dev/null | xargs echo "  TelemetryPolicy Enforced:"

echo ""
echo "========================================="
echo "🔧 Troubleshooting:"
echo "========================================="
echo ""
echo "If policies show 'Not enforced' status:"
echo "1. Check if Gateway API provider is recognized:"
echo "   kubectl describe authpolicy gateway-auth-policy -n openshift-ingress | grep -A 5 'Status:'"
echo ""
echo "2. If Gateway API provider is not installed, restart all Kuadrant operators:"
echo "   kubectl rollout restart deployment/kuadrant-operator-controller-manager -n kuadrant-system"
echo "   # Find and restart Authorino deployment:"
echo "   AUTHORINO_DEPLOYMENT=\$(kubectl get deployments -n kuadrant-system --no-headers | grep authorino | head -n1 | awk '{print \$1}')"
echo "   kubectl rollout restart deployment/\$AUTHORINO_DEPLOYMENT -n kuadrant-system"
echo "   kubectl rollout restart deployment/limitador-operator-controller-manager -n kuadrant-system"
echo ""
echo "3. Check if OpenShift Gateway Controller is available:"
echo "   kubectl get gatewayclass"
echo ""
echo "4. If policies still show 'MissingDependency', ensure environment variable is set:"
echo "   kubectl get deployment kuadrant-operator-controller-manager -n kuadrant-system -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name==\"ISTIO_GATEWAY_CONTROLLER_NAMES\")]}'"
echo ""
echo "5. If environment variable is missing, patch the deployment:"
echo "   kubectl -n kuadrant-system patch deployment kuadrant-operator-controller-manager --type='json' \\"
echo "     -p='[{\"op\": \"add\", \"path\": \"/spec/template/spec/containers/0/env/-\", \"value\": {\"name\": \"ISTIO_GATEWAY_CONTROLLER_NAMES\", \"value\": \"openshift.io/gateway-controller/v1\"}}]'"
echo ""
echo "6. Restart Kuadrant operator after patching:"
echo "   kubectl rollout restart deployment/kuadrant-operator-controller-manager -n kuadrant-system"
echo "   kubectl rollout status deployment/kuadrant-operator-controller-manager -n kuadrant-system --timeout=60s"
echo ""
echo "7. Wait for policies to be enforced (may take 1-2 minutes):"
echo "   kubectl describe authpolicy gateway-auth-policy -n openshift-ingress | grep -A 10 'Status:'"
echo ""
echo "If metrics are not visible in Prometheus:"
echo "1. Check ServiceMonitor:"
echo "   kubectl get servicemonitor limitador-metrics -n kuadrant-system"
echo ""
echo "2. Check Prometheus targets:"
echo "   kubectl port-forward -n openshift-monitoring svc/prometheus-k8s 9090:9091 &"
echo "   # Visit http://localhost:9090/targets and look for limitador targets"
echo ""
echo "If webhook timeout errors occur during model deployment:"
echo "1. Restart ODH model controller:"
echo "   kubectl rollout restart deployment/odh-model-controller -n opendatahub"
echo ""
echo "2. Temporarily bypass webhook:"
echo "   kubectl patch validatingwebhookconfigurations validating.odh-model-controller.opendatahub.io --type='json' -p='[{\"op\": \"replace\", \"path\": \"/webhooks/1/failurePolicy\", \"value\": \"Ignore\"}]'"
echo "   # Deploy your model, then restore:"
echo "   kubectl patch validatingwebhookconfigurations validating.odh-model-controller.opendatahub.io --type='json' -p='[{\"op\": \"replace\", \"path\": \"/webhooks/1/failurePolicy\", \"value\": \"Fail\"}]'"
echo ""
echo "If API calls return 404 errors (Gateway routing issues):"
echo "1. Check HTTPRoute status:"
echo "   kubectl get httproute -A"
echo "   kubectl describe httproute facebook-opt-125m-simulated-kserve-route -n llm"
echo ""
echo "2. Check if model is accessible directly:"
echo "   kubectl get pods -n llm"
echo "   kubectl port-forward -n llm svc/facebook-opt-125m-simulated-kserve-workload-svc 8080:8000 &"
echo "   curl -k https://localhost:8080/health"
echo ""
echo "3. Test model with correct name and HTTPS:"
echo "   curl -k -H \"Content-Type: application/json\" -d '{\"model\": \"facebook/opt-125m\", \"prompt\": \"Hello\", \"max_tokens\": 50}' https://localhost:8080/v1/chat/completions"
echo ""
echo "4. Check Gateway status:"
echo "   kubectl get gateway -A"
echo "   kubectl describe gateway maas-default-gateway -n openshift-ingress"
echo ""
echo "If metrics are not generated despite successful API calls:"
echo "1. Verify policies are enforced:"
echo "   kubectl describe authpolicy gateway-auth-policy -n openshift-ingress | grep -A 5 'Enforced'"
echo "   kubectl describe ratelimitpolicy gateway-rate-limits -n openshift-ingress | grep -A 5 'Enforced'"
echo ""
echo "2. Check Limitador metrics directly:"
echo "   kubectl port-forward -n kuadrant-system svc/limitador-limitador 8080:8080 &"
echo "   curl http://localhost:8080/metrics | grep -E '(authorized_hits|authorized_calls|limited_calls)'"
echo ""
echo "3. Make test API calls to trigger metrics:"
echo "   # Use HTTPS and correct model name: facebook/opt-125m"
echo "   for i in {1..5}; do curl -k -H \"Authorization: Bearer \$TOKEN\" -H \"Content-Type: application/json\" -d '{\"model\": \"facebook/opt-125m\", \"prompt\": \"Hello \$i\", \"max_tokens\": 50}' \"https://\${HOST}/llm/facebook-opt-125m-simulated/v1/chat/completions\"; done"

echo ""
echo "========================================="
echo "📝 Next Steps:"
echo "========================================="
echo ""
echo "1. Deploy a sample model:"
echo "   kustomize build docs/samples/models/simulator | kubectl apply -f -"
echo ""
echo "2. Get Gateway endpoint:"
echo "   CLUSTER_DOMAIN=\$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')"
echo "   HOST=\"maas.\${CLUSTER_DOMAIN}\""
echo ""
echo "3. Get authentication token:"
echo "   TOKEN_RESPONSE=\$(curl -sSk -H \"Authorization: Bearer \$(oc whoami -t)\" -H \"Content-Type: application/json\" -X POST -d '{\"expiration\": \"10m\"}' \"\${HOST}/maas-api/v1/tokens\")"
echo "   TOKEN=\$(echo \$TOKEN_RESPONSE | jq -r .token)"
echo ""
echo "4. Test model endpoint:"
echo "   MODELS=\$(curl -sSk \${HOST}/maas-api/v1/models -H \"Content-Type: application/json\" -H \"Authorization: Bearer \$TOKEN\" | jq -r .)"
echo "   MODEL_NAME=\$(echo \$MODELS | jq -r '.data[0].id')"
echo "   MODEL_URL=\"\${HOST}/llm/facebook-opt-125m-simulated/v1/chat/completions\" # Note: This may be different for your model"
echo "   curl -sSk -H \"Authorization: Bearer \$TOKEN\" -H \"Content-Type: application/json\" -d \"{\\\"model\\\": \\\"\${MODEL_NAME}\\\", \\\"prompt\\\": \\\"Hello\\\", \\\"max_tokens\\\": 50}\" \"\${MODEL_URL}\""
echo ""
echo "5. Test authorization limiting (no token 401 error):"
echo "   curl -sSk -H \"Content-Type: application/json\" -d \"{\\\"model\\\": \\\"\${MODEL_NAME}\\\", \\\"prompt\\\": \\\"Hello\\\", \\\"max_tokens\\\": 50}\" \"\${MODEL_URL}\" -v"
echo ""
echo "6. Test rate limiting (200 OK followed by 429 Rate Limit Exceeded after about 4 requests):"
echo "   for i in {1..16}; do curl -sSk -o /dev/null -w \"%{http_code}\\n\" -H \"Authorization: Bearer \$TOKEN\" -H \"Content-Type: application/json\" -d \"{\\\"model\\\": \\\"\${MODEL_NAME}\\\", \\\"prompt\\\": \\\"Hello\\\", \\\"max_tokens\\\": 50}\" \"\${MODEL_URL}\"; done"
echo ""
echo "7. Run validation script (Runs all the checks again):"
echo "   ./deployment/scripts/validate-deployment.sh"
echo ""
echo "8. Check metrics generation:"
echo "   kubectl port-forward -n kuadrant-system svc/limitador-limitador 8080:8080 &"
echo "   curl http://localhost:8080/metrics | grep -E '(authorized_hits|authorized_calls|limited_calls)'"
echo ""
echo "9. Access Prometheus to view metrics:"
echo "   kubectl port-forward -n openshift-monitoring svc/prometheus-k8s 9090:9091 &"
echo "   # Open http://localhost:9090 in browser and search for: authorized_hits, authorized_calls, limited_calls"
echo ""

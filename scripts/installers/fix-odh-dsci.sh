#!/bin/bash

# ODH DSCInitialization Fix Script
# This script diagnoses and fixes DSCInitialization issues

set -e

echo "========================================="
echo "🔧 ODH DSCInitialization Troubleshooter"
echo "========================================="
echo ""

# Step 1: Check current state
echo "📋 Checking current ODH installation state..."
echo ""

# Check if ODH operator is installed
echo "1. ODH Operator Status:"
if kubectl get csv -n openshift-operators 2>/dev/null | grep -q opendatahub-operator; then
    echo "   ✅ ODH operator is installed"
    ODH_VERSION=$(kubectl get csv -n openshift-operators -o json | jq -r '.items[] | select(.metadata.name | contains("opendatahub")) | .spec.version' 2>/dev/null || echo "unknown")
    echo "   Version: $ODH_VERSION"
else
    echo "   ❌ ODH operator NOT installed"
    echo "   Please install it from OperatorHub or run: ./install-odh.sh"
    exit 1
fi

# Check if CRDs exist
echo ""
echo "2. CRD Status:"
if kubectl get crd dscinitializations.dscinitialization.opendatahub.io &>/dev/null 2>&1; then
    echo "   ✅ DSCInitialization CRD exists"
else
    echo "   ❌ DSCInitialization CRD missing"
    echo "   Waiting for CRD to be created by operator..."
    for i in {1..30}; do
        if kubectl get crd dscinitializations.dscinitialization.opendatahub.io &>/dev/null 2>&1; then
            echo "   ✅ DSCInitialization CRD now available"
            break
        fi
        echo "   Waiting... ($i/30)"
        sleep 10
    done
fi

if kubectl get crd datascienceclusters.datasciencecluster.opendatahub.io &>/dev/null 2>&1; then
    echo "   ✅ DataScienceCluster CRD exists"
else
    echo "   ⚠️ DataScienceCluster CRD missing"
fi

# Check namespace
echo ""
echo "3. Namespace Check:"
if kubectl get namespace opendatahub &>/dev/null 2>&1; then
    echo "   ✅ opendatahub namespace exists"
else
    echo "   Creating opendatahub namespace..."
    kubectl create namespace opendatahub
fi

# Check for existing DSCInitialization
echo ""
echo "4. DSCInitialization Resources:"
DSCI_LIST=$(kubectl get dscinitializations -A 2>/dev/null || echo "none")
if [[ "$DSCI_LIST" == "none" ]] || [[ -z "$DSCI_LIST" ]]; then
    echo "   ❌ No DSCInitialization found"
    NEEDS_DSCI=true
else
    echo "   Found DSCInitialization resources:"
    kubectl get dscinitializations -A
    NEEDS_DSCI=false
fi

# Check for existing DataScienceCluster
echo ""
echo "5. DataScienceCluster Resources:"
DSC_LIST=$(kubectl get datasciencecluster -A 2>/dev/null || echo "none")
if [[ "$DSC_LIST" == "none" ]] || [[ -z "$DSC_LIST" ]]; then
    echo "   No DataScienceCluster found"
    NEEDS_DSC=true
else
    echo "   Found DataScienceCluster resources:"
    kubectl get datasciencecluster -A
    
    # Check if DSC is failing due to missing DSCI
    DSC_ERROR=$(kubectl get datasciencecluster -n opendatahub -o json 2>/dev/null | jq -r '.items[0].status.conditions[] | select(.type=="ReconcileComplete") | .message' 2>/dev/null || echo "")
    if [[ "$DSC_ERROR" == *"dscinitializations"* ]]; then
        echo "   ⚠️ DataScienceCluster is failing due to missing DSCInitialization"
        NEEDS_DSCI=true
    else
        NEEDS_DSC=false
    fi
fi

# Step 2: Fix issues
echo ""
echo "========================================="
echo "🛠️ Applying Fixes"
echo "========================================="
echo ""

if [[ "$NEEDS_DSCI" == "true" ]]; then
    echo "Creating DSCInitialization..."
    cat <<EOF | kubectl apply -f -
apiVersion: dscinitialization.opendatahub.io/v1
kind: DSCInitialization
metadata:
  name: default-dsci
  namespace: opendatahub
spec:
  applicationsNamespace: opendatahub
  monitoring:
    managementState: Managed
    namespace: opendatahub
  serviceMesh:
    managementState: Managed
    auth:
      audiences:
        - "https://kubernetes.default.svc"
    controlPlane:
      name: data-science-smcp
      namespace: istio-system
  trustedCABundle:
    managementState: Managed
EOF
    
    echo "Waiting for DSCInitialization to be ready..."
    for i in {1..30}; do
        STATUS=$(kubectl get dscinitializations -n opendatahub default-dsci -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
        if [[ "$STATUS" == "Ready" ]]; then
            echo "✅ DSCInitialization is ready!"
            break
        fi
        echo "Status: $STATUS ($i/30)"
        sleep 10
    done
else
    echo "DSCInitialization already exists, checking status..."
    kubectl get dscinitializations -n opendatahub -o wide
fi

# If DataScienceCluster exists but was failing, restart it
if [[ "$NEEDS_DSC" == "false" ]] && [[ "$NEEDS_DSCI" == "true" ]]; then
    echo ""
    echo "Restarting DataScienceCluster reconciliation..."
    # Touch the DSC to trigger reconciliation
    kubectl annotate datasciencecluster -n opendatahub --all reconcile-trigger="$(date +%s)" --overwrite
fi

# Step 3: Final verification
echo ""
echo "========================================="
echo "📊 Final Verification"
echo "========================================="
echo ""

echo "DSCInitialization:"
kubectl get dscinitializations -n opendatahub -o wide

echo ""
echo "DataScienceCluster:"
kubectl get datasciencecluster -n opendatahub -o wide 2>/dev/null || echo "No DataScienceCluster found (you may need to create one)"

echo ""
echo "========================================="
echo "📝 Next Steps"
echo "========================================="
echo ""

if [[ "$NEEDS_DSC" == "true" ]]; then
    echo "Now you can create a DataScienceCluster. Example:"
    echo ""
    echo "cat <<EOF | kubectl apply -f -"
    echo "apiVersion: datasciencecluster.opendatahub.io/v1"
    echo "kind: DataScienceCluster"
    echo "metadata:"
    echo "  name: default-dsc"
    echo "  namespace: opendatahub"
    echo "spec:"
    echo "  components:"
    echo "    dashboard:"
    echo "      managementState: Managed"
    echo "    workbenches:"
    echo "      managementState: Managed"
    echo "    kserve:"
    echo "      managementState: Managed"
    echo "      defaultDeploymentMode: RawDeployment"
    echo "      nim:"
    echo "        managementState: Managed"
    echo "      rawDeploymentServiceConfig: Headless"
    echo "      serving:"
    echo "        ingressGateway:"
    echo "          certificate:"
    echo "            type: OpenshiftDefaultIngress"
    echo "        managementState: Removed"
    echo "        name: knative-serving"
    echo "    modelmeshserving:"
    echo "      managementState: Removed"
    echo "EOF"
else
    echo "Your ODH installation should now be working!"
    echo ""
    echo "Check the status with:"
    echo "- kubectl get dscinitializations -n opendatahub"
    echo "- kubectl get datasciencecluster -n opendatahub"
    echo "- kubectl get pods -n opendatahub"
fi 
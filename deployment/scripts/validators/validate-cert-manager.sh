#!/bin/bash

# cert-manager Validation Script
# This script validates that cert-manager is installed and running properly

set -e

echo "========================================="
echo "🔍 Validating cert-manager Installation"
echo "========================================="
echo ""

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl not found. Please install kubectl first."
    exit 1
fi

# Check if we can connect to the cluster
if ! kubectl cluster-info &> /dev/null; then
    echo "❌ Cannot connect to cluster. Please check your kubeconfig."
    exit 1
fi

echo "1️⃣ Checking for cert-manager namespace..."

# Check if cert-manager namespace exists
if ! kubectl get namespace cert-manager &> /dev/null; then
    echo "❌ cert-manager namespace not found"
    echo "   cert-manager may not be installed"
    exit 1
fi

echo "   ✅ cert-manager namespace exists"

echo ""
echo "2️⃣ Checking for cert-manager deployments..."

# Check for cert-manager deployments
DEPLOYMENTS=("cert-manager" "cert-manager-cainjector" "cert-manager-webhook")
ALL_DEPLOYMENTS_READY=true

for deployment in "${DEPLOYMENTS[@]}"; do
    if kubectl get deployment "$deployment" -n cert-manager &> /dev/null; then
        READY=$(kubectl get deployment "$deployment" -n cert-manager -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        DESIRED=$(kubectl get deployment "$deployment" -n cert-manager -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")
        
        if [[ "$READY" == "$DESIRED" ]] && [[ "$READY" != "0" ]]; then
            echo "   ✅ $deployment: $READY/$DESIRED replicas ready"
        else
            echo "   ❌ $deployment: $READY/$DESIRED replicas ready (expected $DESIRED)"
            ALL_DEPLOYMENTS_READY=false
        fi
    else
        echo "   ❌ $deployment: deployment not found"
        ALL_DEPLOYMENTS_READY=false
    fi
done

if [[ "$ALL_DEPLOYMENTS_READY" != true ]]; then
    echo ""
    echo "❌ Not all cert-manager deployments are ready"
    exit 1
fi

echo ""
echo "3️⃣ Checking for cert-manager pods..."

# Check pod status
PODS_READY=$(kubectl get pods -n cert-manager -l app.kubernetes.io/name=cert-manager --no-headers | grep -c "Running" || echo "0")
PODS_TOTAL=$(kubectl get pods -n cert-manager -l app.kubernetes.io/name=cert-manager --no-headers | wc -l)

if [[ "$PODS_READY" -gt 0 ]] && [[ "$PODS_READY" == "$PODS_TOTAL" ]]; then
    echo "   ✅ All cert-manager pods are running ($PODS_READY/$PODS_TOTAL)"
else
    echo "   ❌ cert-manager pods not all running ($PODS_READY/$PODS_TOTAL)"
    echo ""
    echo "   Pod status:"
    kubectl get pods -n cert-manager -l app.kubernetes.io/name=cert-manager
    exit 1
fi

echo ""
echo "4️⃣ Checking for cert-manager CRDs..."

# Check for key cert-manager CRDs
CRDS=("certificates.cert-manager.io" "issuers.cert-manager.io" "clusterissuers.cert-manager.io")
ALL_CRDS_EXIST=true

for crd in "${CRDS[@]}"; do
    if kubectl get crd "$crd" &> /dev/null; then
        echo "   ✅ $crd CRD exists"
    else
        echo "   ❌ $crd CRD not found"
        ALL_CRDS_EXIST=false
    fi
done

if [[ "$ALL_CRDS_EXIST" != true ]]; then
    echo ""
    echo "❌ Not all required cert-manager CRDs are present"
    exit 1
fi

echo ""
echo "========================================="
echo "✅ cert-manager Validation Complete!"
echo "========================================="
echo ""
echo "cert-manager is properly installed and running."
echo "Namespace: cert-manager"
echo "Deployments: ${#DEPLOYMENTS[@]} ready"
echo "Pods: $PODS_READY/$PODS_TOTAL running"
echo "CRDs: ${#CRDS[@]} present"
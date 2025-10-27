#!/bin/bash

# RHOAI Validation Script
# This script validates that RHOAI (Red Hat OpenShift AI) is installed by checking
# for the ClusterServiceVersion with name starting with "rhods-operator" and
# validating that the status.phase is Succeeded

set -e

echo "========================================="
echo "🔍 Validating RHOAI Installation"
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

echo "1️⃣ Checking for RHOAI operator ClusterServiceVersion..."

# Look for ClusterServiceVersion with name starting with "rhods-operator"
# Try using jq first, fallback to grep if jq is not available
if command -v jq &> /dev/null; then
    CSV_INFO=$(kubectl get csv -A -o json | jq -r '.items[] | select(.metadata.name | startswith("rhods-operator")) | "\(.metadata.name) \(.metadata.namespace)"' 2>/dev/null | head -1)
else
    # Fallback: use grep to find CSV with rhods-operator in the name
    CSV_INFO=$(kubectl get csv -A --no-headers | grep "rhods-operator" | head -1)
fi

if [[ -z "$CSV_INFO" ]]; then
    echo "❌ RHOAI operator ClusterServiceVersion not found"
    echo "   Expected a CSV with name starting with 'rhods-operator'"
    echo ""
    echo "   Available CSVs:"
    kubectl get csv -A --no-headers | grep -i rhods || echo "   No RHOAI-related CSVs found"
    exit 1
fi

# Parse the CSV name and namespace
if command -v jq &> /dev/null; then
    CSV_NAME=$(echo "$CSV_INFO" | awk '{print $1}')
    CSV_NAMESPACE=$(echo "$CSV_INFO" | awk '{print $2}')
else
    # For grep output: NAMESPACE NAME ...
    CSV_NAME=$(echo "$CSV_INFO" | awk '{print $2}')
    CSV_NAMESPACE=$(echo "$CSV_INFO" | awk '{print $1}')
fi

echo "   ✅ Found RHOAI operator CSV: $CSV_NAME"

if [[ -z "$CSV_NAMESPACE" ]]; then
    echo "❌ Could not determine namespace for CSV: $CSV_NAME"
    exit 1
fi

echo "   📍 CSV namespace: $CSV_NAMESPACE"

# Check the phase of the ClusterServiceVersion
echo "2️⃣ Checking CSV status phase..."

PHASE=$(kubectl get csv "$CSV_NAME" -n "$CSV_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null)

if [[ -z "$PHASE" ]]; then
    echo "❌ Could not retrieve phase for CSV: $CSV_NAME"
    echo "   CSV may not be ready yet"
    exit 1
fi

echo "   📊 Current phase: $PHASE"

if [[ "$PHASE" == "Succeeded" ]]; then
    echo "   ✅ RHOAI operator is successfully installed and running"
else
    echo "❌ RHOAI operator is not in Succeeded phase"
    echo "   Current phase: $PHASE"
    echo ""
    echo "   Additional information:"
    kubectl get csv "$CSV_NAME" -n "$CSV_NAMESPACE" -o yaml | grep -A 10 -B 5 "status:"
    exit 1
fi

echo ""
echo "========================================="
echo "✅ RHOAI Validation Complete!"
echo "========================================="
echo ""
echo "RHOAI operator is properly installed and running."
echo "CSV: $CSV_NAME"
echo "Namespace: $CSV_NAMESPACE"
echo "Phase: $PHASE"
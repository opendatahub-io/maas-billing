#!/bin/bash
set -euo pipefail

# Grafana Operator Installation Script for MaaS Deployment
# Handles both OpenShift operator installation and vanilla Kubernetes deployment
# Required for dashboard visualization and monitoring

OCP=true

usage() {
  cat <<EOF
Usage: $0 [--kubernetes]

Options:
  --kubernetes    Use vanilla Kubernetes Grafana instead of OpenShift Grafana operator

Examples:
  $0                # Install OpenShift Grafana operator (default)
  $0 --kubernetes   # Install vanilla Grafana (not implemented yet)
EOF
  exit 1
}

# Parse flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    --kubernetes)  OCP=false ; shift ;;
    -h|--help) usage ;;
    *) echo "❌ Unknown option: $1"; usage ;;
  esac
done

echo "📊 Setting up Grafana for MaaS observability"

if [[ "$OCP" == true ]]; then
  echo "Using OpenShift Grafana operator"
  
  echo "🔧 Installing Grafana operator subscription..."
  kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: grafana-operator
  namespace: openshift-operators
spec:
  channel: v5
  installPlanApproval: Automatic
  name: grafana-operator
  source: community-operators
  sourceNamespace: openshift-marketplace
EOF

  echo "⏳ Waiting for Grafana operator to be ready..."
  kubectl wait --for=condition=CatalogSourcesUnhealthy=false subscription/grafana-operator -n openshift-operators --timeout=60s

  # Wait for the operator deployment to be available
  echo "⏳ Waiting for Grafana operator deployment..."
  kubectl wait --for=condition=Available deployment/grafana-operator-controller-manager-v5 -n openshift-operators --timeout=60s

  echo "✅ Grafana operator is installed and running"

else
  echo "❌ Vanilla Kubernetes Grafana installation not implemented yet, skipping"
fi

echo "📊 Grafana operator installation completed!"
echo "Note: To deploy a Grafana instance, apply the Grafana custom resources from infrastructure/kustomize-templates/grafana/"

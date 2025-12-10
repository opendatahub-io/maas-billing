#!/bin/bash
set -euo pipefail

# Prometheus Operator Installation Script for MaaS Deployment
# Handles both OpenShift built-in monitoring and vanilla Prometheus Operator
# Required for observability and metrics collection

OCP=false

usage() {
  cat <<EOF
Usage: $0 [--ocp]

Options:
  --ocp    Validate OpenShift built-in monitoring instead of installing Prometheus Operator

Examples:
  $0           # Install Prometheus Operator
  $0 --ocp     # Validate OpenShift built-in monitoring
EOF
  exit 1
}

# Parse flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    --ocp)  OCP=true ; shift ;;
    -h|--help) usage ;;
    *) echo "❌ Unknown option: $1"; usage ;;
  esac
done

echo "📊 Setting up Prometheus monitoring for MaaS observability"

if [[ "$OCP" == true ]]; then
  echo "Using OpenShift built-in monitoring"
  
  echo "🔍 Validating OpenShift monitoring is available..."
  if ! kubectl get namespace openshift-monitoring >/dev/null 2>&1; then
    echo "❌ OpenShift monitoring namespace not found. Please ensure OpenShift monitoring is enabled."
    exit 1
  fi
  
  echo "⏳ Validating OpenShift monitoring components are running..."
  if ! kubectl get pods -n openshift-monitoring --field-selector=status.phase=Running | grep -q prometheus-operator >/dev/null 2>&1; then
    echo "❌ OpenShift monitoring operator is not running"
    exit 1
  fi
  
  echo "✅ OpenShift built-in monitoring is available and running"
else
  echo "Using vanilla Prometheus Operator"
  
  # Install Prometheus Operator
  echo "🔧 Installing Prometheus Operator..."
  kubectl apply --server-side --force-conflicts --field-manager=quickstart-installer \
    -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/master/bundle.yaml
  
  echo "⏳ Waiting for Prometheus Operator to be ready..."
  kubectl wait --for=condition=Available deployment/prometheus-operator -n default --timeout=300s
fi

echo "📊 Access Prometheus metrics (in separate terminals):"
echo "   kubectl port-forward -n llm-observability svc/models-aas-observability 9090:9090"
echo "   kubectl port-forward -n kuadrant-system svc/limitador-limitador 8080:8080"
echo "   Then visit: http://localhost:9090 (Prometheus) and http://localhost:8080/metrics (Limitador)"
echo

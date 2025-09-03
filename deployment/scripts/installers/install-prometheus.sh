#!/bin/bash
set -euo pipefail

# Prometheus Operator Installation Script for MaaS Deployment
# Required for observability and metrics collection

echo "📊 Installing Prometheus Operator for MaaS observability"

# Install Prometheus Operator
echo "🔧 Installing Prometheus Operator..."
kubectl apply --server-side --force-conflicts --field-manager=quickstart-installer \
  -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/master/bundle.yaml

echo "⏳ Waiting for Prometheus Operator to be ready..."
kubectl wait --for=condition=Available deployment/prometheus-operator -n default --timeout=300s

echo "📊 Access Prometheus metrics (in separate terminals):"
echo "   kubectl port-forward -n llm-observability svc/models-aas-observability 9090:9090"
echo "   kubectl port-forward -n kuadrant-system svc/limitador-limitador 8080:8080"
echo "   Then visit: http://localhost:9090 (Prometheus) and http://localhost:8080/metrics (Limitador)"
echo

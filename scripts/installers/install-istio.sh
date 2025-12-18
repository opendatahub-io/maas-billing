#!/bin/bash
set -euo pipefail

# Istio Installation Script for MaaS Deployment
# Based on deployment/kuadrant-openshift/istio-install.sh

TAG=1.26.2
HUB=gcr.io/istio-release

echo "🚢 Installing Istio for MaaS deployment"
echo "Using Istio version: $TAG from $HUB"

# Check if helm is available
if ! command -v helm &> /dev/null; then
    echo "❌ helm not found. Please install helm first."
    exit 1
fi

# Install Gateway API CRDs first (using v1.3.0 like original)
echo "Checking for Gateway API CRDs..."
if kubectl get crd gatewayclasses.gateway.networking.k8s.io &> /dev/null; then
    echo "Gateway API CRDs already exist (managed by OpenShift Ingress Operator)"
else
    echo "Installing Gateway API CRDs v1.3.0..."
    kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.3.0/standard-install.yaml
fi

# Install Istio base using OCI registry (like original)
echo "🔧 Installing Istio base from OCI registry..."
helm upgrade -i istio-base oci://$HUB/charts/base --version $TAG -n istio-system --create-namespace

# Install Istiod using OCI registry (like original)
echo "🔧 Installing Istiod from OCI registry..."
helm upgrade -i istiod oci://$HUB/charts/istiod --version $TAG -n istio-system --set tag=$TAG --set hub=$HUB --wait

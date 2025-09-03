#!/bin/bash
set -euo pipefail

# cert-manager Installation Script for MaaS Deployment
# Required for webhook certificates and TLS management

CERT_MANAGER_VERSION=v1.18.2

echo "🔒 Installing cert-manager for MaaS deployment"
echo "Using cert-manager version: $CERT_MANAGER_VERSION"

# Install cert-manager
echo "🔧 Installing cert-manager..."
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml

echo "⏳ Waiting for cert-manager to be ready..."
kubectl wait --for=condition=Available deployment/cert-manager            -n cert-manager --timeout=300s
kubectl wait --for=condition=Available deployment/cert-manager-cainjector -n cert-manager --timeout=300s
kubectl wait --for=condition=Available deployment/cert-manager-webhook   -n cert-manager --timeout=300s

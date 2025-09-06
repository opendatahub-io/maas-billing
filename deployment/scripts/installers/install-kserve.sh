#!/bin/bash
set -euo pipefail

# KServe Installation Script for MaaS Deployment
# Handles both OpenShift Serverless validation and vanilla KServe installation

KSERVE_VERSION=v0.15.2
OCP=false

usage() {
  cat <<EOF
Usage: $0 [--ocp]

Options:
  --ocp    Validate OpenShift Serverless instead of installing vanilla KServe

Examples:
  $0           # Install vanilla KServe
  $0 --ocp     # Validate OpenShift Serverless
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

echo "🤖 Installing model serving platform for MaaS deployment"

if [[ "$OCP" == true ]]; then
  echo "Using OpenShift Serverless for OpenShift clusters"
  
  echo "🔍 Validating OpenShift Serverless operator is installed..."
  if ! kubectl get subscription serverless-operator -n openshift-serverless >/dev/null 2>&1; then
    echo "❌ OpenShift Serverless operator not found. Please install it first."
    exit 1
  fi
  
  echo "⏳ Validating OpenShift Serverless controller is running..."
  if ! kubectl wait --for=condition=ready pod --all -n openshift-serverless --timeout=60s >/dev/null 2>&1; then
    echo "❌ OpenShift Serverless controller is not ready"
    exit 1
  fi
  
  echo "✅ OpenShift Serverless operator is installed and running"
else
  echo "Using vanilla KServe version: $KSERVE_VERSION"
  
  echo "🔧 Installing KServe..."
  kubectl apply --server-side -f https://github.com/kserve/kserve/releases/download/${KSERVE_VERSION}/kserve.yaml
  
  echo "⏳ Waiting for KServe controller..."
  kubectl wait --for=condition=Available deployment/kserve-controller-manager -n kserve --timeout=300s
fi

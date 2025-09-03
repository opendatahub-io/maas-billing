#!/bin/bash
set -euo pipefail

# KServe Installation Script for MaaS Deployment
# Handles both OpenShift Serverless and vanilla KServe

KSERVE_VERSION=v0.15.2
OCP=false

usage() {
  cat <<EOF
Usage: $0 [--ocp]

Options:
  --ocp    Install OpenShift Serverless instead of vanilla KServe

Examples:
  $0           # Install vanilla KServe
  $0 --ocp     # Install OpenShift Serverless
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
  
  echo "🔧 Installing OpenShift Serverless operator..."
  kubectl apply --server-side -f https://raw.githubusercontent.com/kserve/kserve/refs/heads/master/docs/openshift/serverless/operator.yaml
  
  echo "⏳ Waiting for OpenShift Serverless controller..."
  kubectl wait --for=condition=ready pod --all -n openshift-serverless --timeout=300s
else
  echo "Using vanilla KServe version: $KSERVE_VERSION"
  
  echo "🔧 Installing KServe..."
  kubectl apply --server-side -f https://github.com/kserve/kserve/releases/download/${KSERVE_VERSION}/kserve.yaml
  
  echo "⏳ Waiting for KServe controller..."
  kubectl wait --for=condition=Available deployment/kserve-controller-manager -n kserve --timeout=300s
fi

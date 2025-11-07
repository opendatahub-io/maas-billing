#!/bin/bash

# This script automates the complete deployment and validation of the MaaS platform on OpenShift

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Options (can be set as environment variables)
SKIP_VALIDATION=${SKIP_VALIDATION:-false}
SKIP_SMOKE=${SKIP_SMOKE:-true}

print_header() {
    echo ""
    echo "----- $1 -----"
}

check_prerequisites() {
    echo "Checking prerequisites..."
    
    # Check if we're on OpenShift
    if ! kubectl api-resources | grep "route.openshift.io"; then
        echo "❌ ERROR: This script is designed for OpenShift clusters only"
        exit 1
    fi
    
    # Check if we're logged in
    if ! oc whoami >/dev/null 2>&1; then
        echo "❌ ERROR: Not logged into OpenShift. Please run 'oc login' first"
        exit 1
    fi
    
    echo "✅ Prerequisites met - logged in as: $(oc whoami)"
}

deploy_maas_platform() {
    echo "Deploying MaaS platform on OpenShift..."
    
    if [ ! -f "$PROJECT_ROOT/deployment/scripts/deploy-openshift.sh" ]; then
        echo "❌ ERROR: Deployment script not found: $PROJECT_ROOT/deployment/scripts/deploy-openshift.sh"
        exit 1
    fi
    
    if ! "$PROJECT_ROOT/deployment/scripts/deploy-openshift.sh"; then
        echo "❌ ERROR: MaaS platform deployment failed"
        exit 1
    fi
    
    echo "✅ MaaS platform deployment completed"
}

deploy_models() {
    echo "Deploying simulator Model"
    if ! (cd "$PROJECT_ROOT" && kustomize build docs/samples/models/simulator/ | kubectl apply -f -); then
        echo "❌ ERROR: Failed to deploy simulator model"
        exit 1
    fi
    echo "✅ Simulator model deployed"
    
    echo "Deploying facebook-opt-125m-cpu Model"
    if ! (cd "$PROJECT_ROOT" && kustomize build docs/samples/models/facebook-opt-125m-cpu/ | kubectl apply -f -); then
        echo "❌ ERROR: Failed to deploy facebook-opt-125m-cpu model"
        exit 1
    fi
    echo "✅ Facebook-opt-125m-cpu model deployed"
    
    echo "Waiting 60 seconds for models to initialize..."
    sleep 60
    echo "✅ Model initialization wait completed"
}

validate_deployment() {
    echo "Deployment Validation"
    if [ "$SKIP_VALIDATION" = false ]; then
        if ! "$PROJECT_ROOT/deployment/scripts/validate-deployment.sh"; then
            echo "❌ ERROR: Deployment validation failed"
            exit 1
        fi
        echo "✅ Deployment validation completed"
    else
        echo "⏭️  Skipping validation"
    fi
}

setup_maas_users() {
    echo "Setting up Maas users for testing"
    if ! (cd "$PROJECT_ROOT" && bash test/e2e/setup-maas-users.sh); then
        echo "❌ ERROR: Failed to setup Maas users"
        exit 1
    fi
    echo "✅ Maas users setup completed"
}

run_smoke_tests() {
    echo "-- Smoke Testing --"
    export CLUSTER_DOMAIN="$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')"    
    if [ -z "$CLUSTER_DOMAIN" ]; then
        echo "❌ ERROR: Failed to retrieve OpenShift cluster domain"
        exit 1
   fi
    
    export HOST="maas.${CLUSTER_DOMAIN}"
    export MAAS_API_BASE_URL="http://${HOST}/maas-api"
    
    if [ "$SKIP_SMOKE" = false ]; then
        if ! (cd "$PROJECT_ROOT" && bash test/e2e/smoke.sh); then
            echo "❌ ERROR: Smoke tests failed"
            exit 1
        fi
        echo "✅ Smoke tests completed successfully"
    else
        echo "⏭️  Skipping smoke tests"
    fi
}

# Main execution
print_header "Deploying Maas on OpenShift"
check_prerequisites
deploy_maas_platform

print_header "Deploying Models"  
deploy_models

print_header "Validating Deployment"
validate_deployment

print_header "Setup maas users for testing"
setup_maas_users

print_header "Running Maas e2e Tests"
run_smoke_tests

echo "🎉 Deployment completed successfully!"
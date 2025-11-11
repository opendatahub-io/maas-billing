#!/bin/bash

# This script automates the complete deployment and validation of the MaaS platform on OpenShift

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Options (can be set as environment variables)
SKIP_VALIDATION=${SKIP_VALIDATION:-false}
SKIP_SMOKE=${SKIP_SMOKE:-false}

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
    
    echo "Waiting for model to be ready..."
    sleep 30
    echo "✅ Simulator Model deployed"
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
    if ! (cd "$PROJECT_ROOT" && bash test/e2e/scripts/setup_maas_users_openshift.sh); then
        echo "❌ ERROR: Failed to setup Maas users on OpenShift"
        exit 1
    fi
    
    # Source the environment variables created by the setup script
    local env_file="$PROJECT_ROOT/maas-users.env"
    if [ -f "$env_file" ]; then
        source "$env_file"
    else
        echo "❌ ERROR: Users credentials not found"
        exit 1
    fi
    
    echo "✅ Maas users setup completed"
}

login_as_user() {
    echo "Logging in as user: $1"
    if ! (oc login -u "$1" -p "$2"); then
        echo "❌ ERROR: Failed to login as user: $1"
        exit 1
    fi
    echo "✅ User: $1 logged in successfully as: $(oc whoami)"
}

setup_vars_for_tests() {
    echo "-- Setting up variables for tests --"
    CLUSTER_DOMAIN="$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')"
    export CLUSTER_DOMAIN
    if [ -z "$CLUSTER_DOMAIN" ]; then
        echo "❌ ERROR: Failed to retrieve OpenShift cluster domain"
        exit 1
    fi
    
    export HOST="maas.${CLUSTER_DOMAIN}"
    export MAAS_API_BASE_URL="http://${HOST}/maas-api"

    echo "CLUSTER_DOMAIN: ${CLUSTER_DOMAIN}"
    echo "HOST: ${HOST}"
    echo "MAAS_API_BASE_URL: ${MAAS_API_BASE_URL}"

    echo "✅ Variables for tests setup completed"
}

run_smoke_tests() {
    echo "-- Smoke Testing --"
    
    if [ "$SKIP_SMOKE" = false ]; then
        if ! (cd "$PROJECT_ROOT" && bash test/e2e/smoke.sh); then
            echo "❌ ERROR: Smoke tests failed"
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

print_header "Setting up variables for tests"
setup_vars_for_tests

print_header "Setup maas users for testing"
setup_maas_users

print_header "Running Maas e2e Tests as admin user"
login_as_user "${OPENSHIFT_ADMIN_USER}" "${OPENSHIFT_ADMIN_PASS}"
run_smoke_tests

print_header "Running Maas e2e Tests as dev user"
login_as_user "${OPENSHIFT_DEV_USER}" "${OPENSHIFT_DEV_PASS}"
run_smoke_tests

echo "🎉 Deployment completed successfully!"
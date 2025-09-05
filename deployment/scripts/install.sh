#!/usr/bin/env bash

set -euo pipefail

# Script to install MaaS deployment with configurable options
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOYMENT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default values
DEPLOYMENT_TYPE=${DEPLOYMENT_TYPE:-simulator}
TEARDOWN=false
AVAILABLE_DEPLOYMENTS=()
# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_usage() {
    echo "Usage: $0 [--teardown] [deployment-type]"
    echo
    echo "Install MaaS deployment with optional teardown and configurable deployment type."
    echo
    echo "Options:"
    echo "  --teardown    Run teardown before installation (optional)"
    echo "  --help        Show this help message"
    echo
    echo "Deployment Types:"
    for deployment in "${AVAILABLE_DEPLOYMENTS[@]}"; do
        echo "  $deployment"
    done
    echo
    echo "Default: $DEPLOYMENT_TYPE"
    echo
    echo "Examples:"
    echo "  $0                    # Install simulator deployment (default)"
    echo "  $0 gpu                # Install gpu deployment"
    echo "  $0 --teardown basic   # Teardown first, then install basic deployment"
    echo "  $0 --teardown         # Teardown first, then install simulator deployment"
}

detect_available_deployments() {
    # Only detect if not already done
    if [ ${#AVAILABLE_DEPLOYMENTS[@]} -eq 0 ]; then
        log_info "Detecting available deployments..."
        
        if [ ! -d "$DEPLOYMENT_DIR/examples" ]; then
            log_error "Examples directory not found: $DEPLOYMENT_DIR/examples"
            exit 1
        fi
        
        for dir in "$DEPLOYMENT_DIR/examples"/*; do
            if [ -d "$dir" ] && [ -f "$dir/kustomization.yaml" ]; then
                deployment_name=$(basename "$dir")
                # Remove "-deployment" suffix if present for cleaner names
                deployment_name=${deployment_name%-deployment}
                AVAILABLE_DEPLOYMENTS+=("$deployment_name")
            fi
        done
        
        if [ ${#AVAILABLE_DEPLOYMENTS[@]} -eq 0 ]; then
            log_error "No valid deployments found in examples directory"
            exit 1
        fi
        
        log_info "Found deployments: ${AVAILABLE_DEPLOYMENTS[*]}"
    fi
}

validate_deployment() {
    local deployment="$1"
    
    # Add "-deployment" suffix if not present
    local deployment_dir="$DEPLOYMENT_DIR/examples/${deployment}-deployment"
    if [ ! -d "$deployment_dir" ]; then
        deployment_dir="$DEPLOYMENT_DIR/examples/${deployment}"
    fi
    
    if [ ! -d "$deployment_dir" ] || [ ! -f "$deployment_dir/kustomization.yaml" ]; then
        log_error "Invalid deployment type: $deployment"
        log_error "Available deployments: ${AVAILABLE_DEPLOYMENTS[*]}"
        exit 1
    fi
    
    echo "$deployment_dir"
}

set_cluster_domain() {
    # Set default CLUSTER_DOMAIN if not already set
    if [ -z "${CLUSTER_DOMAIN:-}" ]; then
        if kubectl get ingresses.config.openshift.io cluster &>/dev/null; then
            # OpenShift cluster - get domain from ingress config
            export CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
        else
            # Fallback for non-OpenShift clusters
            export CLUSTER_DOMAIN="cluster.local"
        fi
        log_info "Detected CLUSTER_DOMAIN: $CLUSTER_DOMAIN"
    else
        log_info "Using existing CLUSTER_DOMAIN: $CLUSTER_DOMAIN"
    fi
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --teardown)
                TEARDOWN=true
                shift
                ;;
            --help)
                detect_available_deployments
                show_usage
                exit 0
                ;;
            --*)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
            *)
                DEPLOYMENT_TYPE="$1"
                shift
                ;;
        esac
    done
}

main() {
    # Detect available deployments first
    detect_available_deployments
    
    # Show help if no arguments provided
    if [ $# -eq 0 ]; then
        show_usage
        exit 0
    fi
    
    log_info "Starting MaaS deployment installation"
    
    # Parse command line arguments
    parse_arguments "$@"
    
    # Validate deployment type
    DEPLOYMENT_PATH=$(validate_deployment "$DEPLOYMENT_TYPE")
    log_info "Using deployment: $DEPLOYMENT_TYPE ($(basename "$DEPLOYMENT_PATH"))"
    
    # Set cluster domain
    set_cluster_domain
    
    # Change to deployment directory
    cd "$DEPLOYMENT_DIR"
    
    # Optional teardown
    if [ "$TEARDOWN" = true ]; then
        log_info "Running teardown..."
        if [ -f "scripts/teardown.sh" ]; then
            scripts/teardown.sh
            log_success "Teardown completed"
        else
            log_error "Teardown script not found: scripts/teardown.sh"
            exit 1
        fi
    fi
    
    # Install dependencies
    log_info "Installing dependencies..."
    scripts/install-dependencies.sh --all
    log_success "Dependencies installed"
    
    # Clean up any conflicting operators
    log_info "Cleaning up conflicting operators..."
    kubectl -n gateway-system delete subscription sailoperator --ignore-not-found
    kubectl -n gateway-system delete csv sailoperator.v0.1.0 --ignore-not-found
    kubectl -n gateway-system delete deployment sail-operator --ignore-not-found
    kubectl -n gateway-system delete deployment istiod --ignore-not-found
    
    # Deploy Kuadrant operators first
    log_info "Deploying Kuadrant operators..."
    kustomize build core-infrastructure/kustomize-templates/kuadrant | envsubst | kubectl apply -f -
    log_success "Kuadrant operators deployed"
    
    # Wait for operators to be ready
    log_info "Waiting for operators to be ready..."
    kubectl wait --for=condition=available deployment/kuadrant-operator-controller-manager -n kuadrant-system --timeout=300s && \
    kubectl wait --for=condition=available deployment/limitador-operator-controller-manager -n kuadrant-system --timeout=300s && \
    kubectl wait --for=condition=available deployment/authorino-operator -n kuadrant-system --timeout=300s
    log_success "All operators are ready"
    
    # Deploy using overlay (always use OpenShift overlay for external access)
    log_info "Deploying $DEPLOYMENT_TYPE with external access..."
    kustomize build overlays/openshift | envsubst | kubectl apply -f -
    log_success "Deployment completed successfully!"
    
    # Show access information
    echo
    log_success "=== Deployment Complete ==="
    log_info "Cluster Domain: $CLUSTER_DOMAIN"
    log_info "External Routes:"
    echo "  - Simulator: simulator-llm.$CLUSTER_DOMAIN"
    echo "  - Qwen3: qwen3-llm.$CLUSTER_DOMAIN"
    echo "  - Key Manager: key-manager.$CLUSTER_DOMAIN"
    echo
    log_info "Check deployment status with:"
    echo "  kubectl get pods -n llm"
    echo "  kubectl get routes -n llm"
}

main "$@"

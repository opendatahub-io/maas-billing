#!/usr/bin/env bash

set -euo pipefail

# Script to install MaaS deployment with configurable options
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOYMENT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default values
DEPLOYMENT_TYPE=${DEPLOYMENT_TYPE:-simulator}
TEARDOWN=false
TEARDOWN_ONLY=false
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

run_teardown() {
    set -eu

    current_cluster=$(oc whoami --show-server 2>/dev/null || echo "Unable to determine cluster")
    current_user=$(oc whoami 2>/dev/null || echo "Unable to determine user")

    echo "Current cluster: $current_cluster"
    echo "Current user: $current_user"
    echo ""
    echo "This will delete the following namespaces and all their resources:"
    echo "  - llm"
    echo "  - llm-observability" 
    echo "  - kuadrant-system"
    echo ""
    read -p "Are you sure you want to proceed? (y/N): " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Teardown cancelled."
        return 0
    fi

    echo "Commencing teardown of MaaS deployment"

    echo "Uninstalling Helm releases..."
    # Uninstall Kuadrant operator releases
    helm uninstall kuadrant-operator -n kuadrant-system --ignore-not-found || true
    helm uninstall authorino-operator -n kuadrant-system --ignore-not-found || true
    helm uninstall limitador-operator -n kuadrant-system --ignore-not-found || true
    
    # Uninstall Istio releases
    helm uninstall istiod -n istio-system --ignore-not-found || true
    helm uninstall istio-base -n istio-system --ignore-not-found || true
    
    # Clean up any old Istio releases in gateway-system
    helm uninstall default-istiod -n gateway-system --ignore-not-found || true
    
    # Remove stale Istio webhook configurations pointing to wrong namespaces
    kubectl delete mutatingwebhookconfiguration istio-sidecar-injector-gateway-system --ignore-not-found || true
    kubectl delete validatingwebhookconfiguration istio-validator-gateway-system --ignore-not-found || true
    
    # Uninstall cert-manager (if installed via Helm)
    helm uninstall cert-manager -n cert-manager --ignore-not-found || true
    
    # Uninstall KServe (if installed via Helm)
    helm uninstall kserve -n kserve --ignore-not-found || true
    helm uninstall kserve-crd -n kserve --ignore-not-found || true

    echo "Removing any remaining CRDs not handled by Helm uninstall..."
    # Only remove CRDs that might not be handled by Helm uninstall
    REMAINING_CRDS=(
      authpolicies.kuadrant.io
      dnspolicies.kuadrant.io
      dnsrecords.kuadrant.io
      tlspolicies.kuadrant.io
    )

    for crd in "${REMAINING_CRDS[@]}"; do
      if kubectl get crd "$crd" >/dev/null 2>&1; then
        echo "Deleting remaining CRD: $crd"
        kubectl delete crd "$crd" --ignore-not-found --wait=true || true
      fi
    done

    echo "Removing Kuadrant-related ClusterRoles and ClusterRoleBindings..."
    CLUSTER_ROLES=(
      authorino-authconfig-editor-role
      authorino-authconfig-viewer-role
      authorino-manager-k8s-auth-role
      authorino-manager-role
      key-manager-kuadrant-restart
      kuadrant-operator-metrics-reader
      limitador-operator-metrics-reader
    )

    for cr in "${CLUSTER_ROLES[@]}"; do
      if kubectl get clusterrole "$cr" >/dev/null 2>&1; then
        echo "Deleting ClusterRole: $cr"
        kubectl delete clusterrole "$cr" --ignore-not-found || true
      fi
      # Also delete corresponding ClusterRoleBinding if it exists
      if kubectl get clusterrolebinding "$cr" >/dev/null 2>&1; then
        echo "Deleting ClusterRoleBinding: $cr"
        kubectl delete clusterrolebinding "$cr" --ignore-not-found || true
      fi
    done

    echo "Removing Kuadrant-related ServiceAccounts and RBAC resources..."
    # Clean up ServiceAccounts in kuadrant-system namespace
    kubectl delete serviceaccount -n kuadrant-system --all --ignore-not-found || true
    
    # Clean up Roles and RoleBindings in kuadrant-system namespace  
    kubectl delete role -n kuadrant-system --all --ignore-not-found || true
    kubectl delete rolebinding -n kuadrant-system --all --ignore-not-found || true

    namespaces="llm llm-observability kuadrant-system"
    for ns in $namespaces; do
        echo "Deleting namespace: $ns"
        if [ "$ns" = "kuadrant-system" ]; then
            echo "Removing finalizers from kuadrant-system resources"
            kubectl patch authorino authorino -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
            kubectl patch kuadrant kuadrant -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
            kubectl patch limitador limitador -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true

            kubectl get kuadrants.kuadrant.io -n "$ns" -o name 2>/dev/null | xargs -r -I {} kubectl patch {} -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge || true
            kubectl get limitadors.limitador.kuadrant.io -n "$ns" -o name 2>/dev/null | xargs -r -I {} kubectl patch {} -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge || true
            kubectl get authorinos.operator.authorino.kuadrant.io -n "$ns" -o name 2>/dev/null | xargs -r -I {} kubectl patch {} -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge || true

            oc delete project "$ns" --force --grace-period=0 --timeout=60s || true

            sleep 5
            if oc get project "$ns" >/dev/null 2>&1; then
                echo "Force removing finalizers from kuadrant-system namespace"
                oc patch project "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge || true

                sleep 5
                if oc get project "$ns" >/dev/null 2>&1; then
                    echo "Namespace still stuck, attempting direct deletion"
                    oc delete namespace "$ns" --force --grace-period=0 --timeout=30s || true
                fi
            fi
        else
            oc delete project "$ns" || true
        fi
    done
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
    local teardown_only=false
    
    # Check if only --teardown flag is provided
    if [ $# -eq 1 ] && [ "$1" = "--teardown" ]; then
        teardown_only=true
    fi
    
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
    
    # If only teardown was requested, set flag to exit after teardown
    if [ "$teardown_only" = true ]; then
        TEARDOWN_ONLY=true
    fi
}

main() {
    # Detect available deployments first
    detect_available_deployments
    
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
        run_teardown
        log_success "Teardown completed"
        
        # Exit if only teardown was requested
        if [ "$TEARDOWN_ONLY" = true ]; then
            exit 0
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
    
    # Apply Kuadrant configuration (CRs) after dependencies installed it via Helm
    log_info "Configuring Kuadrant CRs..."
    kustomize build core-infrastructure/kustomize-templates/kuadrant | envsubst | kubectl apply -f -
    log_success "Kuadrant configured"
    
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
    echo "  - MaaS API: maas-api.$CLUSTER_DOMAIN"
    echo
    log_info "Check deployment status with:"
    echo "  kubectl get pods -n llm"
    echo "  kubectl get routes -n llm"
}

main "$@"

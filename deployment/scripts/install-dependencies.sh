#!/usr/bin/env bash

set -euo pipefail

# Install Dependencies Script for MaaS Deployment
# Orchestrates installation of required platform components
# Supports both vanilla Kubernetes and OpenShift deployments

# Component definitions with installation order
COMPONENTS=("istio" "cert-manager" "odh" "kserve" "prometheus" "kuadrant"  "grafana")

# OpenShift flag
OCP=false

KUADRANT_VERSION="v1.3.0-rc2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLERS_DIR="$SCRIPT_DIR/installers"

get_component_description() {
    case "$1" in
        istio) echo "Service mesh and Gateway API configuration" ;;
        cert-manager) echo "Certificate management for TLS and webhooks" ;;
        odh) echo "OpenDataHub operator for ML/AI platform (OpenShift only)" ;;
        kserve) 
            if [[ "$OCP" == true ]]; then
                echo "Model serving platform (validates OpenShift Serverless)"
            else
                echo "Model serving platform"
            fi
            ;;
        prometheus) 
            if [[ "$OCP" == true ]]; then
                echo "Observability and metrics collection (validates OpenShift monitoring)"
            else
                echo "Observability and metrics collection (optional)"
            fi
            ;;
        grafana) 
            if [[ "$OCP" == true ]]; then
                echo "Dashboard visualization platform (OpenShift operator)"
            else
                echo "Dashboard visualization platform (not implemented for vanilla Kubernetes)"
            fi
            ;;
        kuadrant) echo "API gateway operators via OLM (Kuadrant, Authorino, Limitador)" ;;
        *) echo "Unknown component" ;;
    esac
}

usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Install required dependencies for MaaS deployment."
    echo ""
    echo "Options:"
    echo "  --all                    Install all components"
    echo "  --istio                  Install Istio service mesh"
    echo "  --cert-manager           Install cert-manager"
    echo "  --odh                    Install OpenDataHub operator (OpenShift only)"
    echo "  --kserve                 Install KServe model serving platform"
    echo "  --prometheus             Install Prometheus operator"
    echo "  --grafana                Install Grafana dashboard platform"
    echo "  --kuadrant               Install Kuadrant operators via OLM"
    echo "  --ocp                    Use OpenShift-specific handling (validate instead of install)"
    echo "  -h, --help               Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 --all              # Install all components (vanilla Kubernetes)"
    echo "  $0 --all --ocp         # Validate all components (OpenShift)"
    echo "  $0 --kserve --ocp      # Validate OpenShift Serverless only"
    echo ""
    echo "If no options are provided, interactive mode will prompt for component selection."
    echo ""
    echo "Components are installed in the following order:"
    for component in "${COMPONENTS[@]}"; do
        echo "  - $component: $(get_component_description "$component")"
    done
}

install_component() {
    local component="$1"
    local installer_script="$INSTALLERS_DIR/install-${component}.sh"
    
    # Special handler for ODH (OpenShift only)
    if [[ "$component" == "odh" ]]; then
        if [[ "$OCP" != true ]]; then
            echo "⚠️  ODH is only available on OpenShift clusters, skipping..."
            return 0
        fi
        if [[ -f "$installer_script" ]]; then
            echo "🚀 Installing $component..."
            bash "$installer_script"
        else
            echo "❌ Installer script not found: $installer_script"
            return 1
        fi
        return 0
    fi
    
    # Inline handler for Kuadrant (installed via OLM)
    if [[ "$component" == "kuadrant" ]]; then
        # Ensure kuadrant-system namespace exists
        kubectl create namespace kuadrant-system 2>/dev/null || echo "✅ Namespace kuadrant-system already exists"

        echo "🚀 Creating Kuadrant OperatorGroup..."
        kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: kuadrant-operator-group
  namespace: kuadrant-system
spec: {}
EOF

        # Check if the CatalogSource already exists before applying
        if kubectl get catalogsource kuadrant-operator-catalog -n kuadrant-system &>/dev/null; then
            echo "✅ Kuadrant CatalogSource already exists in namespace kuadrant-system, skipping creation."
        else
            echo "🚀 Creating Kuadrant CatalogSource..."
            kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: kuadrant-operator-catalog
  namespace: kuadrant-system
spec:
  displayName: Kuadrant Operators
  grpcPodConfig:
    securityContextConfig: restricted
  image: 'quay.io/trepel/index-from-errata:rhcl-1.2.0-rc5-multiarch'
  publisher: grpc
  sourceType: grpc
  secrets:
    - kuadrant-pull-secret
EOF
        fi

        echo "🚀 Installing kuadrant operator (via OLM Subscription)..."
        kubectl apply -f - <<EOF
  apiVersion: operators.coreos.com/v1alpha1
  kind: Subscription
  metadata:
    name: rhcl-operator
    namespace: kuadrant-system
  spec:
    channel: stable
    config:
      env:
        - name: RELATED_IMAGE_WASMSHIM
          value: 'registry.stage.redhat.io/rhcl-1/wasm-shim-rhel9@sha256:6c8bbd989e532bc5ed629fc235b7da9eb3a4949b7caf9615bc282ef682ba9221'
        - name: PROTECTED_REGISTRY
          value: registry.stage.redhat.io
    installPlanApproval: Automatic
    name: rhcl-operator
    source: kuadrant-operator-catalog
    sourceNamespace: kuadrant-system
EOF
        # Wait for all operator deployments to be created
        echo "⏳ Waiting for operator deployments to be created..."
        DEPLOYMENTS=("kuadrant-operator-controller-manager" "limitador-operator-controller-manager" "authorino-operator" "dns-operator-controller-manager")
        
        for deployment in "${DEPLOYMENTS[@]}"; do
            ATTEMPTS=0
            MAX_ATTEMPTS=7
            while true; do
                if kubectl get deployment/"$deployment" -n kuadrant-system &>/dev/null; then
                    echo "✅ Deployment $deployment created"
                    break
                else
                    ATTEMPTS=$((ATTEMPTS+1))
                    if [[ $ATTEMPTS -ge $MAX_ATTEMPTS ]]; then
                        echo "⚠️  Deployment $deployment not found after $MAX_ATTEMPTS attempts, continuing..."
                        break
                    fi
                    echo "   Waiting for $deployment deployment to be created... (attempt $ATTEMPTS/$MAX_ATTEMPTS)"
                    sleep $((5 + 5 * $ATTEMPTS))
                fi
            done
        done


        echo "⏳ Waiting for operators to be ready..."
        kubectl wait --for=condition=Available deployment/kuadrant-operator-controller-manager -n kuadrant-system --timeout=300s || \
          echo "   ⚠️  Kuadrant operator taking longer than expected"
        kubectl wait --for=condition=Available deployment/limitador-operator-controller-manager -n kuadrant-system --timeout=300s || \
          echo "   ⚠️  Limitador operator taking longer than expected"  
        kubectl wait --for=condition=Available deployment/authorino-operator -n kuadrant-system --timeout=300s || \
          echo "   ⚠️  Authorino operator taking longer than expected"
        kubectl wait --for=condition=Available deployment/dns-operator-controller-manager -n kuadrant-system --timeout=180s || \
          echo "   ⚠️  DNS operator taking longer than expected"

        sleep 5

        # Patch Kuadrant for OpenShift Gateway Controller
        echo "   Patching Kuadrant operator..."
        if ! kubectl -n kuadrant-system get deployment kuadrant-operator-controller-manager -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ISTIO_GATEWAY_CONTROLLER_NAMES")]}' | grep -q "ISTIO_GATEWAY_CONTROLLER_NAMES"; then
          # Get the actual CSV name dynamically
          CSV_NAME=$(kubectl get csv -n kuadrant-system -o jsonpath='{.items[?(@.spec.displayName=="Red Hat Connectivity Link")].metadata.name}' 2>/dev/null || echo "rhcl-operator.v1.2.0")
          echo "   Patching CSV: $CSV_NAME"
          kubectl patch csv "$CSV_NAME" -n kuadrant-system --type='json' -p='[
            {
              "op": "add",
              "path": "/spec/install/spec/deployments/0/spec/template/spec/containers/0/env/-",
              "value": {
                "name": "ISTIO_GATEWAY_CONTROLLER_NAMES",
                "value": "istio.io/gateway-controller,openshift.io/gateway-controller/v1"
              }
            }
          ]'
          echo "   ✅ Kuadrant operator patched"
        else
          echo "   ✅ Kuadrant operator already configured"
        fi

        echo "✅ Successfully installed kuadrant"
        echo ""
        return 0
    fi

    if [[ ! -f "$installer_script" ]]; then
        echo "❌ Installer not found: $installer_script"
        return 1
    fi
    
    if [[ "$OCP" == true ]]; then
        echo "🚀 Setting up $component for OpenShift..."
    else
        echo "🚀 Installing $component..."
    fi
    
    # Pass --ocp flag to scripts that support it
    local script_args=()
    if [[ "$OCP" == true ]] && [[ "$component" == "kserve" || "$component" == "prometheus" || "$component" == "grafana" ]]; then
        script_args+=("--ocp")
    fi
    
    if ! "$installer_script" "${script_args[@]:-""}"; then
        if [[ "$OCP" == true ]]; then
            echo "❌ Failed to set up $component for OpenShift"
        else
            echo "❌ Failed to install $component"
        fi
        return 1
    fi
    
    if [[ "$OCP" == true ]]; then
        echo "✅ Successfully set up $component for OpenShift"
    else
        echo "✅ Successfully installed $component"
    fi
    echo ""
}

install_all() {
    if [[ "$OCP" == true ]]; then
        echo "🔧 Setting up all MaaS dependencies for OpenShift..."
    else
        echo "🔧 Installing all MaaS dependencies..."
    fi
    echo ""
    
    for component in "${COMPONENTS[@]}"; do
        install_component "$component"
    done
    
    if [[ "$OCP" == true ]]; then
        echo "🎉 All components set up successfully for OpenShift!"
    else
        echo "🎉 All components installed successfully!"
    fi
}

interactive_install() {
    echo "MaaS Dependency Installer"
    echo "========================"
    echo ""
    if [[ "$OCP" == true ]]; then
        echo "The following components will be set up for OpenShift:"
    else
        echo "The following components will be installed:"
    fi
    for component in "${COMPONENTS[@]}"; do
        echo "  - $component: $(get_component_description "$component")"
    done
    echo ""
    
    if [[ "$OCP" == true ]]; then
        read -p "Set up all components for OpenShift? (y/N): " -n 1 -r
    else
        read -p "Install all components? (y/N): " -n 1 -r
    fi
    echo ""
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        install_all
    else
        echo "Setup cancelled."
        exit 0
    fi
}

# Parse command line arguments
# Handle special case: only --ocp flag provided (should go to interactive mode)
if [[ $# -eq 1 ]] && [[ "$1" == "--ocp" ]]; then
    OCP=true
    interactive_install
    exit 0
elif [[ $# -eq 0 ]]; then
    # No arguments - use interactive mode
    interactive_install
    exit 0
fi

# First pass: check for --ocp flag (scan without consuming arguments)
for arg in "$@"; do
    if [[ "$arg" == "--ocp" ]]; then
        OCP=true
        break
    fi
done

# Second pass: process component and action flags
COMPONENT_SELECTED=false
while [[ $# -gt 0 ]]; do
    case $1 in
        --all)
            install_all
            exit 0
            ;;
        --istio)
            install_component "istio"
            COMPONENT_SELECTED=true
            ;;
        --cert-manager)
            install_component "cert-manager"
            COMPONENT_SELECTED=true
            ;;
        --odh)
            install_component "odh"
            COMPONENT_SELECTED=true
            ;;
        --kserve)
            install_component "kserve"
            COMPONENT_SELECTED=true
            ;;
        --prometheus)
            install_component "prometheus"
            COMPONENT_SELECTED=true
            ;;
        --grafana)
            install_component "grafana"
            COMPONENT_SELECTED=true
            ;;
        --ocp)
            # Already processed in first pass, skip
            ;;
        --kuadrant)
            install_component "kuadrant"
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo ""
            usage
            exit 1
            ;;
    esac
    shift
done

# Show success message if components were installed
if [[ "$COMPONENT_SELECTED" == true ]]; then
    if [[ "$OCP" == true ]]; then
        echo "🎉 Selected components set up successfully for OpenShift!"
    else
        echo "🎉 Selected components installed successfully!"
    fi
fi


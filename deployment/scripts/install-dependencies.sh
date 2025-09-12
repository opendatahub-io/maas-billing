#!/usr/bin/env bash

set -euo pipefail

# Install Dependencies Script for MaaS Deployment
# Orchestrates installation of required platform components
# Supports both vanilla Kubernetes and OpenShift deployments

# Component definitions with installation order
COMPONENTS=("istio" "cert-manager" "kserve" "prometheus" "kuadrant"  "grafana")

# OpenShift flag
OCP=false

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLERS_DIR="$SCRIPT_DIR/installers"

get_component_description() {
    case "$1" in
        istio) echo "Service mesh and Gateway API configuration" ;;
        cert-manager) echo "Certificate management for TLS and webhooks" ;;
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
        kuadrant) echo "API gateway operators via Helm (Kuadrant, Authorino, Limitador)" ;;
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
    echo "  --kserve                 Install KServe model serving platform"
    echo "  --prometheus             Install Prometheus operator"
    echo "  --grafana                Install Grafana dashboard platform"
    echo "  --kuadrant               Install Kuadrant operators via Helm"
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
    
    # Inline handler for Kuadrant (installed via Helm)
    if [[ "$component" == "kuadrant" ]]; then
        echo "🚀 Installing kuadrant (via Helm)..."
        NAMESPACE=${NAMESPACE:-kuadrant-system}
        KUADRANT_CHART_VERSION=${KUADRANT_CHART_VERSION:-1.3.0-alpha2}
        AUTHORINO_CHART_VERSION=${AUTHORINO_CHART_VERSION:-0.21.0}
        LIMITADOR_CHART_VERSION=${LIMITADOR_CHART_VERSION:-0.15.0}
        HELM_REPO_NAME=${HELM_REPO_NAME:-kuadrant}
        HELM_REPO_URL=${HELM_REPO_URL:-https://kuadrant.io/helm-charts/}

        if ! command -v helm &> /dev/null; then
            echo "❌ helm not found. Please install helm first."
            return 1
        fi

        if helm repo list | awk '{print $1}' | grep -qx "$HELM_REPO_NAME"; then
            echo "🔄 Updating Helm repo $HELM_REPO_NAME..."
            helm repo update
        else
            echo "➕ Adding Helm repo $HELM_REPO_NAME -> $HELM_REPO_URL..."
            helm repo add "$HELM_REPO_NAME" "$HELM_REPO_URL" --force-update
        fi

        echo "📦 Installing kuadrant-operator chart ($KUADRANT_CHART_VERSION)"
        helm upgrade -i kuadrant-operator "$HELM_REPO_NAME/kuadrant-operator" \
          --version "$KUADRANT_CHART_VERSION" -n "$NAMESPACE" --create-namespace --wait

        echo "⏳ Waiting for operators to be ready..."
        kubectl wait --for=condition=Available deployment/kuadrant-operator-controller-manager -n "$NAMESPACE" --timeout=300s
        kubectl wait --for=condition=Available deployment/limitador-operator-controller-manager -n "$NAMESPACE" --timeout=300s
        kubectl wait --for=condition=Available deployment/authorino-operator-controller-manager -n "$NAMESPACE" --timeout=300s || \
          kubectl wait --for=condition=Available deployment/authorino-operator -n "$NAMESPACE" --timeout=60s || true

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
    
    if ! "$installer_script" "${script_args[@]}"; then
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

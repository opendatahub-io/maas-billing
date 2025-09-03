#!/usr/bin/env bash

set -euo pipefail

# Install Dependencies Script for MaaS Deployment
# Orchestrates installation of required platform components

# Component definitions with installation order
COMPONENTS=("istio" "cert-manager" "kserve" "prometheus")

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALLERS_DIR="$SCRIPT_DIR/installers"

get_component_description() {
    case "$1" in
        istio) echo "Service mesh and Gateway API configuration" ;;
        cert-manager) echo "Certificate management for TLS and webhooks" ;;
        kserve) echo "Model serving platform" ;;
        prometheus) echo "Observability and metrics collection (optional)" ;;
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
    echo "  -h, --help               Show this help message"
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
    
    if [[ ! -f "$installer_script" ]]; then
        echo "❌ Installer not found: $installer_script"
        return 1
    fi
    
    echo "🚀 Installing $component..."
    if ! "$installer_script"; then
        echo "❌ Failed to install $component"
        return 1
    fi
    echo "✅ Successfully installed $component"
    echo ""
}

install_all() {
    echo "🔧 Installing all MaaS dependencies..."
    echo ""
    
    for component in "${COMPONENTS[@]}"; do
        install_component "$component"
    done
    
    echo "🎉 All components installed successfully!"
}

interactive_install() {
    echo "MaaS Dependency Installer"
    echo "========================"
    echo ""
    echo "The following components will be installed:"
    for component in "${COMPONENTS[@]}"; do
        echo "  - $component: $(get_component_description "$component")"
    done
    echo ""
    
    read -p "Install all components? (y/N): " -n 1 -r
    echo ""
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        install_all
    else
        echo "Installation cancelled."
        exit 0
    fi
}

# Parse command line arguments
if [[ $# -eq 0 ]]; then
    # No arguments - use interactive mode
    interactive_install
    exit 0
fi

while [[ $# -gt 0 ]]; do
    case $1 in
        --all)
            install_all
            exit 0
            ;;
        --istio)
            install_component "istio"
            ;;
        --cert-manager)
            install_component "cert-manager"
            ;;
        --kserve)
            install_component "kserve"
            ;;
        --prometheus)
            install_component "prometheus"
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

echo "🎉 Selected components installed successfully!"
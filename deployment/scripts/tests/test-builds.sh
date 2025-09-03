#!/bin/bash
set -euo pipefail

# Build Validation Script
# Tests that all kustomization files can build successfully
# NOTE: This only validates that YAML builds - not that deployments actually work!

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAILED_BUILDS=()
PASSED_BUILDS=()

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
}

test_build() {
    local name="$1"
    local dir="$2"
    
    log_info "Testing build: $name"
    
    if [ ! -f "$dir/kustomization.yaml" ]; then
        log_fail "$name - No kustomization.yaml found"
        FAILED_BUILDS+=("$name")
        return 1
    fi
    
    # Set test environment for domain substitution
    export CLUSTER_DOMAIN="apps.test.example.com"
    
    local output_file="/tmp/build-test-${name//\//-}.yaml"
    
    if (cd "$dir" && kustomize build . > "$output_file" 2>&1); then
        local line_count=$(wc -l < "$output_file")
        log_pass "$name - Built successfully ($line_count lines)"
        PASSED_BUILDS+=("$name")
        return 0
    else
        log_fail "$name - Build failed"
        echo "Error details:"
        head -5 "$output_file"
        FAILED_BUILDS+=("$name")
        return 1
    fi
}

main() {
    cd "$(dirname "$0")/.."
    
    echo "🔨 Testing Kustomization Builds"
    echo "================================="
    echo "This validates that all kustomize configurations build successfully."
    echo "NOTE: Build success does not guarantee deployment will work!"
    echo
    
    # Check prerequisites
    if ! command -v kustomize &> /dev/null; then
        log_fail "kustomize not found. Please install kustomize first."
        exit 1
    fi
    
    # Test core infrastructure
    test_build "core-infrastructure" "../core-infrastructure"
    
    # Test all example deployments
    for example_dir in ../examples/*/; do
        if [ -d "$example_dir" ] && [ -f "$example_dir/kustomization.yaml" ]; then
            example_name="examples/$(basename "$example_dir")"
            test_build "$example_name" "$example_dir"
        fi
    done
    
    # Test component templates (models, auth, observability)
    for component_dir in ../examples/kustomize-templates/*/; do
        if [ -d "$component_dir" ] && [ -f "$component_dir/kustomization.yaml" ]; then
            component_name="components/$(basename "$component_dir")"
            test_build "$component_name" "$component_dir"
        fi
    done
    
    # Summary
    echo
    echo "🧪 BUILD TEST SUMMARY"
    echo "====================="
    echo "Passed: ${#PASSED_BUILDS[@]}"
    echo "Failed: ${#FAILED_BUILDS[@]}"
    
    if [ ${#FAILED_BUILDS[@]} -eq 0 ]; then
        log_pass "All builds successful! ✅"
        echo
        echo "Next steps:"
        echo "1. Set CLUSTER_DOMAIN environment variable"  
        echo "2. Deploy with: kustomize build <directory> | envsubst | kubectl apply -f -"
        exit 0
    else
        log_fail "Some builds failed! ❌"
        echo "Failed builds:"
        for failed in "${FAILED_BUILDS[@]}"; do
            echo "  - $failed"
        done
        exit 1
    fi
}

main "$@"

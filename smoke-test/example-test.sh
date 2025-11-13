#!/bin/bash
# Container runtime: Podman (Docker compatible)
# Replace 'podman' with 'docker' if using Docker

# Example test showing how to use the common framework
# This is a minimal example for demonstration purposes

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source common test framework
source "${SCRIPT_DIR}/common.sh"

set -euo pipefail

# ==========================================
# Test Functions
# ==========================================

test_api_health() {
    print_subheader "Test: API Health"
    
    log_check "Testing /health endpoint"
    
    # Health endpoint doesn't need authentication
    local response=$(curl -sSk -w "\n%{http_code}" "${MAAS_API_BASE_URL}/health" 2>/dev/null || echo -e "\n000")
    parse_response "$response"
    
    local http_code=$(get_response_code)
    local body=$(get_response_body)
    
    cleanup_response_files
    
    if [ "$http_code" = "200" ]; then
        log_success "Health endpoint is responding"
        log_info "Response: $body"
        return 0
    else
        log_error "Health endpoint failed (HTTP $http_code)" "$body"
        return 1
    fi
}

test_models_list() {
    print_subheader "Test: List Models"
    
    log_check "Testing /v1/models endpoint"
    
    local response=$(api_get "/v1/models")
    parse_response "$response"
    
    local http_code=$(get_response_code)
    local body=$(get_response_body)
    
    cleanup_response_files
    
    if [ "$http_code" = "200" ]; then
        local model_count=$(echo "$body" | jq -r '.data | length' 2>/dev/null || echo "0")
        log_success "Models endpoint is accessible"
        log_info "Found $model_count models"
        return 0
    else
        log_error "Models endpoint failed (HTTP $http_code)" "$body"
        return 1
    fi
}

test_token_mint() {
    print_subheader "Test: Mint Token"
    
    log_check "Testing token minting"
    
    local token=$(mint_token "10m")
    
    if [ -n "$token" ]; then
        log_success "Successfully minted token"
        log_info "Token: ${token:0:20}..."
        
        # Clean up
        if revoke_tokens "$OC_TOKEN"; then
            log_info "Token revoked"
        fi
        return 0
    else
        log_error "Failed to mint token"
        return 1
    fi
}

# ==========================================
# Main Test Execution
# ==========================================

main() {
    print_header "🧪 Example Test Suite"
    
    # Check prerequisites
    if ! check_prerequisites; then
        exit 1
    fi
    
    echo ""
    
    # Run tests
    test_api_health || TEST_FAILED=1
    echo ""
    
    test_models_list || TEST_FAILED=1
    echo ""
    
    test_token_mint || TEST_FAILED=1
    echo ""
    
    # Print summary and exit
    if print_summary "Example Test Suite"; then
        exit 0
    else
        exit 1
    fi
}

# Run main
main "$@"


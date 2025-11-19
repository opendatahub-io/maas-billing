#!/bin/bash
# Container runtime: Podman (Docker compatible)
# Replace 'podman' with 'docker' if using Docker

# Named Token Smoke Test
# Tests the named token functionality including:
# - Creating a named token
# - Validating it works for API calls
# - Negative test for 401 with invalid token
# - Verifying Kubernetes secret was created with correct metadata

# Get the directory of this script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source common test framework
source "${SCRIPT_DIR}/common.sh"

set -euo pipefail

# Test-specific variables
TOKEN_NAME="smoke-test-$(date +%s)"
MAAS_TOKEN=""
EXPIRATION_TIMESTAMP=""
SECRET_NAMESPACE=""
SECRET_NAME=""

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --keep-secrets)
            export KEEP_SECRETS=true
            shift
            ;;
        --help|-h)
            echo "Named Token Smoke Test"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --keep-secrets    Don't delete the test secret after the test (default: delete)"
            echo "  -h, --help        Show this help message"
            echo ""
            echo "Environment Variables:"
            echo "  MAAS_API_BASE_URL  Base URL for MaaS API (auto-detected if not set)"
            echo "  OC_TOKEN           Authentication token (auto-detected if not set)"
            echo "  KEEP_SECRETS       Set to 'true' to keep secrets (default: false)"
            echo "  CLEANUP_ON_EXIT    Set to 'false' to skip token cleanup (default: true)"
            echo ""
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# ==========================================
# Named Token Specific Functions
# ==========================================

# Validate secret has all required fields for named tokens
validate_secret_metadata() {
    local namespace="$1"
    local secret_name="$2"
    
    # Get JSON blob
    local json_data=$(kubectl get secret "$secret_name" -n "$namespace" -o jsonpath='{.data.tokens\.json}' | base64 -d 2>/dev/null)
    
    if [ -z "$json_data" ]; then
        log_error "Secret does not contain tokens.json data"
        return 1
    fi

    # Validate root username
    local username=$(echo "$json_data" | jq -r '.username')
    if [ -z "$username" ] || [ "$username" == "null" ]; then
        log_error "JSON does not contain username"
        return 1
    fi
    log_info "  username: $username"

    # Find token by name
    local token_entry=$(echo "$json_data" | jq --arg NAME "$TOKEN_NAME" '.tokens[] | select(.name == $NAME)')
    
    if [ -z "$token_entry" ]; then
        log_error "Token '$TOKEN_NAME' not found in secret JSON"
        return 1
    fi

    local required_fields=("creationDate" "expirationDate" "name" "status" "id")
    local all_present=true
    
    for field in "${required_fields[@]}"; do
        local value=$(echo "$token_entry" | jq -r ".$field")
        if [ -z "$value" ] || [ "$value" == "null" ]; then
            log_error "Required field '$field' not found in token entry"
            all_present=false
        else
            log_info "  ${field}: ${value}"
        fi
    done
    
    # Security check: ensure actual token is NOT stored
    if echo "$token_entry" | jq -e '.token' > /dev/null; then
        log_error "SECURITY ISSUE: Secret contains actual token value!" \
            "Token should NOT be stored in metadata secrets"
        all_present=false
    fi
    
    if [ "$all_present" = true ]; then
        return 0
    else
        return 1
    fi
}

# ==========================================
# Test Functions
# ==========================================

# Test 1: Create a named token
test_create_named_token() {
    print_subheader "Test 1: Creating named token"
    
    log_check "Creating named token '$TOKEN_NAME'"
    
    local response=$(api_post "/v1/tokens" "{\"expiration\": \"1h\", \"name\": \"${TOKEN_NAME}\"}")
    parse_response "$response"
    
    local http_code=$(get_response_code)
    local body=$(get_response_body)
    
    cleanup_response_files
    
    if [ "$http_code" != "201" ]; then
        log_error "Failed to create named token (HTTP $http_code)" "$body"
        return 1
    fi
    
    MAAS_TOKEN=$(echo "$body" | jq -r '.token')
    EXPIRATION_TIMESTAMP=$(echo "$body" | jq -r '.expiresAt')
    
    if [ -z "$MAAS_TOKEN" ] || [ "$MAAS_TOKEN" = "null" ]; then
        log_error "Token not found in response" "$body"
        return 1
    fi
    
    log_success "Named token created successfully"
    log_info "Token: ${MAAS_TOKEN:0:20}..."
    log_info "Expires at: $EXPIRATION_TIMESTAMP ($(date -d @$EXPIRATION_TIMESTAMP 2>/dev/null || date -r $EXPIRATION_TIMESTAMP 2>/dev/null || echo 'N/A'))"
    
    return 0
}

# Test 2: Validate the token works
test_token_works() {
    print_subheader "Test 2: Validating token works"
    
    log_check "Testing token for API access"
    
    if [ -z "$MAAS_TOKEN" ]; then
        log_error "No token available for testing"
        return 1
    fi
    
    local response=$(api_get "/v1/models" "$MAAS_TOKEN")
    parse_response "$response"
    
    local http_code=$(get_response_code)
    local body=$(get_response_body)
    
    cleanup_response_files
    
    if [ "$http_code" != "200" ]; then
        log_error "Token validation failed (HTTP $http_code)" "$body"
        return 1
    fi
    
    log_success "Token works correctly for authenticated requests"
    
    return 0
}

# Test 3: Negative test - Invalid token should return 401
test_invalid_token_401() {
    print_subheader "Test 3: Testing invalid token rejection"
    
    log_check "Testing invalid token returns 401"
    
    local invalid_token="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.invalid.token"
    
    local response=$(api_get "/v1/models" "$invalid_token")
    parse_response "$response"
    
    local http_code=$(get_response_code)
    
    cleanup_response_files
    
    if [ "$http_code" = "401" ]; then
        log_success "Invalid token correctly returned 401 Unauthorized"
        return 0
    else
        log_error "Expected 401 for invalid token, got HTTP $http_code"
        return 1
    fi
}

# Test 4: Verify Kubernetes secret exists with correct metadata
test_secret_metadata() {
    print_subheader "Test 4: Verifying Kubernetes secret metadata"
    
    log_check "Looking for token metadata secret"
    log_info "Token name: $TOKEN_NAME"
    
    local secret_location=$(find_secret_by_token_name "$TOKEN_NAME")
    
    if [ -z "$secret_location" ]; then
        log_error "Token metadata secret not found" \
            "Secret should have been created for named token: $TOKEN_NAME" \
            "Check maas-api logs: kubectl logs -n maas-api -l app=maas-api"
        return 1
    fi
    
    SECRET_NAMESPACE=$(echo "$secret_location" | cut -d: -f1)
    SECRET_NAME=$(echo "$secret_location" | cut -d: -f2)
    
    log_success "Found token metadata secret: $SECRET_NAME in namespace: $SECRET_NAMESPACE"
    
    # Validate secret contents
    log_check "Validating secret metadata fields"
    
    if validate_secret_metadata "$SECRET_NAMESPACE" "$SECRET_NAME"; then
        log_success "All required metadata fields present and valid"
        log_success "Confirmed: Actual token value is NOT stored in the secret (as expected)"
        return 0
    else
        log_error "Secret metadata validation failed"
        return 1
    fi
}

# Test 5: Verify expiredAt field is added when tokens are revoked
test_expiredat_on_revocation() {
    print_subheader "Test 5: Verifying expiredAt field on revocation"
    
    # Get JSON and find token
    local get_status_cmd="kubectl get secret $SECRET_NAME -n $SECRET_NAMESPACE -o jsonpath='{.data.tokens\.json}' | base64 -d | jq -r --arg NAME \"$TOKEN_NAME\" '.tokens[] | select(.name == \$NAME).status'"
    
    log_check "Checking current status (should be 'active')"
    local status=$(eval $get_status_cmd)
    
    if [ "$status" != "active" ]; then
        log_warning "Expected status 'active', got '$status'"
    else
        log_success "Secret status is 'active'"
    fi
    
    log_check "Revoking all tokens"
    if ! revoke_tokens "$OC_TOKEN"; then
        log_error "Failed to revoke tokens"
        return 1
    fi
    log_success "Tokens revoked"
    
    # Wait for secret to be updated
    log_info "Waiting for secret to be updated..."
    sleep 3
    
    log_check "Verifying status changed to 'expired'"
    status=$(eval $get_status_cmd)
    if [ "$status" != "expired" ]; then
        log_error "Expected status 'expired', got '$status'" \
            "Secret may not have been updated during revocation"
        return 1
    fi
    log_success "Secret status is now 'expired'"
    
    log_check "Verifying expiredAt field was added"
    local get_expired_at_cmd="kubectl get secret $SECRET_NAME -n $SECRET_NAMESPACE -o jsonpath='{.data.tokens\.json}' | base64 -d | jq -r --arg NAME \"$TOKEN_NAME\" '.tokens[] | select(.name == \$NAME).expiredAt'"
    local expired_at=$(eval $get_expired_at_cmd)
    
    if [ -z "$expired_at" ] || [ "$expired_at" == "null" ]; then
        log_error "expiredAt field is NOT set!" \
            "This should be set when tokens are revoked" \
            "Check maas-api logs for errors during token revocation"
        return 1
    fi
    
    log_success "expiredAt field found: $expired_at"
    
    # Validate timestamp format
    log_check "Validating timestamp format"
    if date -d "$expired_at" &>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%S" "$expired_at" &>/dev/null 2>&1; then
        log_success "expiredAt is valid RFC3339 timestamp"
    else
        log_error "expiredAt has invalid timestamp format: $expired_at"
        return 1
    fi
    
    return 0
}

# ==========================================
# Cleanup
# ==========================================

cleanup() {
    local cleanup_performed=false
    
    # Clean up token
    if [ "${CLEANUP_ON_EXIT}" = "true" ] && [ -n "${MAAS_TOKEN}" ]; then
        log_info "Cleaning up: Revoking test token..."
        if revoke_tokens "$OC_TOKEN"; then
            log_info "Token revoked successfully"
            cleanup_performed=true
        else
            log_warning "Token revocation failed (non-critical)"
        fi
    fi
    
    # Clean up secret (default: yes, unless --keep-secrets flag is used)
    if [ "${KEEP_SECRETS}" != "true" ] && [ -n "${SECRET_NAME}" ] && [ -n "${SECRET_NAMESPACE}" ]; then
        log_info "Cleaning up: Deleting test secret..."
        if delete_secret "$SECRET_NAMESPACE" "$SECRET_NAME"; then
            log_info "Secret deleted successfully"
            cleanup_performed=true
        else
            log_warning "Secret deletion failed (non-critical)"
        fi
    elif [ "${KEEP_SECRETS}" = "true" ] && [ -n "${SECRET_NAME}" ]; then
        log_info "Keeping secret $SECRET_NAME for inspection (--keep-secrets flag was set)"
    fi
    
    if [ "$cleanup_performed" = true ]; then
        log_info "Cleanup completed"
    fi
}

# ==========================================
# Main Test Execution
# ==========================================

main() {
    print_header "🧪 Named Token Smoke Test"
    
    # Check prerequisites
    if ! check_prerequisites; then
        exit 1
    fi
    
    echo ""
    
    # Run tests
    test_create_named_token || TEST_FAILED=1
    echo ""
    
    if [ $TEST_FAILED -eq 0 ]; then
        test_token_works || TEST_FAILED=1
        echo ""
    fi
    
    test_invalid_token_401 || TEST_FAILED=1
    echo ""
    
    if [ $TEST_FAILED -eq 0 ]; then
        test_secret_metadata || TEST_FAILED=1
        echo ""
    fi
    
    if [ $TEST_FAILED -eq 0 ]; then
        test_expiredat_on_revocation || TEST_FAILED=1
        echo ""
    fi
    
    # Cleanup
    cleanup
    
    # Print summary and exit
    if print_summary "Named Token Test"; then
        exit 0
    else
        exit 1
    fi
}

# Trap cleanup on exit
trap cleanup EXIT

# Run main
main "$@"

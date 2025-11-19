#!/bin/bash
# Container runtime: Podman (Docker compatible)
# Replace 'podman' with 'docker' if using Docker

# Common Test Framework for MaaS Smoke Tests
# Source this file in your test scripts: source ./common.sh

# Prevent double-sourcing
if [ -n "${MAAS_COMMON_LOADED:-}" ]; then
    return 0
fi
MAAS_COMMON_LOADED=1

# ==========================================
# Color Codes
# ==========================================
export RED='\033[0;31m'
export GREEN='\033[0;32m'
export YELLOW='\033[1;33m'
export BLUE='\033[0;34m'
export NC='\033[0m' # No Color

# ==========================================
# Configuration & Environment
# ==========================================

# Try to auto-detect base URL from OpenShift cluster if not set
auto_detect_base_url() {
    if command -v kubectl &>/dev/null; then
        local cluster_domain=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}' 2>/dev/null || echo "")
        if [ -n "$cluster_domain" ]; then
            echo "https://maas.${cluster_domain}/maas-api"
        fi
    fi
}

# Set defaults
export MAAS_API_BASE_URL="${MAAS_API_BASE_URL:-$(auto_detect_base_url)}"
export OC_TOKEN="${OC_TOKEN:-}"
export NAMESPACE_PREFIX="${NAMESPACE_PREFIX:-maas}"
export CLEANUP_ON_EXIT="${CLEANUP_ON_EXIT:-true}"
export KEEP_SECRETS="${KEEP_SECRETS:-false}"

# Test state
export TEST_FAILED=0
export PASSED=0
export FAILED=0
export WARNINGS=0

# ==========================================
# Logging Functions
# ==========================================

log_info() {
    echo -e "${BLUE}ℹ️  [INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}✅ [PASS]${NC} $1"
    ((PASSED++))
}

log_error() {
    echo -e "${RED}❌ [FAIL]${NC} $1"
    if [ -n "${2:-}" ]; then
        echo -e "${RED}   Reason: $2${NC}"
    fi
    if [ -n "${3:-}" ]; then
        echo -e "${YELLOW}   Suggestion: $3${NC}"
    fi
    ((FAILED++))
    TEST_FAILED=1
}

log_warning() {
    echo -e "${YELLOW}⚠️  [WARN]${NC} $1"
    if [ -n "${2:-}" ]; then
        echo -e "${YELLOW}   Note: $2${NC}"
    fi
    ((WARNINGS++))
}

log_check() {
    echo -e "${BLUE}🔍 Checking: $1${NC}"
}

print_header() {
    echo ""
    echo "========================================="
    echo "$1"
    echo "========================================="
    echo ""
}

print_subheader() {
    echo ""
    echo "--- $1 ---"
    echo ""
}

# ==========================================
# Prerequisite Checks
# ==========================================

check_command() {
    local cmd="$1"
    local install_hint="${2:-}"
    
    if ! command -v "$cmd" &> /dev/null; then
        log_error "$cmd is not installed or not in PATH" "" "$install_hint"
        return 1
    fi
    return 0
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local all_ok=true
    
    # Check required commands
    if ! check_command "curl" "Install curl: sudo dnf install curl"; then
        all_ok=false
    fi
    
    if ! check_command "jq" "Install jq: sudo dnf install jq"; then
        all_ok=false
    fi
    
    if ! check_command "kubectl" "Install kubectl: https://kubernetes.io/docs/tasks/tools/"; then
        all_ok=false
    fi
    
    # Check environment variables
    if [ -z "$MAAS_API_BASE_URL" ]; then
        log_error "MAAS_API_BASE_URL environment variable is not set" \
            "Cannot determine MaaS API endpoint" \
            "Set with: export MAAS_API_BASE_URL='https://maas.example.com/maas-api'"
        all_ok=false
    fi
    
    if [ -z "$OC_TOKEN" ]; then
        # Try to get token automatically
        if command -v oc &> /dev/null; then
            OC_TOKEN=$(oc whoami -t 2>/dev/null || echo "")
            export OC_TOKEN
            if [ -n "$OC_TOKEN" ]; then
                log_info "Automatically obtained OC token"
            fi
        fi
        
        if [ -z "$OC_TOKEN" ]; then
            log_error "OC_TOKEN environment variable is not set" \
                "Authentication token is required" \
                "Set with: export OC_TOKEN=\$(oc whoami -t)"
            all_ok=false
        fi
    fi
    
    if [ "$all_ok" = true ]; then
        log_success "All prerequisites met"
        log_info "Using MAAS_API_BASE_URL: $MAAS_API_BASE_URL"
        return 0
    else
        return 1
    fi
}

# ==========================================
# API Helper Functions
# ==========================================

# Make authenticated GET request
api_get() {
    local endpoint="$1"
    local token="${2:-$OC_TOKEN}"
    
    curl -sSk \
        -w "\n%{http_code}" \
        -H "Authorization: Bearer ${token}" \
        "${MAAS_API_BASE_URL}${endpoint}"
}

# Make authenticated POST request
api_post() {
    local endpoint="$1"
    local data="$2"
    local token="${3:-$OC_TOKEN}"
    
    curl -sSk \
        -w "\n%{http_code}" \
        -X POST \
        -H "Authorization: Bearer ${token}" \
        -H "Content-Type: application/json" \
        -d "$data" \
        "${MAAS_API_BASE_URL}${endpoint}"
}

# Make authenticated DELETE request
api_delete() {
    local endpoint="$1"
    local token="${2:-$OC_TOKEN}"
    
    curl -sSk \
        -w "\n%{http_code}" \
        -X DELETE \
        -H "Authorization: Bearer ${token}" \
        "${MAAS_API_BASE_URL}${endpoint}"
}

# Parse response: returns body and http_code separately
parse_response() {
    local response="$1"
    echo "$response" | head -n-1 > /tmp/maas_response_body.$$
    echo "$response" | tail -n1 > /tmp/maas_response_code.$$
}

get_response_body() {
    cat /tmp/maas_response_body.$$ 2>/dev/null || echo ""
}

get_response_code() {
    cat /tmp/maas_response_code.$$ 2>/dev/null || echo "000"
}

cleanup_response_files() {
    rm -f /tmp/maas_response_body.$$ /tmp/maas_response_code.$$ 2>/dev/null
}

# ==========================================
# Token Management
# ==========================================

# Create a MaaS token
mint_token() {
    local expiration="${1:-1h}"
    local name="${2:-}"
    
    local payload="{\"expiration\": \"${expiration}\""
    if [ -n "$name" ]; then
        payload="${payload}, \"name\": \"${name}\""
    fi
    payload="${payload}}"
    
    local response=$(api_post "/v1/tokens" "$payload")
    parse_response "$response"
    
    local http_code=$(get_response_code)
    local body=$(get_response_body)
    
    cleanup_response_files
    
    if [ "$http_code" = "201" ]; then
        echo "$body" | jq -r '.token'
        return 0
    else
        log_error "Failed to mint token (HTTP $http_code)" "$body"
        return 1
    fi
}

# Revoke all tokens for current user
revoke_tokens() {
    local token="${1:-$OC_TOKEN}"
    
    local response=$(api_delete "/v1/tokens" "$token")
    parse_response "$response"
    
    local http_code=$(get_response_code)
    cleanup_response_files
    
    if [ "$http_code" = "204" ] || [ "$http_code" = "200" ]; then
        return 0
    else
        return 1
    fi
}

# ==========================================
# Kubernetes Secret Helpers
# ==========================================

# Find tier namespaces
find_tier_namespaces() {
    local namespaces=$(kubectl get namespaces \
        -l "maas.opendatahub.io/tier-namespace=true" \
        -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
    
    if [ -z "$namespaces" ]; then
        # Fallback to default namespace pattern
        namespaces="${NAMESPACE_PREFIX}-tier-free ${NAMESPACE_PREFIX}-tier-premium ${NAMESPACE_PREFIX}-tier-enterprise"
    fi
    
    echo "$namespaces"
}

# Find a secret by token name inside the JSON blob
find_secret_by_token_name() {
    local token_name="$1"
    local namespaces=$(find_tier_namespaces)
    
    for ns in $namespaces; do
        # Get all token secrets in this namespace
        local secret_names=$(kubectl get secrets -n "$ns" \
            -l "maas.opendatahub.io/token-secret=true" \
            -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
            
        for secret in $secret_names; do
             # Decode JSON content
             local token_json=$(kubectl get secret "$secret" -n "$ns" -o jsonpath='{.data.tokens\.json}' | base64 -d 2>/dev/null)
             
             # Check if token exists
             if echo "$token_json" | jq -e --arg NAME "$token_name" '.tokens[] | select(.name == $NAME)' > /dev/null; then
                 echo "${ns}:${secret}"
                 return 0
             fi
        done
    done
    
    return 1
}

# Get secret field value (base64 decoded)
get_secret_field() {
    local namespace="$1"
    local secret_name="$2"
    local field="$3"
    
    kubectl get secret "$secret_name" -n "$namespace" \
        -o jsonpath="{.data.${field}}" 2>/dev/null | base64 -d 2>/dev/null
}

# Delete a secret
delete_secret() {
    local namespace="$1"
    local secret_name="$2"
    
    if kubectl delete secret "$secret_name" -n "$namespace" 2>/dev/null; then
        log_info "Deleted secret $secret_name from namespace $namespace"
        return 0
    else
        log_warning "Failed to delete secret $secret_name from namespace $namespace"
        return 1
    fi
}

# ==========================================
# Test Summary
# ==========================================

print_summary() {
    local test_name="${1:-Test Suite}"
    
    print_header "📊 ${test_name} - Summary"
    
    echo "Passed:   $PASSED"
    echo "Failed:   $FAILED"
    echo "Warnings: $WARNINGS"
    echo ""
    
    if [ $FAILED -eq 0 ]; then
        log_success "All tests passed!"
        return 0
    else
        log_error "Some tests failed"
        return 1
    fi
}

# ==========================================
# Cluster Detection
# ==========================================

# Check if running on OpenShift
is_openshift() {
    kubectl api-resources 2>/dev/null | grep -q "route.openshift.io"
}

# Get cluster domain
get_cluster_domain() {
    if is_openshift; then
        kubectl get ingresses.config.openshift.io cluster \
            -o jsonpath='{.spec.domain}' 2>/dev/null || echo ""
    else
        # For vanilla Kubernetes, you might need different logic
        echo ""
    fi
}

# ==========================================
# Initialization
# ==========================================

# This function is called automatically when sourced
_init_common() {
    # Set up cleanup trap for temporary files
    trap 'cleanup_response_files' EXIT
    
    # If MAAS_API_BASE_URL is empty, try auto-detection
    if [ -z "$MAAS_API_BASE_URL" ]; then
        MAAS_API_BASE_URL=$(auto_detect_base_url)
        export MAAS_API_BASE_URL
    fi
}

_init_common

# Export all functions
export -f log_info log_success log_error log_warning log_check
export -f print_header print_subheader
export -f check_command check_prerequisites
export -f api_get api_post api_delete
export -f parse_response get_response_body get_response_code cleanup_response_files
export -f mint_token revoke_tokens
export -f find_tier_namespaces find_secret_by_token_name
export -f get_secret_field delete_secret
export -f print_summary
export -f is_openshift get_cluster_domain auto_detect_base_url


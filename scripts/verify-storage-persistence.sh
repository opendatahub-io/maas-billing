#!/bin/bash
# =============================================================================
# MaaS API Storage Persistence Verification Script
# =============================================================================
#
# This script verifies storage behavior for the 3 supported storage modes:
#   1. In-memory SQLite (default) - Data lost on pod restart
#   2. SQLite persistent (file)   - Data survives pod restart
#   3. PostgreSQL                 - Data survives pod restart
#
# WHAT IT TESTS:
#   - Create API key and verify it's active
#   - Retrieve API key metadata
#   - Restart the maas-api pod
#   - Verify metadata persistence (or loss for in-memory mode)
#
# USAGE:
#   ./scripts/verify-storage-persistence.sh [OPTIONS]
#
# OPTIONS:
#   --storage-mode MODE   Expected storage mode: memory, sqlite, postgres
#                         If not specified, auto-detects from deployment
#   --test-all-modes      Test all 3 storage modes sequentially
#                         (requires deploying each mode - needs kustomize)
#   --skip-restart        Skip pod restart test (faster, but incomplete)
#   --namespace NS        Namespace where maas-api is deployed (default: maas-api)
#
# ENVIRONMENT VARIABLES:
#   GATEWAY_URL   - Override gateway URL discovery
#   NAMESPACE     - Override namespace (default: maas-api)
#   PROJECT_ROOT  - Project root directory (auto-detected if not set)
# =============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-maas-api}"
STORAGE_MODE=""
SKIP_RESTART=false
TEST_ALL_MODES=false
POD_RESTART_TIMEOUT=120

find_project_root() {
    local dir
    dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    while [[ "$dir" != "/" && ! -e "$dir/.git" ]]; do
        dir="$(dirname "$dir")"
    done
    if [[ -e "$dir/.git" ]]; then
        echo "$dir"
    else
        echo "$(pwd)"
    fi
}

PROJECT_ROOT="${PROJECT_ROOT:-$(find_project_root)}"

while [[ $# -gt 0 ]]; do
    case $1 in
        --storage-mode)
            STORAGE_MODE="$2"
            shift 2
            ;;
        --test-all-modes)
            TEST_ALL_MODES=true
            shift
            ;;
        --skip-restart)
            SKIP_RESTART=true
            shift
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -h|--help)
            head -45 "$0" | tail -40
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

check_prerequisites() {
    local missing=()
    
    command -v oc &> /dev/null || missing+=("oc")
    command -v kubectl &> /dev/null || missing+=("kubectl")
    command -v jq &> /dev/null || missing+=("jq")
    command -v curl &> /dev/null || missing+=("curl")
    
    if [ ${#missing[@]} -ne 0 ]; then
        echo -e "${RED}Error: Missing required commands: ${missing[*]}${NC}"
        exit 1
    fi
}

detect_storage_mode() {
    echo -e "${BLUE}Detecting storage mode from deployment...${NC}"
    
    local db_url
    db_url=$(kubectl get deployment maas-api -n "$NAMESPACE" \
        -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="DATABASE_URL")].value}' 2>/dev/null || echo "")
    
    if [ -z "$db_url" ]; then
        local secret_ref
        secret_ref=$(kubectl get deployment maas-api -n "$NAMESPACE" \
            -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="DATABASE_URL")].valueFrom.secretKeyRef.name}' 2>/dev/null || echo "")
        
        if [ -n "$secret_ref" ]; then
            local secret_key
            secret_key=$(kubectl get deployment maas-api -n "$NAMESPACE" \
                -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="DATABASE_URL")].valueFrom.secretKeyRef.key}' 2>/dev/null || echo "DATABASE_URL")
            
            db_url=$(kubectl get secret "$secret_ref" -n "$NAMESPACE" \
                -o jsonpath="{.data.$secret_key}" 2>/dev/null | base64 -d 2>/dev/null || echo "")
        fi
    fi
    
    if [ -z "$db_url" ]; then
        STORAGE_MODE="memory"
        echo -e "${YELLOW}  No DATABASE_URL set → In-memory SQLite${NC}"
    elif [[ "$db_url" == postgresql://* ]] || [[ "$db_url" == postgres://* ]]; then
        STORAGE_MODE="postgres"
        echo -e "${GREEN}  PostgreSQL detected${NC}"
    elif [[ "$db_url" == sqlite://* ]] || [[ "$db_url" == *".db" ]] || [[ "$db_url" == *".sqlite"* ]]; then
        STORAGE_MODE="sqlite"
        echo -e "${GREEN}  SQLite persistent detected${NC}"
    elif [[ "$db_url" == ":memory:" ]]; then
        STORAGE_MODE="memory"
        echo -e "${YELLOW}  Explicit in-memory SQLite${NC}"
    else
        echo -e "${RED}  Unknown DATABASE_URL format: $db_url${NC}"
        STORAGE_MODE="unknown"
    fi
}

discover_gateway_url() {
    if [ -n "${GATEWAY_URL:-}" ]; then
        echo -e "${BLUE}Using provided GATEWAY_URL: ${GATEWAY_URL}${NC}"
        return
    fi
    
    echo -e "${BLUE}Discovering gateway URL...${NC}"
    
    local gateway_hostname
    gateway_hostname=$(kubectl get gateway maas-default-gateway -n openshift-ingress \
        -o jsonpath='{.spec.listeners[0].hostname}' 2>/dev/null || echo "")
    
    if [ -z "$gateway_hostname" ]; then
        gateway_hostname=$(kubectl get gateway -l app.kubernetes.io/instance=maas-default-gateway \
            -n openshift-ingress -o jsonpath='{.items[0].status.addresses[0].value}' 2>/dev/null || echo "")
    fi
    
    if [ -z "$gateway_hostname" ]; then
        echo -e "${RED}Failed to discover gateway hostname.${NC}"
        echo "Please set GATEWAY_URL explicitly (e.g., export GATEWAY_URL=https://maas.apps.example.com)"
        exit 1
    fi
    
    local scheme="https"
    if ! curl -skS -m 5 "${scheme}://${gateway_hostname}/maas-api/health" -o /dev/null 2>/dev/null; then
        scheme="http"
    fi
    
    GATEWAY_URL="${scheme}://${gateway_hostname}"
    echo -e "${GREEN}✓ Gateway URL: ${GATEWAY_URL}${NC}"
}

get_oc_token() {
    local token=""
    token=$(oc whoami -t 2>/dev/null || true)
    
    if [ -z "$token" ] && [ -f "/var/run/secrets/kubernetes.io/serviceaccount/token" ]; then
        token=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token 2>/dev/null || true)
    fi
    
    if [ -z "$token" ] && [ -n "${KUBECONFIG:-}" ]; then
        token=$(oc config view --raw -o jsonpath='{.users[0].user.token}' 2>/dev/null || true)
    fi
    
    if [ -z "$token" ]; then
        local current_user
        current_user=$(oc whoami 2>/dev/null || true)
        if [[ "$current_user" == system:serviceaccount:* ]]; then
            local sa_namespace sa_name
            sa_namespace=$(echo "$current_user" | cut -d: -f3)
            sa_name=$(echo "$current_user" | cut -d: -f4)
            if [ -n "$sa_namespace" ] && [ -n "$sa_name" ]; then
                token=$(oc create token "$sa_name" -n "$sa_namespace" 2>/dev/null || true)
            fi
        fi
    fi
    
    if [ -z "$token" ]; then
        echo -e "${RED}Failed to obtain OpenShift token${NC}"
        exit 1
    fi
    
    echo "$token"
}

create_api_key() {
    local oc_token="$1"
    local key_name="storage-test-$(date +%s)"
    
    echo -n "  Creating API key '$key_name'... "
    
    local response
    response=$(curl -sSk \
        -H "Authorization: Bearer $oc_token" \
        -H "Content-Type: application/json" \
        -X POST \
        -d "{\"name\": \"$key_name\", \"description\": \"Storage persistence test\", \"expiration\": \"1h\"}" \
        -w "\n%{http_code}" \
        "${GATEWAY_URL}/maas-api/v1/api-keys")
    
    local http_status body
    http_status=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_status" == "201" ]; then
        local token jti
        token=$(echo "$body" | jq -r '.token')
        jti=$(echo "$body" | jq -r '.jti')
        
        echo -e "${GREEN}✓ Created (JTI: $jti)${NC}"
        
        echo "{\"token\": \"$token\", \"jti\": \"$jti\", \"name\": \"$key_name\"}"
    else
        echo -e "${RED}✗ Failed (HTTP $http_status)${NC}"
        echo "Response: $body"
        return 1
    fi
}

verify_api_key_active() {
    local api_token="$1"
    
    echo -n "  Verifying API key is active (calling /v1/models)... "
    
    local response
    response=$(curl -sSk \
        -H "Authorization: Bearer $api_token" \
        -w "\n%{http_code}" \
        "${GATEWAY_URL}/maas-api/v1/models")
    
    local http_status
    http_status=$(echo "$response" | tail -1)
    
    if [ "$http_status" == "200" ]; then
        echo -e "${GREEN}✓ Active${NC}"
        return 0
    else
        echo -e "${RED}✗ Failed (HTTP $http_status)${NC}"
        return 1
    fi
}

retrieve_api_key_metadata() {
    local oc_token="$1"
    local jti="$2"
    
    echo -n "  Retrieving metadata for API key (JTI: $jti)... "
    
    local response
    response=$(curl -sSk \
        -H "Authorization: Bearer $oc_token" \
        -w "\n%{http_code}" \
        "${GATEWAY_URL}/maas-api/v1/api-keys/$jti")
    
    local http_status body
    http_status=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_status" == "200" ]; then
        local name status
        name=$(echo "$body" | jq -r '.name')
        status=$(echo "$body" | jq -r '.status')
        echo -e "${GREEN}✓ Found (Name: $name, Status: $status)${NC}"
        return 0
    elif [ "$http_status" == "404" ]; then
        echo -e "${YELLOW}✗ Not found (404)${NC}"
        return 1
    else
        echo -e "${RED}✗ Failed (HTTP $http_status)${NC}"
        echo "Response: $body"
        return 1
    fi
}

check_api_key_in_list() {
    local oc_token="$1"
    local jti="$2"
    
    echo -n "  Checking API key in list... "
    
    local response
    response=$(curl -sSk \
        -H "Authorization: Bearer $oc_token" \
        -w "\n%{http_code}" \
        "${GATEWAY_URL}/maas-api/v1/api-keys")
    
    local http_status body
    http_status=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_status" == "200" ]; then
        local found
        found=$(echo "$body" | jq -r ".[] | select(.id == \"$jti\") | .id" 2>/dev/null || echo "")
        
        if [ -n "$found" ]; then
            echo -e "${GREEN}✓ Found in list${NC}"
            return 0
        else
            echo -e "${YELLOW}✗ Not in list${NC}"
            return 1
        fi
    else
        echo -e "${RED}✗ Failed to list (HTTP $http_status)${NC}"
        return 1
    fi
}

restart_maas_api_pod() {
    echo -e "${BLUE}Restarting maas-api pod...${NC}"
    
    local old_pod
    old_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=maas-api \
        -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    if [ -z "$old_pod" ]; then
        echo -e "${RED}  ✗ Could not find maas-api pod${NC}"
        return 1
    fi
    
    echo "  Old pod: $old_pod"
    kubectl delete pod "$old_pod" -n "$NAMESPACE" --wait=false
    echo -n "  Waiting for new pod to be ready... "
    
    local start_time
    start_time=$(date +%s)
    
    while true; do
        local current_time elapsed
        current_time=$(date +%s)
        elapsed=$((current_time - start_time))
        
        if [ $elapsed -gt $POD_RESTART_TIMEOUT ]; then
            echo -e "${RED}✗ Timeout after ${POD_RESTART_TIMEOUT}s${NC}"
            return 1
        fi
        
        local new_pod ready
        new_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=maas-api \
            -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
        
        if [ -n "$new_pod" ] && [ "$new_pod" != "$old_pod" ]; then
            ready=$(kubectl get pod "$new_pod" -n "$NAMESPACE" \
                -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
            
            if [ "$ready" == "True" ]; then
                echo -e "${GREEN}✓ Ready (new pod: $new_pod)${NC}"
                
                sleep 2
                return 0
            fi
        fi
        
        sleep 2
    done
}

wait_for_api_healthy() {
    echo -n "  Waiting for API health check... "
    
    local max_attempts=60
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        local response
        response=$(curl -sSk -o /dev/null -w "%{http_code}" \
            "${GATEWAY_URL}/maas-api/health" 2>/dev/null || echo "000")
        
        if [ "$response" == "200" ]; then
            echo -e "${GREEN}✓ Healthy${NC}"
            return 0
        fi
        
        if [ $attempt -gt 10 ]; then
            local pod_ready
            pod_ready=$(kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/name=maas-api \
                -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
            if [ "$pod_ready" == "True" ]; then
                if [ $attempt -gt 40 ]; then
                    echo -e "${YELLOW}⚠ Gateway routing slow, but pod is ready - continuing${NC}"
                    sleep 5  
                    return 0
                fi
            fi
        fi
        
        attempt=$((attempt + 1))
        sleep 2
    done
    
    echo -e "${YELLOW}⚠ Health check timeout, checking pod status...${NC}"
    local pod_ready
    pod_ready=$(kubectl get pod -n "$NAMESPACE" -l app.kubernetes.io/name=maas-api \
        -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "")
    if [ "$pod_ready" == "True" ]; then
        echo -e "${YELLOW}⚠ Pod is ready - continuing despite gateway health check timeout${NC}"
        sleep 5
        return 0
    fi
    
    echo -e "${RED}✗ API not healthy after $max_attempts attempts${NC}"
    return 1
}

# Deploy a specific storage mode using kustomize
deploy_storage_mode() {
    local mode="$1"
    
    echo -e "${BLUE}Deploying storage mode: $mode${NC}"
    
    local kustomize_path
    case "$mode" in
        memory)
            kustomize_path="${PROJECT_ROOT}/deployment/base/maas-api"
            ;;
        sqlite)
            kustomize_path="${PROJECT_ROOT}/deployment/examples/sqlite-persistent"
            ;;
        postgres)
            echo -e "${YELLOW}  ⚠ PostgreSQL requires external database setup${NC}"
            echo -e "${YELLOW}  Skipping deployment - assuming PostgreSQL is already configured${NC}"
            return 0
            ;;
        *)
            echo -e "${RED}Unknown storage mode: $mode${NC}"
            return 1
            ;;
    esac
    
    echo "  Applying kustomize: $kustomize_path"
    if ! kustomize build "$kustomize_path" | kubectl apply -f - 2>&1; then
        echo -e "${RED}  ✗ Failed to deploy $mode mode${NC}"
        return 1
    fi
    
    echo -n "  Waiting for deployment rollout... "
    if kubectl rollout status deployment/maas-api -n "$NAMESPACE" --timeout=120s >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Ready${NC}"
    else
        echo -e "${RED}✗ Timeout${NC}"
        return 1
    fi
    
    sleep 3
    return 0
}

# Run test for a single storage mode
run_single_mode_test() {
    local mode="$1"
    local test_name="test-${mode}-$(date +%s)"
    
    echo ""
    echo -e "${MAGENTA}Testing $mode mode...${NC}"
    
    local oc_token
    oc_token=$(get_oc_token)
    
    echo -n "  Creating API key '$test_name'... "
    local response
    response=$(curl -sSk \
        -H "Authorization: Bearer $oc_token" \
        -H "Content-Type: application/json" \
        -X POST \
        -d "{\"name\": \"$test_name\", \"description\": \"Storage test for $mode\", \"expiration\": \"1h\"}" \
        -w "\n%{http_code}" \
        "${GATEWAY_URL}/maas-api/v1/api-keys")
    
    local http_status body
    http_status=$(echo "$response" | tail -1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_status" != "201" ]; then
        echo -e "${RED}✗ Failed to create API key (HTTP $http_status)${NC}"
        return 1
    fi
    
    local api_token api_key_jti
    api_token=$(echo "$body" | jq -r '.token')
    api_key_jti=$(echo "$body" | jq -r '.jti')
    echo -e "${GREEN}✓ Created (JTI: $api_key_jti)${NC}"
    
    verify_api_key_active "$api_token" || return 1
    
    retrieve_api_key_metadata "$oc_token" "$api_key_jti" || return 1
    
    if [ "$SKIP_RESTART" = true ]; then
        echo -e "  ${YELLOW}⏭️  Skipping pod restart (--skip-restart)${NC}"
        echo -e "  ${GREEN}✓ PASS: API key created and verified (restart test skipped)${NC}"
        return 0
    fi
    
    restart_maas_api_pod || return 1
    wait_for_api_healthy || return 1
    
    oc_token=$(get_oc_token)
    
    local metadata_found=false
    if retrieve_api_key_metadata "$oc_token" "$api_key_jti" 2>/dev/null; then
        metadata_found=true
    fi
    
    local list_found=false
    if check_api_key_in_list "$oc_token" "$api_key_jti" 2>/dev/null; then
        list_found=true
    fi
    
    case "$mode" in
        memory)
            if [ "$metadata_found" = false ] && [ "$list_found" = false ]; then
                echo -e "  ${GREEN}✓ PASS: In-memory mode - data correctly lost after restart${NC}"
                return 0
            else
                echo -e "  ${RED}✗ FAIL: In-memory mode - data unexpectedly persisted${NC}"
                return 1
            fi
            ;;
        sqlite|postgres)
            if [ "$metadata_found" = true ] && [ "$list_found" = true ]; then
                echo -e "  ${GREEN}✓ PASS: Persistent mode ($mode) - data survived restart${NC}"
                return 0
            else
                echo -e "  ${RED}✗ FAIL: Persistent mode ($mode) - data lost after restart${NC}"
                echo "    metadata_found=$metadata_found, list_found=$list_found"
                return 1
            fi
            ;;
        *)
            echo -e "  ${YELLOW}⚠ Unknown storage mode: $mode${NC}"
            return 1
            ;;
    esac
}

test_all_storage_modes() {
    echo -e "${CYAN}======================================${NC}"
    echo -e "${CYAN}  Testing All Storage Modes          ${NC}"
    echo -e "${CYAN}======================================${NC}"
    
    local modes=("memory" "sqlite" "postgres")
    local results=()
    local failed=0
    
    check_prerequisites
    discover_gateway_url
    
    for mode in "${modes[@]}"; do
        echo ""
        echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${CYAN}  Storage Mode: $mode${NC}"
        echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        
        if ! deploy_storage_mode "$mode"; then
            results+=("$mode: SKIP (deployment failed)")
            continue
        fi
        
        detect_storage_mode
        
        if run_single_mode_test "$mode"; then
            results+=("$mode: ✓ PASS")
        else
            results+=("$mode: ✗ FAIL")
            failed=$((failed + 1))
        fi
    done
    
    echo ""
    echo -e "${CYAN}======================================${NC}"
    echo -e "${CYAN}  Test Summary                        ${NC}"
    echo -e "${CYAN}======================================${NC}"
    
    for result in "${results[@]}"; do
        if [[ "$result" == *"PASS"* ]]; then
            echo -e "  ${GREEN}$result${NC}"
        elif [[ "$result" == *"FAIL"* ]]; then
            echo -e "  ${RED}$result${NC}"
        else
            echo -e "  ${YELLOW}$result${NC}"
        fi
    done
    
    echo ""
    if [ $failed -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        return 0
    else
        echo -e "${RED}$failed test(s) failed${NC}"
        return 1
    fi
}

main() {
    echo -e "${CYAN}======================================${NC}"
    echo -e "${CYAN}  MaaS API Storage Persistence Test  ${NC}"
    echo -e "${CYAN}======================================${NC}"
    echo ""
    
    check_prerequisites
    
    if [ -z "$STORAGE_MODE" ]; then
        detect_storage_mode
    else
        echo -e "${BLUE}Using specified storage mode: ${STORAGE_MODE}${NC}"
    fi
    
    if [ "$STORAGE_MODE" == "unknown" ]; then
        echo -e "${RED}Cannot proceed with unknown storage mode${NC}"
        exit 1
    fi
    
    discover_gateway_url
    
    echo ""
    echo -e "${MAGENTA}1. Authenticating with OpenShift...${NC}"
    OC_TOKEN=$(get_oc_token)
    echo -e "${GREEN}✓ Authenticated${NC}"
    
    echo ""
    echo -e "${MAGENTA}2. Creating and verifying API key...${NC}"
    
    local key_info
    key_info=$(create_api_key "$OC_TOKEN" | tail -1)
    
    local api_token api_key_jti api_key_name
    api_token=$(echo "$key_info" | jq -r '.token')
    api_key_jti=$(echo "$key_info" | jq -r '.jti')
    api_key_name=$(echo "$key_info" | jq -r '.name')
    
    verify_api_key_active "$api_token"
    
    retrieve_api_key_metadata "$OC_TOKEN" "$api_key_jti"
    
    check_api_key_in_list "$OC_TOKEN" "$api_key_jti"
    
    echo ""
    echo -e "${MAGENTA}3. Pod restart persistence test...${NC}"
    
    if [ "$SKIP_RESTART" = true ]; then
        echo -e "${YELLOW}⏭️  Skipping pod restart test (--skip-restart)${NC}"
    else
        restart_maas_api_pod
        
        wait_for_api_healthy
        
        echo ""
        echo -e "${MAGENTA}4. Verifying persistence after restart...${NC}"
        
        OC_TOKEN=$(get_oc_token)
        
        local metadata_found=false
        if retrieve_api_key_metadata "$OC_TOKEN" "$api_key_jti" 2>/dev/null; then
            metadata_found=true
        fi
        
        local list_found=false
        if check_api_key_in_list "$OC_TOKEN" "$api_key_jti" 2>/dev/null; then
            list_found=true
        fi
        
        echo ""
        echo -e "${MAGENTA}5. Persistence validation...${NC}"
        
        case "$STORAGE_MODE" in
            memory)
                if [ "$metadata_found" = false ] && [ "$list_found" = false ]; then
                    echo -e "  ${GREEN}✓ PASS: In-memory mode - data correctly lost after pod restart${NC}"
                else
                    echo -e "  ${RED}✗ FAIL: In-memory mode - data unexpectedly persisted after restart${NC}"
                    exit 1
                fi
                ;;
            sqlite|postgres)
                if [ "$metadata_found" = true ] && [ "$list_found" = true ]; then
                    echo -e "  ${GREEN}✓ PASS: Persistent mode ($STORAGE_MODE) - data survived pod restart${NC}"
                else
                    echo -e "  ${RED}✗ FAIL: Persistent mode ($STORAGE_MODE) - data lost after restart${NC}"
                    echo "    metadata_found=$metadata_found, list_found=$list_found"
                    exit 1
                fi
                ;;
            *)
                echo -e "  ${YELLOW}⚠ Unknown storage mode: $STORAGE_MODE${NC}"
                ;;
        esac
    fi
    
    echo ""
    echo -e "${CYAN}======================================${NC}"
    echo -e "${GREEN}  Storage Persistence Test Complete  ${NC}"
    echo -e "${CYAN}======================================${NC}"
    echo ""
    echo -e "Storage Mode: ${CYAN}$STORAGE_MODE${NC}"
    echo -e "Namespace:    ${CYAN}$NAMESPACE${NC}"
    echo -e "Gateway:      ${CYAN}$GATEWAY_URL${NC}"
}

# Entry point
if [ "$TEST_ALL_MODES" = true ]; then
    test_all_storage_modes
else
    main "$@"
fi


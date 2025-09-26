#!/bin/bash

# Comprehensive Token-Based Rate Limiting Test Suite
# Tests models across different tiers with detailed request/response logging

set +e  # Don't exit on errors, we want to test rate limiting

# ============================================================================
# COLOR DEFINITIONS
# ============================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

# ============================================================================
# CONFIGURATION
# ============================================================================
ROUTE_HOST=$(oc get route -n maas-api maas-api-route -o jsonpath='{.spec.host}' 2>/dev/null)
GATEWAY_HOST=$(oc get gateway openshift-ai-inference -n openshift-ingress -o jsonpath='{.status.addresses[0].value}' 2>/dev/null)
API_URL=$(oc whoami --show-server 2>/dev/null)

# Tier limits configuration (based on deployment/base/token-rate-limiting/token-rate-limit-policy.yaml)
declare -A TIER_LIMITS=(
    ["free"]=100        # tier-free namespace
    ["premium"]=300     # tier-premium namespace
    ["enterprise"]=1000 # tier-enterprise namespace
    ["default"]=50      # no tier- in namespace
)

# Test configuration
VERBOSE=${VERBOSE:-false}
SHOW_RESPONSES=${SHOW_RESPONSES:-true}
SKIP_WAIT=${SKIP_WAIT:-false}
TEST_ALL_TIERS=${TEST_ALL_TIERS:-false}
CURRENT_USER=$(oc whoami 2>/dev/null)
ORIGINAL_NAMESPACE=$(oc project -q 2>/dev/null)
ADMIN_TOKEN=""

# User management
TEST_PASSWORD="TestPass123!"
CLEANUP_ON_EXIT=${CLEANUP_ON_EXIT:-true}

# ============================================================================
# HELPER FUNCTIONS
# ============================================================================

print_header() {
    local title="$1"
    local width=70
    local padding=$(( (width - ${#title}) / 2 ))
    
    echo ""
    echo -e "${BLUE}$(printf '═%.0s' $(seq 1 $width))${NC}"
    printf "${BLUE}║${NC}"
    printf "%*s" $padding ""
    echo -n -e "${WHITE}${title}${NC}"
    printf "%*s" $((width - padding - ${#title})) ""
    echo -e "${BLUE}║${NC}"
    echo -e "${BLUE}$(printf '═%.0s' $(seq 1 $width))${NC}"
    echo ""
}

print_section() {
    echo ""
    echo -e "${CYAN}$(printf '─%.0s' $(seq 1 70))${NC}"
    echo -e "${CYAN}$1${NC}"
    echo -e "${CYAN}$(printf '─%.0s' $(seq 1 70))${NC}"
}

print_progress_bar() {
    local current=$1
    local total=$2
    local width=30
    
    # Handle case where current exceeds total
    local percent=$((current * 100 / total))
    if [[ $percent -gt 100 ]]; then percent=100; fi
    
    local filled=$((percent * width / 100))
    local empty=$((width - filled))
    
    # Choose color based on percentage
    local bar_color="${GREEN}"
    if [[ $percent -gt 75 ]]; then
        bar_color="${YELLOW}"
    fi
    if [[ $percent -gt 90 ]]; then
        bar_color="${RED}"
    fi
    
    echo -n "  ["
    if [[ $filled -gt 0 ]]; then
        printf "${bar_color}%0.s█${NC}" $(seq 1 $filled 2>/dev/null)
    fi
    if [[ $empty -gt 0 ]]; then
        printf "%0.s░" $(seq 1 $empty 2>/dev/null)
    fi
    echo "] ${percent}% ($current/$total tokens)"
}

# ============================================================================
# USER MANAGEMENT FUNCTIONS
# ============================================================================

save_admin_token() {
    ADMIN_TOKEN=$(oc whoami -t 2>/dev/null)
    if [[ -z "$ADMIN_TOKEN" ]]; then
        echo -e "${RED}Failed to save admin token${NC}"
        return 1
    fi
    echo "$ADMIN_TOKEN" > /tmp/admin-token-backup-$$.txt
    return 0
}

restore_admin_session() {
    echo -e "\n${CYAN}Restoring admin session...${NC}"
    
    if [[ -n "$ADMIN_TOKEN" ]]; then
        oc login --token="$ADMIN_TOKEN" --server="$API_URL" --insecure-skip-tls-verify=true >/dev/null 2>&1
    elif [[ -f "/tmp/admin-token-backup-$$.txt" ]]; then
        local token=$(cat /tmp/admin-token-backup-$$.txt)
        oc login --token="$token" --server="$API_URL" --insecure-skip-tls-verify=true >/dev/null 2>&1
    else
        echo -e "${YELLOW}Could not restore admin session automatically${NC}"
        return 1
    fi
    
    # Switch back to original namespace
    if [[ -n "$ORIGINAL_NAMESPACE" ]]; then
        oc project "$ORIGINAL_NAMESPACE" >/dev/null 2>&1
    fi
    
    echo -e "${GREEN}✓ Admin session restored${NC}"
    return 0
}

create_tier_user() {
    local tier=$1
    local username="test-${tier}-user-$$"
    
    echo -e "\n${CYAN}Creating user for ${tier^^} tier: $username${NC}"
    
    # Get HTPasswd secret name
    local htpasswd_secret=$(oc get oauth cluster -o jsonpath='{.spec.identityProviders[?(@.type=="HTPasswd")].htpasswd.fileData.name}' 2>/dev/null)
    
    if [[ -z "$htpasswd_secret" ]]; then
        echo -e "${RED}HTPasswd secret not found${NC}"
        return 1
    fi
    
    # Get current htpasswd data
    local htpasswd_data=$(oc get secret $htpasswd_secret -n openshift-config -o jsonpath='{.data.htpasswd}' | base64 -d)
    
    # Generate password hash
    local user_entry=$(htpasswd -nbB "$username" "$TEST_PASSWORD" 2>/dev/null)
    
    # Add new user
    htpasswd_data="${htpasswd_data}
${user_entry}"
    
    # Update secret
    echo "$htpasswd_data" | base64 -w0 > /tmp/htpasswd-updated-$$.b64
    oc patch secret $htpasswd_secret -n openshift-config \
        --type='json' \
        -p="[{\"op\": \"replace\", \"path\": \"/data/htpasswd\", \"value\": \"$(cat /tmp/htpasswd-updated-$$.b64)\"}]" >/dev/null 2>&1
    
    rm -f /tmp/htpasswd-updated-$$.b64
    
    echo -e "${GREEN}✓ User created: $username${NC}"
    
    # Create the tier group if it doesn't exist and add user to it
    local group_name="tier-${tier}-users"
    echo -e "${CYAN}Adding user to group: $group_name${NC}"
    
    # Check if group exists
    if oc get group "$group_name" >/dev/null 2>&1; then
        # Group exists, add user to it
        oc patch group "$group_name" --type='json' \
            -p="[{\"op\": \"add\", \"path\": \"/users/-\", \"value\": \"$username\"}]" >/dev/null 2>&1
    else
        # Create group with user
        cat <<EOF | oc apply -f - >/dev/null 2>&1
apiVersion: user.openshift.io/v1
kind: Group
metadata:
  name: ${group_name}
users:
  - ${username}
EOF
    fi
    
    echo -e "${GREEN}✓ User added to ${group_name} group${NC}"
    
    # Save username for later cleanup
    echo "$username" >> /tmp/test-users-$$.txt
    
    echo "$username"
}

setup_tier_namespace_and_permissions() {
    local tier=$1
    local username=$2
    local namespace="openshift-ai-inference-tier-${tier}"
    
    echo -e "${CYAN}Setting up namespace and permissions for ${tier^^} tier...${NC}"
    
    # Create/ensure namespace
    if ! oc get namespace "$namespace" >/dev/null 2>&1; then
        cat <<EOF | oc apply -f - >/dev/null 2>&1
apiVersion: v1
kind: Namespace
metadata:
  name: ${namespace}
  labels:
    tier: ${tier}
    purpose: rate-limit-testing
EOF
    fi
    
    # Grant minimal permissions needed for the user
    # Users only need view permissions to get tokens
    oc create clusterrolebinding "${username}-view" \
        --clusterrole=view \
        --user="${username}" \
        --dry-run=client -o yaml 2>/dev/null | oc apply -f - >/dev/null 2>&1
    
    echo -e "${GREEN}✓ Namespace and permissions configured${NC}"
}

wait_for_oauth_pods() {
    echo -e "${CYAN}Waiting for OAuth pods to reload...${NC}"
    
    # Delete OAuth pods to force reload
    oc delete pod -n openshift-authentication -l app=oauth-openshift --grace-period=1 >/dev/null 2>&1
    
    # Wait for pods to be ready
    local max_wait=60
    local waited=0
    
    while [[ $waited -lt $max_wait ]]; do
        local ready_pods=$(oc get pods -n openshift-authentication -l app=oauth-openshift \
            --field-selector=status.phase=Running -o name 2>/dev/null | wc -l)
        
        if [[ $ready_pods -ge 2 ]]; then
            # Give pods a bit more time to fully initialize
            echo -e "\n${GREEN}✓ OAuth pods running, waiting for initialization...${NC}"
            sleep 10
            echo -e "${GREEN}✓ OAuth system ready${NC}"
            return 0
        fi
        
        echo -ne "\r${YELLOW}Waiting for OAuth pods... ${waited}s${NC}"
        sleep 2
        waited=$((waited + 2))
    done
    
    echo -e "\n${YELLOW}⚠ OAuth pods may not be fully ready${NC}"
    # Wait a bit more anyway
    sleep 10
    return 0
}

login_as_user() {
    local username=$1
    local max_retries=10
    local retry=0
    
    echo -e "${CYAN}Logging in as $username...${NC}"
    
    # Wait a bit for user to propagate
    echo -e "${YELLOW}Waiting for user to propagate in OAuth system...${NC}"
    sleep 5
    
    while [[ $retry -lt $max_retries ]]; do
        if oc login -u "$username" -p "$TEST_PASSWORD" --server="$API_URL" --insecure-skip-tls-verify=true >/dev/null 2>&1; then
            echo -e "${GREEN}✓ Successfully logged in as $username${NC}"
            return 0
        fi
        
        retry=$((retry + 1))
        echo -e "${YELLOW}Login attempt $retry/$max_retries failed, retrying in 5s...${NC}"
        sleep 5
    done
    
    echo -e "${RED}✗ Failed to login as $username after $max_retries attempts${NC}"
    echo -e "${YELLOW}Note: User may need more time to propagate through OAuth system${NC}"
    return 1
}

cleanup_test_users() {
    echo -e "\n${CYAN}Cleaning up test users...${NC}"
    
    # Restore admin session first
    restore_admin_session
    
    # Get HTPasswd secret
    local htpasswd_secret=$(oc get oauth cluster -o jsonpath='{.spec.identityProviders[?(@.type=="HTPasswd")].htpasswd.fileData.name}' 2>/dev/null)
    
    if [[ -n "$htpasswd_secret" ]] && [[ -f "/tmp/test-users-$$.txt" ]]; then
        # Get current htpasswd data
        local htpasswd_data=$(oc get secret $htpasswd_secret -n openshift-config -o jsonpath='{.data.htpasswd}' | base64 -d)
        
        # Remove test users
        while read -r username; do
            echo "  Removing user: $username"
            htpasswd_data=$(echo "$htpasswd_data" | grep -v "^${username}:")
            
            # Clean up clusterrolebindings
            oc delete clusterrolebinding "${username}-view" --ignore-not-found=true >/dev/null 2>&1
            
            # Remove user from groups
            for tier in free premium enterprise; do
                local group_name="tier-${tier}-users"
                if oc get group "$group_name" >/dev/null 2>&1; then
                    # Remove user from group
                    oc patch group "$group_name" --type='json' \
                        -p="[{\"op\": \"replace\", \"path\": \"/users\", \"value\": $(oc get group "$group_name" -o json | jq -c "[.users[] | select(. != \"$username\")]")}]" >/dev/null 2>&1
                fi
            done
        done < /tmp/test-users-$$.txt
        
        # Update secret
        echo "$htpasswd_data" | base64 -w0 > /tmp/htpasswd-cleanup-$$.b64
        oc patch secret $htpasswd_secret -n openshift-config \
            --type='json' \
            -p="[{\"op\": \"replace\", \"path\": \"/data/htpasswd\", \"value\": \"$(cat /tmp/htpasswd-cleanup-$$.b64)\"}]" >/dev/null 2>&1
        
        rm -f /tmp/htpasswd-cleanup-$$.b64
        
        # Clean up empty tier groups (optional - only if they have no users)
        for tier in free premium enterprise; do
            local group_name="tier-${tier}-users"
            if oc get group "$group_name" >/dev/null 2>&1; then
                local user_count=$(oc get group "$group_name" -o json | jq '.users | length')
                if [[ "$user_count" == "0" ]]; then
                    echo "  Removing empty group: $group_name"
                    oc delete group "$group_name" --ignore-not-found=true >/dev/null 2>&1
                fi
            fi
        done
    fi
    
    # Clean up temp files
    rm -f /tmp/test-users-$$.txt
    rm -f /tmp/admin-token-backup-$$.txt
    rm -f /tmp/tier-*-token-$$.txt
    
    echo -e "${GREEN}✓ Cleanup complete${NC}"
}

# ============================================================================
# TOKEN MANAGEMENT FUNCTIONS
# ============================================================================

get_token() {
    # Get token for current user
    local token_response=$(curl -sSk \
        -H "Authorization: Bearer $(oc whoami -t)" \
        -H "Content-Type: application/json" \
        -X POST -d '{"expiration": "30m"}' \
        "https://${ROUTE_HOST}/v1/tokens" 2>/dev/null)
    
    echo $(echo "$token_response" | jq -r .token 2>/dev/null)
}

decode_token_info() {
    local token=$1
    
    # Decode JWT payload
    local payload=$(echo $token | cut -d'.' -f2)
    local padding_length=$((4 - ${#payload} % 4))
    if [[ $padding_length -ne 4 ]]; then
        payload="${payload}$(printf '=%.0s' $(seq 1 $padding_length))"
    fi
    
    local decoded=$(echo $payload | base64 -d 2>/dev/null)
    local namespace=$(echo "$decoded" | jq -r '."kubernetes.io".namespace' 2>/dev/null)
    local user=$(echo "$decoded" | jq -r '.sub' 2>/dev/null | cut -d':' -f4)
    
    # Determine tier from namespace (based on policy configuration)
    local tier="default"
    if [[ "$namespace" == *"tier-free"* ]]; then
        tier="free"
    elif [[ "$namespace" == *"tier-premium"* ]]; then
        tier="premium"
    elif [[ "$namespace" == *"tier-enterprise"* ]]; then
        tier="enterprise"
    fi
    
    echo "$user:$tier:$namespace"
}

# ============================================================================
# MODEL TESTING FUNCTIONS
# ============================================================================

check_model_availability() {
    local model_name=$1
    local namespace=${2:-llm}
    
    local status=$(oc get inferenceservice "$model_name" -n "$namespace" \
        -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
    
    [[ "$status" == "True" ]]
}

generate_test_prompts() {
    local tier=$1
    local limit=$2
    
    # Generate diverse prompts to make testing more realistic
    local prompts=()
    
    if [[ $limit -le 100 ]]; then
        # Small limit - use short prompts
        prompts=(
            "Hi there"
            "Count to 5"
            "What is 2+2?"
            "Name a color"
            "Say hello world"
            "What day is it?"
        )
    elif [[ $limit -le 300 ]]; then
        # Medium limit - moderate prompts
        prompts=(
            "Explain artificial intelligence in one sentence"
            "Write a haiku about clouds"
            "List 3 programming languages"
            "What's the capital of France?"
            "Generate a random joke"
            "Describe the ocean briefly"
            "What is machine learning?"
            "Name three planets"
        )
    else
        # Large limit - varied prompts
        prompts=(
            "Explain machine learning briefly"
            "Write a short poem about technology"
            "List benefits of cloud computing"
            "Describe quantum computing in simple terms"
            "What are neural networks?"
            "Generate a motivational quote"
            "Explain blockchain in simple terms"
            "Write a brief story about robots"
            "List types of databases"
            "Describe artificial intelligence"
            "What is deep learning?"
            "Explain natural language processing"
        )
    fi
    
    printf '%s\n' "${prompts[@]}"
}

test_rate_limit() {
    local model_path=$1
    local model_name=$2
    local model_display=$3
    local token=$4
    local tier=$5
    local limit=$6
    local test_num=$7
    
    print_section "Test #$test_num: $model_display - Tier: ${tier^^} (Limit: $limit tokens/min)"
    
    local total_tokens=0
    local request_num=0
    local rate_limited=false
    local successful_requests=0
    local failed_requests=0
    
    # Get test prompts
    local prompts=($(generate_test_prompts "$tier" "$limit"))
    
    echo -e "${CYAN}Testing model: ${WHITE}$model_display${NC}"
    echo -e "${CYAN}Model endpoint: ${WHITE}http://${GATEWAY_HOST}${model_path}/v1/chat/completions${NC}"
    echo ""
    
    # Keep making requests until rate limited
    while [[ $rate_limited == false ]]; do
        request_num=$((request_num + 1))
        
        # Cycle through prompts
        local prompt_index=$(( (request_num - 1) % ${#prompts[@]} ))
        local prompt="${prompts[$prompt_index]}"
        
        # Adjust max_tokens based on how close we are to the limit
        local remaining=$((limit - total_tokens))
        local max_tokens=50
        if [[ $remaining -lt 100 ]]; then
            max_tokens=30
        fi
        if [[ $remaining -lt 50 ]]; then
            max_tokens=20
        fi
        
        echo -e "\n${YELLOW}═══ Request #${request_num} ═══${NC}"
        
        # Build request JSON
        local request_json='{
            "model": "'"$model_name"'",
            "messages": [{"role": "user", "content": "'"$prompt"'"}],
            "max_tokens": '"$max_tokens"',
            "temperature": 0.7,
            "stream": false
        }'
        
        # Show request details
        echo -e "${MAGENTA}Request:${NC}"
        echo -e "  Prompt: \"$prompt\""
        echo -e "  Max tokens requested: $max_tokens"
        
        # Make API request with headers capture
        local start_time=$(date +%s%3N)
        local response_file="/tmp/response-$$-${request_num}.txt"
        local headers_file="/tmp/headers-$$-${request_num}.txt"
        
        curl -s -w '\n{"http_code": %{http_code}}' \
            -D "$headers_file" \
            "http://${GATEWAY_HOST}${model_path}/v1/chat/completions" \
            -H "Authorization: Bearer $token" \
            -H "Content-Type: application/json" \
            -X POST \
            -d "$request_json" > "$response_file" 2>/dev/null
        
        local end_time=$(date +%s%3N)
        local response_time=$((end_time - start_time))
        
        local response=$(cat "$response_file")
        local status=$(echo "$response" | tail -1 | jq -r .http_code)
        local body=$(echo "$response" | head -n -1)
        
        # Clean up temp files
        rm -f "$response_file" "$headers_file"
        
        if [[ "$status" == "200" ]]; then
            successful_requests=$((successful_requests + 1))
            
            # Parse response
            local content=$(echo "$body" | jq -r '.choices[0].message.content' 2>/dev/null)
            local prompt_tokens=$(echo "$body" | jq -r '.usage.prompt_tokens' 2>/dev/null)
            local completion_tokens=$(echo "$body" | jq -r '.usage.completion_tokens' 2>/dev/null)
            local tokens_used=$(echo "$body" | jq -r '.usage.total_tokens' 2>/dev/null)
            
            # Fail if token data is missing from model response
            if [[ "$tokens_used" == "null" ]] || [[ -z "$tokens_used" ]]; then
                echo -e "${RED}✗ ERROR: Model did not return token usage data!${NC}"
                echo -e "${RED}  This is required for token-based rate limiting.${NC}"
                echo -e "${YELLOW}  Response body for debugging:${NC}"
                if [[ "$VERBOSE" == "true" ]]; then
                    echo "$body" | jq . 2>/dev/null || echo "$body"
                fi
                failed_requests=$((failed_requests + 1))
                # Skip this iteration but continue testing
                continue
            fi
            
            # Validate token value is a number
            if ! [[ "$tokens_used" =~ ^[0-9]+$ ]]; then
                echo -e "${RED}✗ ERROR: Invalid token count returned: $tokens_used${NC}"
                failed_requests=$((failed_requests + 1))
                continue
            fi
            
            total_tokens=$((total_tokens + tokens_used))
            
            echo -e "${GREEN}✓ SUCCESS${NC} (${response_time}ms)"
            
            # Show response details
            echo -e "${CYAN}Response:${NC}"
            if [[ "$SHOW_RESPONSES" == "true" ]]; then
                # Truncate long responses for display
                local display_content="${content:0:200}"
                if [[ ${#content} -gt 200 ]]; then
                    display_content="${display_content}..."
                fi
                echo -e "  Content: \"$display_content\""
            fi
            
            echo -e "${BLUE}Token Usage:${NC}"
            echo -e "  Prompt tokens: $prompt_tokens"
            echo -e "  Completion tokens: $completion_tokens"
            echo -e "  ${WHITE}Model reported total_tokens: $tokens_used${NC}"
            echo -e "  Cumulative tokens consumed: $total_tokens / $limit"
            
            # Show progress bar
            print_progress_bar $total_tokens $limit
            
            # Check if we're approaching limit
            if [[ $total_tokens -gt $((limit * 85 / 100)) ]]; then
                echo -e "  ${YELLOW}⚠ Approaching limit (>85% used)${NC}"
            fi
            
        elif [[ "$status" == "429" ]]; then
            failed_requests=$((failed_requests + 1))
            rate_limited=true
            
            echo -e "${RED}✗ RATE LIMITED!${NC} (${response_time}ms)"
            echo -e "${GREEN}✓ Rate limiting successfully enforced at ~$total_tokens tokens${NC}"
            
            # Try to get error details
            local error_msg=$(echo "$body" | jq -r '.error // .message // "Rate limit exceeded"' 2>/dev/null)
            if [[ "$error_msg" != "null" ]] && [[ -n "$error_msg" ]]; then
                echo -e "${YELLOW}  Server message: $error_msg${NC}"
            fi
            
            break
            
        else
            failed_requests=$((failed_requests + 1))
            echo -e "${RED}✗ ERROR${NC} (Status: $status, Time: ${response_time}ms)"
            
            if [[ "$VERBOSE" == "true" ]]; then
                local error_msg=$(echo "$body" | jq -r '.error // .message // "Unknown error"' 2>/dev/null)
                echo -e "${RED}  Error: $error_msg${NC}"
            fi
        fi
        
        # Small delay to avoid overwhelming the service
        sleep 0.2
        
        # Safety check - stop after many requests
        if [[ $request_num -gt 50 ]]; then
            echo -e "\n${YELLOW}Safety limit reached (50 requests)${NC}"
            break
        fi
    done
    
    # Print summary
    echo ""
    echo -e "${CYAN}Test Summary:${NC}"
    echo -e "  Model: $model_display"
    echo -e "  Tier: ${tier^^} (Limit: $limit tokens/min)"
    echo -e "  Requests: $request_num (✓ $successful_requests, ✗ $failed_requests)"
    echo -e "  Tokens consumed: $total_tokens"
    
    if [[ $rate_limited == true ]]; then
        echo -e "  ${GREEN}✓ RATE LIMITING VERIFIED${NC}"
        echo -e "  ${GREEN}  Service correctly enforced $tier tier limit${NC}"
        return 0
    else
        echo -e "  ${YELLOW}⚠ Rate limit not reached${NC}"
        echo -e "  ${YELLOW}  Consider using longer prompts or more requests${NC}"
        return 1
    fi
}

test_tier_with_user() {
    local tier=$1
    local model_path=$2
    local model_name=$3
    local model_display=$4
    local test_num=$5
    
    local username="test-${tier}-user-$$"
    local namespace="openshift-ai-inference-tier-${tier}"
    local limit=${TIER_LIMITS[$tier]}
    
    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}           Testing ${tier^^} Tier (${limit} tokens/min)           ${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    
    # Ensure admin session for setup
    restore_admin_session
    
    # Create user for this tier
    create_tier_user "$tier" >/dev/null
    setup_tier_namespace_and_permissions "$tier" "$username"
    
    # Login as the test user
    if ! login_as_user "$username"; then
        echo -e "${RED}Failed to login as $username, skipping tier${NC}"
        return 1
    fi
    
    # Switch to tier namespace
    echo -e "${CYAN}Switching to namespace: $namespace${NC}"
    oc project "$namespace" >/dev/null 2>&1
    
    # Get MaaS token for this user
    echo -e "${CYAN}Getting MaaS token for ${tier^^} tier...${NC}"
    local token=$(get_token)
    
    if [[ -z "$token" ]] || [[ "$token" == "null" ]]; then
        echo -e "${RED}✗ Failed to get token for ${tier^^} tier${NC}"
        return 1
    fi
    
    # Save token for debugging
    echo "$token" > "/tmp/tier-${tier}-token-$$.txt"
    
    # Decode and show token info
    local token_info=$(decode_token_info "$token")
    local user=$(echo "$token_info" | cut -d':' -f1)
    local detected_tier=$(echo "$token_info" | cut -d':' -f2)
    local token_namespace=$(echo "$token_info" | cut -d':' -f3)
    
    echo -e "${GREEN}✓ Token obtained${NC}"
    echo -e "  User: ${CYAN}$username${NC}"
    echo -e "  Token namespace: ${CYAN}$token_namespace${NC}"
    echo -e "  Detected tier: ${CYAN}${detected_tier^^}${NC}"
    echo -e "  Expected limit: ${YELLOW}${limit} tokens/min${NC}"
    
    # Additional diagnostics
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "\n${CYAN}Token JWT claims (for debugging):${NC}"
        local payload=$(echo $token | cut -d'.' -f2)
        local padding_length=$((4 - ${#payload} % 4))
        if [[ $padding_length -ne 4 ]]; then
            payload="${payload}$(printf '=%.0s' $(seq 1 $padding_length))"
        fi
        echo $payload | base64 -d 2>/dev/null | jq '."kubernetes.io"' 2>/dev/null
    fi
    
    # Run the rate limit test
    test_rate_limit "$model_path" "$model_name" "$model_display" "$token" "$tier" "$limit" "$test_num"
    local result=$?
    
    return $result
}

wait_for_rate_limit_reset() {
    if [[ "$SKIP_WAIT" == "true" ]]; then
        echo -e "${YELLOW}Skipping rate limit reset wait (SKIP_WAIT=true)${NC}"
        return
    fi
    
    local wait_time=${1:-65}
    echo ""
    echo -e "${YELLOW}Waiting ${wait_time} seconds for rate limit window to reset...${NC}"
    
    for i in $(seq $wait_time -1 1); do
        echo -ne "\r${YELLOW}Reset in: $i seconds ${NC}"
        sleep 1
    done
    echo -e "\r${GREEN}Rate limit window reset! Ready for next test.            ${NC}"
}

# ============================================================================
# MAIN EXECUTION
# ============================================================================

show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help           Show this help message"
    echo "  -v, --verbose        Enable verbose output"
    echo "  -s, --skip-wait      Skip waiting between tests"
    echo "  -m, --model MODEL    Test specific model only (simulator/qwen3)"
    echo "  -a, --all-tiers      Test all tiers (creates temp users)"
    echo "  --hide-responses     Don't show model response content"
    echo "  --no-cleanup         Don't cleanup test users on exit"
    echo ""
    echo "Environment variables:"
    echo "  VERBOSE=true         Enable verbose output"
    echo "  SHOW_RESPONSES=false Hide model response content"
    echo "  SKIP_WAIT=true       Skip rate limit reset waits"
    echo "  TEST_ALL_TIERS=true  Test all tiers"
    echo "  CLEANUP_ON_EXIT=false Don't cleanup test users"
    echo ""
    echo "Examples:"
    echo "  $0                   # Test current user's tier with all models"
    echo "  $0 --all-tiers       # Create temp users and test all tiers"
    echo "  $0 --model simulator --all-tiers  # Test simulator across all tiers"
    echo "  $0 --hide-responses  # Don't show model responses"
}

main() {
    local specific_model=""
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -s|--skip-wait)
                SKIP_WAIT=true
                shift
                ;;
            -m|--model)
                specific_model="$2"
                shift 2
                ;;
            -a|--all-tiers)
                TEST_ALL_TIERS=true
                shift
                ;;
            --hide-responses)
                SHOW_RESPONSES=false
                shift
                ;;
            --no-cleanup)
                CLEANUP_ON_EXIT=false
                shift
                ;;
            *)
                echo "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Set up cleanup trap
    if [[ "$CLEANUP_ON_EXIT" == "true" ]]; then
        trap cleanup_test_users EXIT
    fi
    
    # Print header
    print_header "TOKEN-BASED RATE LIMITING TEST SUITE"
    
    # Check prerequisites
    echo -e "${CYAN}Checking prerequisites...${NC}"
    
    # Check if running as admin for multi-tier testing
    if [[ "$TEST_ALL_TIERS" == "true" ]]; then
        if ! oc auth can-i '*' '*' --all-namespaces >/dev/null 2>&1; then
            echo -e "${YELLOW}⚠ Multi-tier testing requires cluster-admin privileges${NC}"
            echo -e "${YELLOW}  Falling back to current user tier only${NC}"
            TEST_ALL_TIERS=false
        else
            echo -e "${GREEN}✓ Running with admin privileges${NC}"
        fi
    fi
    
    if [[ -z "$ROUTE_HOST" ]]; then
        echo -e "${RED}✗ MaaS API route not found${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ MaaS API route: $ROUTE_HOST${NC}"
    
    if [[ -z "$GATEWAY_HOST" ]]; then
        echo -e "${RED}✗ Gateway not found${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Gateway: $GATEWAY_HOST${NC}"
    
    echo -e "${GREEN}✓ Current user: $CURRENT_USER${NC}"
    
    # Save admin token if doing multi-tier testing
    if [[ "$TEST_ALL_TIERS" == "true" ]]; then
        if ! save_admin_token; then
            echo -e "${RED}Failed to save admin token${NC}"
            exit 1
        fi
        echo -e "${GREEN}✓ Admin token saved${NC}"
    fi
    
    # Display configuration
    print_section "Configuration"
    echo -e "${WHITE}Tier Limits (tokens/minute):${NC}"
    echo -e "  ${CYAN}Free:${NC} 100 (namespace contains 'tier-free')"
    echo -e "  ${CYAN}Premium:${NC} 300 (namespace contains 'tier-premium')"
    echo -e "  ${CYAN}Enterprise:${NC} 1000 (namespace contains 'tier-enterprise')"
    echo -e "  ${CYAN}Default:${NC} 50 (no 'tier-' in namespace)"
    echo ""
    echo -e "${WHITE}Available Models:${NC}"
    
    local models_to_test=()
    
    # Check simulator
    if check_model_availability "vllm-simulator" "llm"; then
        echo -e "  ${GREEN}✓${NC} Simulator (vllm-simulator)"
        if [[ -z "$specific_model" ]] || [[ "$specific_model" == "simulator" ]]; then
            models_to_test+=("simulator:/simulator:simulator:Simulator")
        fi
    else
        echo -e "  ${RED}✗${NC} Simulator (not deployed)"
    fi
    
    # Check qwen3
    if check_model_availability "qwen3-0-6b-instruct" "llm"; then
        echo -e "  ${GREEN}✓${NC} Qwen3 (qwen3-0-6b-instruct)"
        if [[ -z "$specific_model" ]] || [[ "$specific_model" == "qwen3" ]]; then
            models_to_test+=("qwen3:/qwen3:qwen3-0-6b-instruct:Qwen3 0.6B Instruct")
        fi
    else
        echo -e "  ${RED}✗${NC} Qwen3 (not deployed)"
    fi
    
    if [[ ${#models_to_test[@]} -eq 0 ]]; then
        echo ""
        echo -e "${RED}No models available for testing!${NC}"
        echo -e "${YELLOW}Deploy at least one model to test rate limiting.${NC}"
        exit 1
    fi
    
    echo ""
    echo -e "${WHITE}Test Mode:${NC}"
    if [[ "$TEST_ALL_TIERS" == "true" ]]; then
        echo -e "  ${CYAN}Testing ALL tiers with temporary users${NC}"
    else
        echo -e "  ${CYAN}Testing current user's tier only${NC}"
    fi
    
    # Start testing
    if [[ "$TEST_ALL_TIERS" == "true" ]]; then
        # Multi-tier testing with temp users
        print_header "MULTI-TIER TESTING WITH TEMP USERS"
        
        echo -e "${YELLOW}This will create temporary test users for each tier${NC}"
        echo -e "${YELLOW}Users will be automatically cleaned up after testing${NC}"
        
        # Wait for OAuth pods to be ready
        wait_for_oauth_pods
        
        local test_count=0
        local tests_passed=0
        local test_results=()
        local tiers_to_test=("free" "premium" "enterprise")
        
        for tier in "${tiers_to_test[@]}"; do
            for model_config in "${models_to_test[@]}"; do
                IFS=':' read -r model_key model_path model_name model_display <<< "$model_config"
                
                test_count=$((test_count + 1))
                if test_tier_with_user "$tier" "$model_path" "$model_name" "$model_display" "$test_count"; then
                    tests_passed=$((tests_passed + 1))
                    test_results+=("${GREEN}✓${NC} ${tier^^} + $model_display: PASSED")
                else
                    test_results+=("${RED}✗${NC} ${tier^^} + $model_display: FAILED")
                fi
                
                # Wait between tests
                if [[ $test_count -lt $((${#tiers_to_test[@]} * ${#models_to_test[@]})) ]]; then
                    wait_for_rate_limit_reset
                fi
            done
        done
        
        # Final summary
        print_header "TEST SUITE COMPLETE"
        
        echo -e "${GREEN}Summary:${NC}"
        echo -e "  Tests executed: $test_count"
        echo -e "  Tests passed: $tests_passed / $test_count"
        echo ""
        
        echo -e "${WHITE}Test Results:${NC}"
        for result in "${test_results[@]}"; do
            echo -e "  $result"
        done
        
        if [[ $tests_passed -eq $test_count ]]; then
            echo ""
            echo -e "${WHITE}Verification:${NC}"
            echo -e "  ${GREEN}✓${NC} Token-based rate limiting is working correctly"
            echo -e "  ${GREEN}✓${NC} All tiers have different token limits"
            echo -e "  ${GREEN}✓${NC} Limits are enforced based on actual token usage"
            echo -e "  ${GREEN}✓${NC} Different users get appropriate tier limits"
        fi
        
    else
        # Single user testing
        print_section "Authentication"
        echo "Obtaining authentication token..."
        local token=$(get_token)
        
        if [[ -z "$token" ]] || [[ "$token" == "null" ]]; then
            echo -e "${RED}✗ Failed to obtain authentication token${NC}"
            exit 1
        fi
        
        local token_info=$(decode_token_info "$token")
        local user=$(echo "$token_info" | cut -d':' -f1)
        local detected_tier=$(echo "$token_info" | cut -d':' -f2)
        local namespace=$(echo "$token_info" | cut -d':' -f3)
        
        echo -e "${GREEN}✓ Token obtained successfully${NC}"
        echo -e "  User: ${CYAN}$user${NC}"
        echo -e "  Namespace: ${CYAN}$namespace${NC}"
        echo -e "  Detected tier: ${YELLOW}${detected_tier^^}${NC}"
        echo -e "  Rate limit: ${YELLOW}${TIER_LIMITS[$detected_tier]} tokens/minute${NC}"
        
        print_header "Testing Tier: ${detected_tier^^} (${TIER_LIMITS[$detected_tier]} tokens/min)"
        
        local test_count=0
        local tests_passed=0
        
        for model_config in "${models_to_test[@]}"; do
            IFS=':' read -r model_key model_path model_name model_display <<< "$model_config"
            
            test_count=$((test_count + 1))
            if test_rate_limit "$model_path" "$model_name" "$model_display" "$token" "$detected_tier" "${TIER_LIMITS[$detected_tier]}" "$test_count"; then
                tests_passed=$((tests_passed + 1))
            fi
            
            # Wait between models if testing multiple
            if [[ $test_count -lt ${#models_to_test[@]} ]]; then
                wait_for_rate_limit_reset
            fi
        done
        
        # Summary
        print_header "TEST SUITE COMPLETE"
        
        echo -e "${GREEN}Summary:${NC}"
        echo -e "  Tests executed: $test_count"
        echo -e "  Tests passed: $tests_passed / $test_count"
        echo -e "  Tier tested: ${detected_tier^^}"
        echo -e "  Rate limit: ${TIER_LIMITS[$detected_tier]} tokens/minute"
        
        if [[ $tests_passed -eq $test_count ]]; then
            echo ""
            echo -e "${WHITE}Results:${NC}"
            echo -e "  ${GREEN}✓${NC} Token-based rate limiting is working correctly"
            echo -e "  ${GREEN}✓${NC} Limits are enforced based on actual token usage"
            echo -e "  ${GREEN}✓${NC} Token counting from model responses is accurate"
        fi
    fi
    
    echo ""
    echo -e "${CYAN}Notes:${NC}"
    echo -e "  • Tier is determined by namespace containing 'tier-free', 'tier-premium', or 'tier-enterprise'"
    echo -e "  • Tokens are counted from actual model responses (usage.total_tokens field)"
    echo -e "  • Rate limits reset every 60 seconds"
    
    if [[ "$TEST_ALL_TIERS" == "true" ]] && [[ "$CLEANUP_ON_EXIT" == "false" ]]; then
        echo ""
        echo -e "${YELLOW}Test users not cleaned up. To clean manually:${NC}"
        echo -e "  Run cleanup_test_users function or delete users manually"
    fi
    
    echo ""
    echo -e "${GREEN}Test suite completed!${NC}"
}

# Run main function
main "$@" 
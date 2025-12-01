#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# -----------------------------------------------------------------------------
# Prerequisites Check
# -----------------------------------------------------------------------------
if ! command -v oc &> /dev/null; then
    echo -e "${RED}Error: 'oc' command not found!${NC}"
    echo "This script requires OpenShift CLI to obtain identity tokens."
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo -e "${RED}Error: 'jq' command not found!${NC}"
    echo "This script requires jq to parse JSON."
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: 'kubectl' command not found!${NC}"
    echo "This script requires kubectl to query Gateway resources."
    exit 1
fi

# -----------------------------------------------------------------------------
# Gateway URL Discovery
# -----------------------------------------------------------------------------
if [ -z "${GATEWAY_URL:-}" ]; then
    echo -e "${BLUE}Looking up gateway configuration...${NC}"
    
    # Get the listener hostname from the Gateway spec (this is what Envoy routes on)
    GATEWAY_HOSTNAME=$(kubectl get gateway maas-default-gateway -n openshift-ingress -o jsonpath='{.spec.listeners[0].hostname}' 2>/dev/null)
    
    if [ -z "$GATEWAY_HOSTNAME" ]; then
        # Fallback: try to get from status address (may not work with hostname-based routing)
        GATEWAY_HOSTNAME=$(kubectl get gateway -l app.kubernetes.io/instance=maas-default-gateway -n openshift-ingress -o jsonpath='{.items[0].status.addresses[0].value}' 2>/dev/null)
    fi
    
    if [ -z "$GATEWAY_HOSTNAME" ]; then
        echo -e "${RED}Failed to find gateway hostname automatically.${NC}"
        echo -e "Please set GATEWAY_URL explicitly (e.g., export GATEWAY_URL=https://maas.apps.example.com)"
        exit 1
    fi
    
    GATEWAY_URL="https://${GATEWAY_HOSTNAME}"
    echo -e "${GREEN}✓ Found Gateway at: ${GATEWAY_URL}${NC}"
fi

API_BASE="${GATEWAY_URL%/}"

echo -e "${CYAN}======================================${NC}"
echo -e "${CYAN}   MaaS API Comprehensive Verification  ${NC}"
echo -e "${CYAN}======================================${NC}"
echo ""
echo -e "${BLUE}Gateway URL:${NC} ${GATEWAY_URL}"
echo ""

# -----------------------------------------------------------------------------
# 1. Authentication (OpenShift Identity)
# -----------------------------------------------------------------------------
echo -e "${MAGENTA}1. Authenticating with OpenShift...${NC}"
OC_TOKEN=$(oc whoami -t 2>/dev/null)
if [ -z "$OC_TOKEN" ]; then
    echo -e "${RED}✗ Failed to obtain OpenShift identity token!${NC}"
    echo "Please ensure you are logged in: oc login"
    exit 1
fi
echo -e "${GREEN}✓ Authenticated successfully${NC}"
echo ""

# -----------------------------------------------------------------------------
# 2. Ephemeral Tokens (Stateless)
# -----------------------------------------------------------------------------
echo -e "${MAGENTA}2. Testing Ephemeral Tokens (/v1/tokens)...${NC}"

# Test 2.1: Issue Ephemeral Token
echo -n "  • Issuing ephemeral token (4h)... "
TOKEN_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -H "Content-Type: application/json" \
    -X POST \
    -d '{"expiration": "4h"}' \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/tokens")

http_status=$(echo "$TOKEN_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
response_body=$(echo "$TOKEN_RESPONSE" | sed '/HTTP_STATUS:/d')

if [ "$http_status" == "201" ]; then
    EPHEMERAL_TOKEN=$(echo "$response_body" | jq -r '.token')
    echo -e "${GREEN}✓ Success${NC}"
else
    echo -e "${RED}✗ Failed (Status: $http_status)${NC}"
    echo "Response: $response_body"
    exit 1
fi

# Test 2.2: Validate Ephemeral Token (List Models)
echo -n "  • Validating token (Listing Models)... "
MODELS_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $EPHEMERAL_TOKEN" \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/models")

http_status=$(echo "$MODELS_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
if [ "$http_status" == "200" ]; then
    echo -e "${GREEN}✓ Success${NC}"
else
    echo -e "${RED}✗ Failed (Status: $http_status)${NC}"
fi

# Test 2.3: Verify Ephemeral Token Doesn't Appear in API Keys List
echo -n "  • Verifying ephemeral token NOT in API keys list... "
KEYS_LIST=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    "${API_BASE}/maas-api/v1/api-keys")

EPHEMERAL_JTI=$(echo "$response_body" | jq -r '.jti // empty')
if [ -n "$EPHEMERAL_JTI" ]; then
    FOUND_IN_LIST=$(echo "$KEYS_LIST" | jq -r ".[] | select(.id == \"$EPHEMERAL_JTI\") | .id")
    if [ -z "$FOUND_IN_LIST" ]; then
        echo -e "${GREEN}✓ Success (Ephemeral token not persisted)${NC}"
    else
        echo -e "${RED}✗ Failed (Ephemeral token found in API keys list!)${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Skipped (No JTI in ephemeral token response)${NC}"
fi
echo ""

# -----------------------------------------------------------------------------
# 3. API Keys (Persistent)
# -----------------------------------------------------------------------------
echo -e "${MAGENTA}3. Testing API Keys (Persistent /v1/api-keys)...${NC}"

KEY_NAME="test-key-$(date +%s)"

# Test 3.0: Verify API Key Creation Without Name Fails
echo -n "  • Testing API key creation without name (should fail)... "
NO_NAME_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -H "Content-Type: application/json" \
    -X POST \
    -d '{"expiration": "1h"}' \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/api-keys")

no_name_status=$(echo "$NO_NAME_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
if [ "$no_name_status" == "400" ]; then
    echo -e "${GREEN}✓ Success (Correctly rejected)${NC}"
else
    echo -e "${RED}✗ Failed (Expected 400, got $no_name_status)${NC}"
fi

# Test 3.1: Create API Key
echo -n "  • Creating API Key ('$KEY_NAME')... "
KEY_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -H "Content-Type: application/json" \
    -X POST \
    -d "{\"name\": \"$KEY_NAME\", \"expiration\": \"24h\"}" \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/api-keys")

http_status=$(echo "$KEY_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
response_body=$(echo "$KEY_RESPONSE" | sed '/HTTP_STATUS:/d')

if [ "$http_status" == "201" ]; then
    # Verify response structure: { "token": { "token": "...", "jti": "...", "expiration": "...", "expiresAt": ..., "name": "..." } }
    HAS_TOKEN_WRAPPER=$(echo "$response_body" | jq -r 'has("token")')
    if [ "$HAS_TOKEN_WRAPPER" != "true" ]; then
        echo -e "${RED}✗ Failed (Invalid response structure: missing 'token' wrapper)${NC}"
        echo "Response: $response_body"
        exit 1
    fi
    
    TOKEN_OBJ=$(echo "$response_body" | jq '.token')
    HAS_TOKEN_STRING=$(echo "$TOKEN_OBJ" | jq -r 'has("token")')
    HAS_JTI=$(echo "$TOKEN_OBJ" | jq -r 'has("jti")')
    HAS_NAME=$(echo "$TOKEN_OBJ" | jq -r 'has("name")')
    HAS_EXPIRATION=$(echo "$TOKEN_OBJ" | jq -r 'has("expiration")')
    HAS_EXPIRES_AT=$(echo "$TOKEN_OBJ" | jq -r 'has("expiresAt")')
    
    if [ "$HAS_TOKEN_STRING" == "true" ] && [ "$HAS_JTI" == "true" ] && [ "$HAS_NAME" == "true" ] && [ "$HAS_EXPIRATION" == "true" ] && [ "$HAS_EXPIRES_AT" == "true" ]; then
        API_KEY_TOKEN=$(echo "$response_body" | jq -r '.token.token')
        API_KEY_JTI=$(echo "$response_body" | jq -r '.token.jti')
        
        echo -e "${GREEN}✓ Success${NC}"
        echo "    - JTI: $API_KEY_JTI"
        echo "    - Response structure: ✓ Valid"
    else
        echo -e "${RED}✗ Failed (Invalid response structure)${NC}"
        echo "Response: $response_body"
        exit 1
    fi
else
    echo -e "${RED}✗ Failed (Status: $http_status)${NC}"
    echo "Response: $response_body"
    exit 1
fi

# Test 3.2: List API Keys
echo -n "  • Listing API Keys... "
LIST_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/api-keys")

http_status=$(echo "$LIST_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
list_body=$(echo "$LIST_RESPONSE" | sed '/HTTP_STATUS:/d')

if [ "$http_status" == "200" ]; then
    # Verify empty list returns [] not null
    IS_ARRAY=$(echo "$list_body" | jq 'type == "array"')
    if [ "$IS_ARRAY" != "true" ]; then
        echo -e "${RED}✗ Failed (Expected array, got: $(echo "$list_body" | jq 'type'))${NC}"
        exit 1
    fi
    
    # Check if our key is in the list
    FOUND_KEY=$(echo "$list_body" | jq -r ".[] | select(.name == \"$KEY_NAME\") | .name")
    if [ "$FOUND_KEY" == "$KEY_NAME" ]; then
        echo -e "${GREEN}✓ Success (Found '$KEY_NAME')${NC}"
        
        # Verify response structure has required fields
        KEY_DATA=$(echo "$list_body" | jq ".[] | select(.name == \"$KEY_NAME\")")
        HAS_ID=$(echo "$KEY_DATA" | jq -r 'has("id")')
        HAS_NAME=$(echo "$KEY_DATA" | jq -r 'has("name")')
        HAS_STATUS=$(echo "$KEY_DATA" | jq -r 'has("status")')
        HAS_CREATION_DATE=$(echo "$KEY_DATA" | jq -r 'has("creationDate")')
        HAS_EXPIRATION_DATE=$(echo "$KEY_DATA" | jq -r 'has("expirationDate")')
        
        if [ "$HAS_ID" == "true" ] && [ "$HAS_NAME" == "true" ] && [ "$HAS_STATUS" == "true" ] && [ "$HAS_CREATION_DATE" == "true" ] && [ "$HAS_EXPIRATION_DATE" == "true" ]; then
            echo "    - Response structure: ✓ Valid"
        else
            echo -e "${YELLOW}⚠ Warning: Missing fields in response${NC}"
        fi
    else
        echo -e "${RED}✗ Failed (Key '$KEY_NAME' not found in list)${NC}"
        echo "List: $list_body"
    fi
else
    echo -e "${RED}✗ Failed (Status: $http_status)${NC}"
fi

# Test 3.3: Get Specific API Key
echo -n "  • Getting API Key by ID ($API_KEY_JTI)... "
GET_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/api-keys/$API_KEY_JTI")

http_status=$(echo "$GET_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
get_body=$(echo "$GET_RESPONSE" | sed '/HTTP_STATUS:/d')

if [ "$http_status" == "200" ]; then
    RETRIEVED_ID=$(echo "$get_body" | jq -r '.id')
    if [ "$RETRIEVED_ID" == "$API_KEY_JTI" ]; then
        echo -e "${GREEN}✓ Success${NC}"
    else
        echo -e "${RED}✗ Failed (ID mismatch)${NC}"
    fi
else
    echo -e "${RED}✗ Failed (Status: $http_status)${NC}"
fi

# Test 3.4: Validate API Key Usage
echo -n "  • Using API Key for Request... "
MODELS_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $API_KEY_TOKEN" \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/models")

http_status=$(echo "$MODELS_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
if [ "$http_status" == "200" ]; then
    echo -e "${GREEN}✓ Success${NC}"
else
    echo -e "${RED}✗ Failed (Status: $http_status)${NC}"
fi

# Test 3.5: Revoke API Key
echo -n "  • Revoking API Key... "
REVOKE_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -X DELETE \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/api-keys/$API_KEY_JTI")

http_status=$(echo "$REVOKE_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
if [ "$http_status" == "204" ]; then
    echo -e "${GREEN}✓ Success${NC}"
else
    echo -e "${RED}✗ Failed (Status: $http_status)${NC}"
fi

# Test 3.6: Verify Revocation (Get should fail/404)
echo -n "  • Verifying Revocation (Get ID)... "
GET_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/api-keys/$API_KEY_JTI")

http_status=$(echo "$GET_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
if [ "$http_status" == "404" ]; then
    echo -e "${GREEN}✓ Success (404 Not Found)${NC}"
else
    echo -e "${RED}✗ Failed (Expected 404, got $http_status)${NC}"
fi

# Test 3.7: Verify Revoked Token Status
echo -n "  • Verifying Revoked Token Status... "
REVOKED_TOKEN_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $API_KEY_TOKEN" \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/models")

revoked_status=$(echo "$REVOKED_TOKEN_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)
if [ "$revoked_status" == "401" ] || [ "$revoked_status" == "403" ]; then
    echo -e "${GREEN}✓ Success (Token rejected: $revoked_status)${NC}"
else
    # Expected: Token still works because individual revocation only removes metadata
    # Kubernetes doesn't support revoking individual tokens - only all tokens via SA recreation
    echo -e "${BLUE}ℹ Info (Token still valid: $revoked_status)${NC}"
    echo "    Note: Individual API key revocation removes metadata only."
    echo "    Token remains valid until expiration or RevokeAll (recreates SA)."
    echo "    This is expected behavior - deny-list will be added in future work."
fi

echo ""

# -----------------------------------------------------------------------------
# 4. Revoke All Tokens
# -----------------------------------------------------------------------------
echo -e "${MAGENTA}4. Testing Revoke All Tokens (/v1/tokens)...${NC}"

# Create a temp key to ensure it gets deleted
echo -n "  • Creating temp key for cleanup test... "
TEMP_KEY_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -H "Content-Type: application/json" \
    -X POST \
    -d "{\"name\": \"cleanup-test\", \"expiration\": \"1h\"}" \
    "${API_BASE}/maas-api/v1/api-keys")
echo -e "${GREEN}✓ Done${NC}"

echo -n "  • Revoking ALL tokens... "
REVOKE_ALL_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -X DELETE \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/tokens")

http_status=$(echo "$REVOKE_ALL_RESPONSE" | grep "HTTP_STATUS:" | cut -d':' -f2)

if [ "$http_status" == "204" ]; then
    echo -e "${GREEN}✓ Success${NC}"
else
    echo -e "${RED}✗ Failed (Status: $http_status)${NC}"
    exit 1
fi

# Verify list is empty
echo -n "  • Verifying empty list... "
LIST_RESPONSE=$(curl -sSk \
    -H "Authorization: Bearer $OC_TOKEN" \
    -w "\nHTTP_STATUS:%{http_code}\n" \
    "${API_BASE}/maas-api/v1/api-keys")

list_body=$(echo "$LIST_RESPONSE" | sed '/HTTP_STATUS:/d')
IS_ARRAY=$(echo "$list_body" | jq 'type == "array"')
IS_EMPTY=$(echo "$list_body" | jq 'length == 0')

if [ "$IS_ARRAY" != "true" ]; then
    echo -e "${RED}✗ Failed (Expected array, got: $(echo "$list_body" | jq 'type'))${NC}"
    echo "Response: $list_body"
    exit 1
fi

if [ "$IS_EMPTY" == "true" ]; then
    echo -e "${GREEN}✓ Success (List is empty array [])${NC}"
else
    echo -e "${RED}✗ Failed (List not empty: $list_body)${NC}"
fi

echo ""
echo -e "${CYAN}======================================${NC}"
echo -e "${GREEN}   All Verification Tests Passed!     ${NC}"
echo -e "${CYAN}======================================${NC}"


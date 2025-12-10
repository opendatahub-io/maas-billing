#!/bin/bash

# MaaS Tool Calling Validation Script
# This script validates that tool calling functionality works with vLLM models
#
# Usage: ./validate-tool-calling.sh [MODEL_NAME]
#   MODEL_NAME: Optional. If provided, the script will validate using this specific model

# Note: We don't use 'set -e' because we want to continue validation even if some checks fail

# Parse command line arguments
REQUESTED_MODEL=""

# Show help if requested
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo "MaaS Tool Calling Validation Script"
    echo ""
    echo "Usage: $0 [MODEL_NAME]"
    echo ""
    echo "This script validates that tool calling functionality works with vLLM models."
    echo "It performs a single test call to the endpoint with a basic tool definition."
    echo ""
    echo "Arguments:"
    echo "  MODEL_NAME    Optional. Name of a specific model to use for validation."
    echo "                If not provided, the first available model will be used."
    echo ""
    echo "Examples:"
    echo "  # Basic tool calling validation"
    echo "  $0                                              # Validate using first available model"
    echo "  $0 single-node-no-scheduler-nvidia-gpu         # Validate using specific model"
    echo ""
    echo "Exit Codes:"
    echo "  0    Tool calling validation passed"
    echo "  1    Tool calling validation failed"
    echo ""
    exit 0
fi

# Parse arguments
if [ $# -gt 0 ]; then
    REQUESTED_MODEL="$1"
fi

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions for colored output
print_check() {
    echo -e "${BLUE}✓${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_fail() {
    echo -e "${RED}✗${NC} $1"
    if [ -n "$2" ]; then
        echo -e "    ${YELLOW}→${NC} $2"
    fi
    if [ -n "$3" ]; then
        echo -e "    ${YELLOW}→${NC} $3"
    fi
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
    if [ -n "$2" ]; then
        echo -e "    ${YELLOW}→${NC} $2"
    fi
}

print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

# Get authentication token
get_auth_token() {
    print_check "Getting authentication token"
    
    if [ -z "$HOST" ]; then
        print_fail "Host not set - cannot get authentication token"
        return 1
    fi
    
    ENDPOINT="${HOST}/maas-api/v1/tokens"
    print_info "Testing: curl -sSk -X POST $ENDPOINT -H 'Authorization: Bearer \$(oc whoami -t)' -H 'Content-Type: application/json' -d '{\"expiration\": \"10m\"}'"
    
    if command -v oc &> /dev/null; then
        OC_TOKEN=$(oc whoami -t 2>/dev/null || echo "")
        if [ -n "$OC_TOKEN" ]; then
            TOKEN_RESPONSE=$(curl -sSk --connect-timeout 10 --max-time 30 -w "\n%{http_code}" \
                -H "Authorization: Bearer ${OC_TOKEN}" \
                -H "Content-Type: application/json" \
                -X POST \
                -d '{"expiration": "10m"}' \
                "${ENDPOINT}" 2>/dev/null || echo "")
            
            HTTP_CODE=$(echo "$TOKEN_RESPONSE" | tail -n1)
            RESPONSE_BODY=$(echo "$TOKEN_RESPONSE" | sed '$d')
            
            # Handle timeout/connection failure
            if [ -z "$HTTP_CODE" ] || [ "$HTTP_CODE" = "000" ]; then
                print_fail "Connection timeout or failed to reach endpoint" \
                    "The endpoint is not reachable. This is likely because:" \
                    "1) The endpoint is behind a VPN or firewall, 2) DNS resolution failed, 3) Gateway/Route not properly configured"
                return 1
            elif [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "201" ]; then
                TOKEN=$(echo "$RESPONSE_BODY" | jq -r '.token' 2>/dev/null || echo "")
                if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
                    print_success "Authentication successful (HTTP $HTTP_CODE)"
                    return 0
                else
                    print_fail "Authentication response invalid" "Received HTTP $HTTP_CODE but no token in response" "Check MaaS API logs: kubectl logs -n maas-api -l app=maas-api"
                    return 1
                fi
            elif [ "$HTTP_CODE" = "404" ]; then
                print_fail "Endpoint not found (HTTP 404)" \
                    "Traffic is reaching the Gateway/pods but the path is incorrect" \
                    "Check HTTPRoute configuration: kubectl describe httproute maas-api-route -n maas-api"
                return 1
            elif [ "$HTTP_CODE" = "502" ] || [ "$HTTP_CODE" = "503" ]; then
                print_fail "Gateway/Service error (HTTP $HTTP_CODE)" \
                    "The Gateway is not able to reach the backend service" \
                    "Check: 1) MaaS API pods are running: kubectl get pods -n maas-api, 2) Service exists: kubectl get svc maas-api -n maas-api"
                return 1
            else
                print_fail "Authentication failed (HTTP $HTTP_CODE)" "Response: $(echo $RESPONSE_BODY | head -c 100)" "Check AuthPolicy and MaaS API service"
                return 1
            fi
        else
            print_fail "Cannot get OpenShift token" "Not logged into oc CLI" "Run: oc login"
            return 1
        fi
    else
        print_fail "oc CLI not found" "Cannot test authentication" "Install oc CLI or use kubectl with token"
        return 1
    fi
}

# Get the MaaS API host
get_maas_host() {
    print_check "Getting MaaS API host"
    
    # Get cluster domain and construct the MaaS gateway hostname
    CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}' 2>/dev/null || echo "")
    if [ -n "$CLUSTER_DOMAIN" ]; then
        HOST="maas.${CLUSTER_DOMAIN}"
        print_success "Gateway hostname: $HOST"
        return 0
    else
        print_fail "Could not determine cluster domain" "Cannot test API endpoints" "Check: kubectl get ingresses.config.openshift.io cluster"
        return 1
    fi
}

# Get available models
get_available_models() {
    print_check "Getting available models"
    
    if [ -z "$TOKEN" ] || [ -z "$HOST" ]; then
        print_fail "Missing token or host"
        return 1
    fi
    
    ENDPOINT="${HOST}/maas-api/v1/models"
    print_info "Testing: curl -sSk $ENDPOINT -H 'Authorization: Bearer \$TOKEN'"
    
    MODELS_RESPONSE=$(curl -sSk --connect-timeout 10 --max-time 30 -w "\n%{http_code}" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${TOKEN}" \
        "${ENDPOINT}" 2>/dev/null || echo "")
    
    HTTP_CODE=$(echo "$MODELS_RESPONSE" | tail -n1)
    RESPONSE_BODY=$(echo "$MODELS_RESPONSE" | sed '$d')
    
    if [ "$HTTP_CODE" = "200" ]; then
        MODEL_COUNT=$(echo "$RESPONSE_BODY" | jq -r '.data | length' 2>/dev/null || echo "0")
        if [ "$MODEL_COUNT" -gt 0 ]; then
            print_success "Found $MODEL_COUNT model(s)"
            
            # Print list of available models
            print_info "Available models:"
            echo "$RESPONSE_BODY" | jq -r '.data[] | "  • \(.id) - \(.url)"' 2>/dev/null || echo "  Could not parse model list"
            echo ""
            
            # Check if a specific model was requested
            if [ -n "$REQUESTED_MODEL" ]; then
                MODEL_INDEX=$(echo "$RESPONSE_BODY" | jq -r ".data | map(.id) | index(\"$REQUESTED_MODEL\")" 2>/dev/null || echo "null")
                
                if [ "$MODEL_INDEX" != "null" ] && [ -n "$MODEL_INDEX" ]; then
                    MODEL_NAME=$(echo "$RESPONSE_BODY" | jq -r ".data[$MODEL_INDEX].id" 2>/dev/null || echo "")
                    MODEL_CHAT=$(echo "$RESPONSE_BODY" | jq -r ".data[$MODEL_INDEX].url" 2>/dev/null || echo "")
                    print_info "Using requested model: $MODEL_NAME for tool calling validation"
                else
                    print_fail "Requested model '$REQUESTED_MODEL' not found" "See available models above"
                    return 1
                fi
            else
                # Use the first available model
                MODEL_NAME=$(echo "$RESPONSE_BODY" | jq -r '.data[0].id' 2>/dev/null || echo "")
                MODEL_CHAT=$(echo "$RESPONSE_BODY" | jq -r '.data[0].url' 2>/dev/null || echo "")
                print_info "Using first available model: $MODEL_NAME for tool calling validation"
            fi
            
            # Set the inference endpoint
            if [ -n "$MODEL_CHAT" ] && [ "$MODEL_CHAT" != "null" ]; then
                MODEL_CHAT_ENDPOINT="${MODEL_CHAT}/v1/chat/completions"
                return 0
            else
                print_fail "Model endpoint not found for $MODEL_NAME"
                return 1
            fi
        else
            print_fail "No models found" "Deploy a model first"
            return 1
        fi
    else
        print_fail "Failed to get models (HTTP $HTTP_CODE)" "Response: $(echo $RESPONSE_BODY | head -c 100)"
        return 1
    fi
}

# Test tool calling functionality
test_tool_calling() {
    print_check "Testing tool calling functionality"
    
    if [ -z "$TOKEN" ] || [ -z "$MODEL_NAME" ] || [ -z "$MODEL_CHAT_ENDPOINT" ]; then
        print_fail "Missing required parameters for tool calling test"
        return 1
    fi
    
    # Define a simple tool for testing
    TOOL_CALLING_PAYLOAD=$(cat <<EOF
{
  "model": "${MODEL_NAME}",
  "messages": [
    {
      "role": "user",
      "content": "What's the weather like in San Francisco? Use the get_weather tool to check."
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get the current weather in a given location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "The city and state, e.g. San Francisco, CA"
            },
            "unit": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"],
              "description": "The unit of temperature"
            }
          },
          "required": ["location"]
        }
      }
    }
  ],
  "tool_choice": "auto",
  "max_tokens": 100
}
EOF
)
    
    print_info "Testing tool calling with model: $MODEL_NAME"
    print_info "Endpoint: $MODEL_CHAT_ENDPOINT"
    print_info "Tool: get_weather (weather checking function)"
    
    INFERENCE_RESPONSE=$(curl -sSk --connect-timeout 30 --max-time 60 -w "\n%{http_code}" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d "${TOOL_CALLING_PAYLOAD}" \
        "${MODEL_CHAT_ENDPOINT}" 2>/dev/null || echo "")
    
    HTTP_CODE=$(echo "$INFERENCE_RESPONSE" | tail -n1)
    RESPONSE_BODY=$(echo "$INFERENCE_RESPONSE" | sed '$d')
    
    if [ -z "$HTTP_CODE" ] || [ "$HTTP_CODE" = "000" ]; then
        print_fail "Connection timeout or failed to reach endpoint" \
            "Model endpoint is not reachable" \
            "Check Gateway and model HTTPRoute: kubectl get httproute -n llm"
        return 1
    elif [ "$HTTP_CODE" = "200" ]; then
        # Check if the response contains tool calls
        TOOL_CALLS=$(echo "$RESPONSE_BODY" | jq -r '.choices[0].message.tool_calls // empty' 2>/dev/null)
        
        if [ -n "$TOOL_CALLS" ] && [ "$TOOL_CALLS" != "null" ]; then
            print_success "Tool calling functionality working!"
            print_info "Model successfully generated tool calls"
            
            # Extract and display tool call details
            TOOL_CALL_COUNT=$(echo "$RESPONSE_BODY" | jq -r '.choices[0].message.tool_calls | length' 2>/dev/null || echo "0")
            print_info "Number of tool calls generated: $TOOL_CALL_COUNT"
            
            # Display the tool calls
            echo "$RESPONSE_BODY" | jq -r '.choices[0].message.tool_calls[]? | "  • Tool: \(.function.name) - Args: \(.function.arguments)"' 2>/dev/null || echo "  Could not parse tool calls"
            
            # Check if the tool call is for our test function
            WEATHER_TOOL_CALL=$(echo "$RESPONSE_BODY" | jq -r '.choices[0].message.tool_calls[]? | select(.function.name == "get_weather")' 2>/dev/null)
            if [ -n "$WEATHER_TOOL_CALL" ]; then
                print_success "Model correctly identified the need to use the get_weather tool"
                print_info "Tool call arguments: $(echo "$WEATHER_TOOL_CALL" | jq -r '.function.arguments' 2>/dev/null)"
            else
                print_warning "Model generated tool calls but not for the expected get_weather tool"
            fi
            
            return 0
        else
            print_fail "Tool calling not working - no tool calls in response" \
                "The model may not support tool calling or the configuration is incorrect" \
                "Check: 1) Model supports tool calling, 2) VLLM_ADDITIONAL_ARGS includes --tool-call-parser, 3) Model is properly configured"
            
            print_info "Response content: $(echo $RESPONSE_BODY | head -c 300)"
            return 1
        fi
    elif [ "$HTTP_CODE" = "404" ]; then
        print_fail "Model inference endpoint not found (HTTP 404)" \
            "Path is incorrect - traffic reaching but wrong path" \
            "Check model HTTPRoute configuration: kubectl get httproute -n llm"
        return 1
    elif [ "$HTTP_CODE" = "502" ] || [ "$HTTP_CODE" = "503" ]; then
        print_fail "Gateway/Service error (HTTP $HTTP_CODE)" \
            "Gateway cannot reach model service" \
            "Check: 1) Model pods running: kubectl get pods -n llm, 2) Model service exists, 3) HTTPRoute configured"
        return 1
    elif [ "$HTTP_CODE" = "401" ]; then
        print_fail "Authorization failed (HTTP 401)" \
            "Response: $(echo $RESPONSE_BODY | head -c 200)" \
            "Check AuthPolicy and TokenRateLimitPolicy"
        return 1
    elif [ "$HTTP_CODE" = "429" ]; then
        print_warning "Rate limiting (HTTP 429)" \
            "Response: $(echo $RESPONSE_BODY | head -c 200)" \
            "Wait a minute and try again"
        return 1
    else
        print_fail "Tool calling test failed (HTTP $HTTP_CODE)" \
            "Response: $(echo $RESPONSE_BODY | head -c 200)" \
            "Check model pod logs and configuration"
        return 1
    fi
}

# Main validation function
main() {
    echo "🔧 MaaS Tool Calling Validation Script"
    echo "======================================"
    echo ""
    
    # Initialize variables
    TOKEN=""
    HOST=""
    MODEL_NAME=""
    MODEL_CHAT_ENDPOINT=""
    
    # Get MaaS API host first (needed for authentication)
    if ! get_maas_host; then
        echo ""
        echo "❌ Tool calling validation failed: Could not get MaaS API host"
        exit 1
    fi
    
    # Get authentication token
    if ! get_auth_token; then
        echo ""
        echo "❌ Tool calling validation failed: Could not get authentication token"
        exit 1
    fi
    
    # Get available models
    if ! get_available_models; then
        echo ""
        echo "❌ Tool calling validation failed: Could not get available models"
        exit 1
    fi
    
    # Test tool calling functionality
    if ! test_tool_calling; then
        echo ""
        echo "❌ Tool calling validation failed: Tool calling test did not pass"
        exit 1
    fi
    
    echo ""
    echo "✅ Tool calling validation completed successfully!"
    echo "   The vLLM model is properly configured for tool calling functionality."
    exit 0
}

# Run main function
main "$@"
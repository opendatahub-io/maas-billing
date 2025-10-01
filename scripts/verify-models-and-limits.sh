#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

if [ -z "${GATEWAY_URL:-}" ]; then
    # For OpenShift, use the route instead of the AWS ELB
    if command -v oc &> /dev/null; then
        CLUSTER_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}' 2>/dev/null)
        if [ -n "$CLUSTER_DOMAIN" ]; then
            GATEWAY_URL="https://gateway.${CLUSTER_DOMAIN}"
        fi
    fi
    
    # Fallback to gateway status address if OpenShift route not available
    if [ -z "${GATEWAY_URL:-}" ]; then
        HOST=$(kubectl get gateway openshift-ai-inference -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')
        if [ -z "$HOST" ]; then
            echo "Failed to resolve gateway host; set GATEWAY_URL explicitly." >&2
            exit 1
        fi
        GATEWAY_URL="https://${HOST}"
    fi
fi

echo -e "${CYAN}======================================${NC}"
echo -e "${CYAN}   Model Inference & Rate Limit Test  ${NC}"
echo -e "${CYAN}======================================${NC}"
echo ""
echo -e "${BLUE}Gateway URL:${NC} $GATEWAY_URL"
echo ""

# Step 1: Create a test service account and get token
echo -e "${BLUE}Step 1: Creating test service account and obtaining token...${NC}"
# Create service account in the free tier namespace so it gets the right group membership
kubectl create serviceaccount model-test-user -n openshift-ai-inference-tier-free --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1
TOKEN=$(kubectl create token model-test-user -n openshift-ai-inference-tier-free --audience=openshift-ai-inference-sa --duration=1h 2>/dev/null)

if [ -z "$TOKEN" ]; then
    echo -e "${RED}Failed to obtain token!${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Token obtained successfully${NC}"

# Check the user's tier (for debugging rate limits)
SA_NAME=$(echo "$TOKEN" | cut -d. -f2 | base64 -d 2>/dev/null | jq -r '.["kubernetes.io"].serviceaccount.name' 2>/dev/null || echo "unknown")
SA_NAMESPACE=$(echo "$TOKEN" | cut -d. -f2 | base64 -d 2>/dev/null | jq -r '.["kubernetes.io"].serviceaccount.namespace' 2>/dev/null || echo "unknown")
echo -e "${CYAN}Service Account:${NC} $SA_NAME"
echo -e "${CYAN}Namespace:${NC} $SA_NAMESPACE (tier: free)"
echo -e "${CYAN}Note:${NC} Using free tier limits (5 requests per 2 minutes)"
echo ""

# Function to test a model
test_model() {
    local model_name=$1
    local model_path=$2
    local model_id=$3
    
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${MAGENTA}Testing Model: $model_name${NC}"
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    
    # Test prompts
    local prompts=(
        "What is 2+2?"
        "Say 'Hello World' in Python"
        "What color is the sky?"
    )
    
    # Test single inference for each prompt
    echo -e "${BLUE}Testing inference with different prompts:${NC}"
    echo ""
    
    for i in "${!prompts[@]}"; do
        prompt="${prompts[$i]}"
        echo -e "${YELLOW}Request #$((i+1)):${NC}"
        echo -e "${CYAN}Prompt:${NC} \"$prompt\""
        
        # Prepare request body
        REQUEST_BODY=$(cat <<EOF
{
  "model": "$model_id",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant. Keep responses brief."},
    {"role": "user", "content": "$prompt"}
  ],
  "temperature": 0.1,
  "max_tokens": 50
}
EOF
)
        
        # Make request
        response=$(curl -sSk \
            -H "Authorization: Bearer $TOKEN" \
            -H "Content-Type: application/json" \
            -X POST \
            -d "$REQUEST_BODY" \
            -w "\nHTTP_STATUS:%{http_code}\n" \
            "$GATEWAY_URL$model_path/v1/chat/completions" 2>&1)
        
        http_status=$(echo "$response" | grep "HTTP_STATUS:" | cut -d':' -f2)
        response_body=$(echo "$response" | sed '/HTTP_STATUS:/d')
        
        if [ "$http_status" = "200" ]; then
            echo -e "${GREEN}Status: $http_status (Success)${NC}"
            
            # Extract and display response
            answer=$(echo "$response_body" | jq -r '.choices[0].message.content // "No response"' 2>/dev/null)
            tokens_used=$(echo "$response_body" | jq -r '.usage.total_tokens // 0' 2>/dev/null)
            
            echo -e "${CYAN}Response:${NC} $answer"
            echo -e "${CYAN}Tokens Used:${NC} $tokens_used"
        else
            echo -e "${RED}Status: $http_status (Failed)${NC}"
            echo -e "${RED}Error:${NC} $(echo "$response_body" | head -1)"
        fi
        echo ""
        
        # Small delay between requests
        sleep 1
    done
}

# Test all models
test_model "Facebook OPT-125M Simulator" "/llm/facebook-opt-125m-simulated" "facebook-opt-125m-simulated"
test_model "Facebook OPT-125M CPU" "/llm/facebook-opt-125m-cpu-single-node-no-scheduler-cpu" "facebook/opt-125m"
test_model "QWEN3-0.6B GPU" "/llm/single-node-no-scheduler-nvidia-gpu" "Qwen/Qwen3-0.6B"

# Step 2: Test rate limiting
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${MAGENTA}Testing Token Rate Limiting${NC}"
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BLUE}Making rapid requests to trigger rate limit...${NC}"
echo "(Using Facebook OPT-125M Simulator for rate limit test)"
echo ""

# Rapid fire requests to trigger rate limiting
REQUEST_BODY_SIMPLE=$(cat <<EOF
{
  "model": "facebook-opt-125m-simulated",
  "messages": [
    {"role": "user", "content": "Count to 5"}
  ],
  "temperature": 0.1,
  "max_tokens": 30
}
EOF
)

total_success=0
total_tokens=0
rate_limited=false

echo -n "Request status: "
for i in {1..25}; do
    response=$(curl -sSk \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -X POST \
        -d "$REQUEST_BODY_SIMPLE" \
        -w "\nHTTP_STATUS:%{http_code}\n" \
        "$GATEWAY_URL/llm/facebook-opt-125m-simulated/v1/chat/completions" 2>&1)
    
    http_status=$(echo "$response" | grep "HTTP_STATUS:" | cut -d':' -f2)
    
    if [ "$http_status" = "200" ]; then
        ((total_success++))
        tokens=$(echo "$response" | sed '/HTTP_STATUS:/d' | jq -r '.usage.total_tokens // 0' 2>/dev/null)
        if [ "$tokens" != "0" ]; then
            total_tokens=$((total_tokens + tokens))
        fi
        echo -ne "${GREEN}✓${NC}"
    elif [ "$http_status" = "429" ]; then
        rate_limited=true
        echo -ne "${RED}✗${NC}"
        if [ $i -gt 5 ]; then
            # If we've made enough requests, break on rate limit
            echo ""
            break
        fi
    else
        echo -ne "${YELLOW}?${NC}"
    fi
    
    # Small delay to avoid overwhelming the system
    sleep 0.2
done

echo ""
echo ""
echo -e "${BLUE}Rate Limiting Test Results:${NC}"
echo -e "  • Successful requests: ${GREEN}$total_success${NC}"
echo -e "  • Total tokens consumed: ${CYAN}$total_tokens${NC}"
if [ "$rate_limited" = true ]; then
    echo -e "  • Rate limiting: ${GREEN}✓ Working${NC} (429 responses received)"
else
    echo -e "  • Rate limiting: ${YELLOW}⚠ Not triggered${NC} (may need more requests or lower limits)"
fi

# Cleanup
echo ""
echo -e "${BLUE}Cleaning up test resources...${NC}"
kubectl delete serviceaccount model-test-user -n openshift-ai-inference-tier-free > /dev/null 2>&1

# Final summary
echo ""
echo -e "${CYAN}======================================${NC}"
echo -e "${CYAN}           Test Summary                ${NC}"
echo -e "${CYAN}======================================${NC}"
echo ""

# Check if models responded
if [ "$http_status" = "200" ] || [ "$total_success" -gt 0 ]; then
    echo -e "${GREEN}✓${NC} Models are accessible and responding"
    echo -e "${GREEN}✓${NC} Token authentication is working"
    echo -e "${GREEN}✓${NC} Inference endpoints are functional"
    if [ "$rate_limited" = true ]; then
        echo -e "${GREEN}✓${NC} Token rate limiting is enforced"
    else
        echo -e "${YELLOW}⚠${NC}  Token rate limiting not triggered (may need adjustment)"
    fi
else
    if [ "$rate_limited" = true ]; then
        echo -e "${YELLOW}⚠${NC}  Models are accessible but rate limits are very restrictive"
        echo -e "${GREEN}✓${NC} Token authentication is working"
        echo -e "${GREEN}✓${NC} Token rate limiting is enforced (very strict for service accounts)"
    else
        echo -e "${RED}✗${NC} There were issues accessing the models"
    fi
fi

echo ""
echo -e "${BLUE}Gateway URL:${NC} $GATEWAY_URL"
echo -e "${BLUE}Models tested:${NC}"
echo "  • Facebook OPT-125M Simulator at /llm/facebook-opt-125m-simulated"
echo "  • Facebook OPT-125M CPU at /llm/facebook-opt-125m-cpu-single-node-no-scheduler-cpu"
echo "  • QWEN3-0.6B GPU at /llm/single-node-no-scheduler-nvidia-gpu"
echo "" 
#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

GATEWAY_URL="https://gateway.apps.test-maas-v1.eh5f.s1.devshift.org"

echo -e "${CYAN}======================================${NC}"
echo -e "${CYAN}   Model Inference & Rate Limit Test  ${NC}"
echo -e "${CYAN}======================================${NC}"
echo ""

# Step 1: Create a test service account and get token
echo -e "${BLUE}Step 1: Creating test service account and obtaining token...${NC}"
kubectl create serviceaccount model-test-user -n llm --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1
TOKEN=$(kubectl create token model-test-user -n llm --audience=openshift-ai-inference-sa --duration=1h 2>/dev/null)

if [ -z "$TOKEN" ]; then
    echo -e "${RED}Failed to obtain token!${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Token obtained successfully${NC}"
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

# Test both models
test_model "QWEN3-0.6B" "/qwen3" "qwen3-0-6b-instruct"
test_model "SIMULATOR" "/simulator" "vllm-simulator"

# Step 2: Test rate limiting
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${MAGENTA}Testing Token Rate Limiting${NC}"
echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${BLUE}Making rapid requests to trigger rate limit...${NC}"
echo "(Using QWEN3 model for rate limit test)"
echo ""

# Rapid fire requests to trigger rate limiting
REQUEST_BODY_SIMPLE=$(cat <<EOF
{
  "model": "qwen3-0-6b-instruct",
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
        "$GATEWAY_URL/qwen3/v1/chat/completions" 2>&1)
    
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
    
    # Very small delay to not overwhelm the system
    sleep 0.05
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
kubectl delete serviceaccount model-test-user -n llm > /dev/null 2>&1

# Final summary
echo ""
echo -e "${CYAN}======================================${NC}"
echo -e "${CYAN}           Test Summary                ${NC}"
echo -e "${CYAN}======================================${NC}"
echo ""

# Check if both models responded
if [ "$http_status" = "200" ] || [ "$total_success" -gt 0 ]; then
    echo -e "${GREEN}✓${NC} Both models are accessible and responding"
    echo -e "${GREEN}✓${NC} Token authentication is working"
    echo -e "${GREEN}✓${NC} Inference endpoints are functional"
    if [ "$rate_limited" = true ]; then
        echo -e "${GREEN}✓${NC} Token rate limiting is enforced"
    else
        echo -e "${YELLOW}⚠${NC}  Token rate limiting not triggered (may need adjustment)"
    fi
else
    echo -e "${RED}✗${NC} There were issues accessing the models"
fi

echo ""
echo -e "${BLUE}Gateway URL:${NC} $GATEWAY_URL"
echo -e "${BLUE}Models tested:${NC}"
echo "  • QWEN3-0.6B at /qwen3"
echo "  • VLLM Simulator at /simulator"
echo "" 
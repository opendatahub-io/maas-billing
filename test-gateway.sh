#!/bin/bash

echo "==================================================================="
echo "Testing MaaS API Gateway Routing - Complete Flow"
echo "==================================================================="

# Get gateway host
HOST="$(kubectl get gateway openshift-ai-inference -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"
echo "Gateway Host: $HOST"

# Get token
echo -e "\n1. Getting token from gateway..."
TOKEN=$(curl -s \
  -H "Authorization: Bearer $(oc whoami -t)" \
  -H "Content-Type: application/json" \
  -X POST -d '{"expiration": "10m"}' \
  "http://${HOST}/maas-api/v1/tokens" | jq -r .token)

if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
  echo "✓ Token obtained successfully"
  echo "  Token (first 50 chars): ${TOKEN:0:50}..."
else
  echo "✗ Failed to get token"
  exit 1
fi

# Test Simulator
echo -e "\n2. Testing Simulator Model..."
echo "   POST http://${HOST}/simulator/v1/chat/completions"
curl -s -X POST "http://${HOST}/simulator/v1/chat/completions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "vllm-simulator",
    "messages": [{"role": "user", "content": "Hello!"}]
  }' | jq '.'

# Test Qwen (will fail if no GPU)
echo -e "\n3. Testing Qwen3 Model (requires GPU)..."
echo "   POST http://${HOST}/qwen3/v1/chat/completions"
curl -s -X POST "http://${HOST}/qwen3/v1/chat/completions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3-0-6b-instruct",
    "messages": [{"role": "user", "content": "Hello!"}]
  }' | jq '.' || echo "Note: Qwen3 requires GPU nodes to work"

echo -e "\n==================================================================="
echo "Test Complete!"
echo "==================================================================="

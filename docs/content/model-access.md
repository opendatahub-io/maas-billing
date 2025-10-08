# Model Access Guide

This guide explains how to interact with deployed models on the MaaS Platform, including authentication, making requests, and handling responses.

## Overview

The MaaS Platform provides a secure, tier-based model access system where:
- Users authenticate with tokens obtained from the MaaS API
- Access is controlled by subscription tiers (free, premium, enterprise)
- Rate limiting and token consumption tracking are enforced
- Models are accessed through the MaaS gateway for policy enforcement

## Authentication

### Getting a Token

Before accessing models, you need to obtain an authentication token:

```bash
# Get your OpenShift token
OC_TOKEN=$(oc whoami -t)

# Set your MaaS API endpoint
HOST="https://maas-api.your-domain.com"
MAAS_API_URL="${HOST}/maas-api"

# Request an access token
TOKEN_RESPONSE=$(curl -sSk \
  -H "Authorization: Bearer ${OC_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"expiration": "15m"}' \
  "${MAAS_API_URL}/v1/tokens")

ACCESS_TOKEN=$(echo $TOKEN_RESPONSE | jq -r .token)
```

### Token Lifecycle

- **Default lifetime**: 4 hours
- **Maximum lifetime**: Determined by cluster configuration
- **Refresh**: Request a new token before expiration
- **Revocation**: Tokens can be revoked if compromised

## Discovering Models

### List Available Models

Get a list of models available to your tier:

```bash
MODELS=$(curl ${HOST}/maas-api/v1/models \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

echo $MODELS | jq .
```

Example response:
```json
{
  "data": [
    {
      "id": "simulator",
      "name": "Simulator Model",
      "url": "https://gateway.your-domain.com/simulator/v1/chat/completions",
      "tier": "free"
    },
    {
      "id": "qwen3",
      "name": "Qwen3 Model",
      "url": "https://gateway.your-domain.com/qwen3/v1/chat/completions",
      "tier": "premium"
    }
  ]
}
```

### Get Model Details

Get detailed information about a specific model:

```bash
MODEL_ID="simulator"
MODEL_INFO=$(curl ${HOST}/maas-api/v1/models \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" | \
    jq --arg model "$MODEL_ID" '.data[] | select(.id == $model)')

echo $MODEL_INFO | jq .
```

## Making Requests

### Basic Chat Completion

Make a simple chat completion request:

```bash
MODEL_URL="https://gateway.your-domain.com/simulator/v1/chat/completions"

curl -sSk \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
        "model": "simulator",
        "messages": [
          {
            "role": "user",
            "content": "Hello, how are you?"
          }
        ],
        "max_tokens": 100
      }' \
  "${MODEL_URL}"
```

### Advanced Request Parameters

Use additional parameters for more control:

```bash
curl -sSk \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
        "model": "simulator",
        "messages": [
          {
            "role": "system",
            "content": "You are a helpful assistant."
          },
          {
            "role": "user",
            "content": "Explain quantum computing in simple terms."
          }
        ],
        "max_tokens": 200,
        "temperature": 0.7,
        "top_p": 0.9,
        "stream": false
      }' \
  "${MODEL_URL}"
```

### Streaming Responses

For real-time responses, use streaming:

```bash
curl -sSk \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
        "model": "simulator",
        "messages": [
          {
            "role": "user",
            "content": "Write a short story about a robot."
          }
        ],
        "max_tokens": 300,
        "stream": true
      }' \
  "${MODEL_URL}" | while IFS= read -r line; do
    if [[ $line == data:* ]]; then
      echo "${line#data: }" | jq -r '.choices[0].delta.content // empty' 2>/dev/null
    fi
  done
```

## Handling Responses

### Standard Response Format

Models return responses in the OpenAI-compatible format:

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "simulator",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! I am doing well, thank you for asking."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 12,
    "total_tokens": 21
  }
}
```

### Processing Responses

Extract content from responses:

```bash
RESPONSE=$(curl -sSk \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
        "model": "simulator",
        "messages": [
          {
            "role": "user",
            "content": "What is the capital of France?"
          }
        ],
        "max_tokens": 50
      }' \
  "${MODEL_URL}")

# Extract the response content
CONTENT=$(echo $RESPONSE | jq -r '.choices[0].message.content')
echo "Model response: $CONTENT"

# Extract token usage
PROMPT_TOKENS=$(echo $RESPONSE | jq -r '.usage.prompt_tokens')
COMPLETION_TOKENS=$(echo $RESPONSE | jq -r '.usage.completion_tokens')
TOTAL_TOKENS=$(echo $RESPONSE | jq -r '.usage.total_tokens')

echo "Token usage: $TOTAL_TOKENS total ($PROMPT_TOKENS prompt + $COMPLETION_TOKENS completion)"
```

## Error Handling

### Common Error Responses

**401 Unauthorized**
```json
{
  "error": {
    "message": "Invalid authentication token",
    "type": "invalid_request_error",
    "code": "invalid_api_key"
  }
}
```

**403 Forbidden**
```json
{
  "error": {
    "message": "Insufficient permissions for this model",
    "type": "permission_error",
    "code": "access_denied"
  }
}
```

**429 Too Many Requests**
```json
{
  "error": {
    "message": "Rate limit exceeded",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded"
  }
}
```

### Handling Errors in Scripts

```bash
make_request() {
  local model_url="$1"
  local prompt="$2"
  
  response=$(curl -sSk \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
          \"model\": \"simulator\",
          \"messages\": [
            {
              \"role\": \"user\",
              \"content\": \"$prompt\"
            }
          ],
          \"max_tokens\": 100
        }" \
    "${model_url}")
  
  # Check for errors
  if echo "$response" | jq -e '.error' > /dev/null; then
    error_message=$(echo "$response" | jq -r '.error.message')
    error_code=$(echo "$response" | jq -r '.error.code')
    echo "Error: $error_message (Code: $error_code)" >&2
    return 1
  fi
  
  # Extract and return content
  echo "$response" | jq -r '.choices[0].message.content'
}

# Usage
if result=$(make_request "$MODEL_URL" "Hello, world!"); then
  echo "Success: $result"
else
  echo "Request failed"
fi
```

## Rate Limiting and Quotas

### Understanding Limits

Each tier has different limits:

| Tier | Requests/2min | Tokens/min |
|------|---------------|------------|
| Free | 5 | 100 |
| Premium | 20 | 50,000 |
| Enterprise | 50 | 100,000 |

### Monitoring Usage

Check your current usage through response headers or the metrics dashboard:

```bash
# Make a request and check headers
curl -I -sSk \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"model": "simulator", "messages": [{"role": "user", "content": "test"}]}' \
  "${MODEL_URL}" | grep -i "x-ratelimit"
```

### Implementing Rate Limiting

For applications that need to respect rate limits:

```bash
# Simple rate limiting implementation
make_request_with_backoff() {
  local model_url="$1"
  local prompt="$2"
  local max_retries=3
  local retry_count=0
  
  while [ $retry_count -lt $max_retries ]; do
    response=$(curl -sSk \
      -H "Authorization: Bearer ${ACCESS_TOKEN}" \
      -H "Content-Type: application/json" \
      -d "{
            \"model\": \"simulator\",
            \"messages\": [
              {
                \"role\": \"user\",
                \"content\": \"$prompt\"
              }
            ],
            \"max_tokens\": 100
          }" \
      "${model_url}")
    
    # Check for rate limit error
    if echo "$response" | jq -e '.error.code == "rate_limit_exceeded"' > /dev/null; then
      retry_count=$((retry_count + 1))
      echo "Rate limit exceeded, waiting before retry $retry_count/$max_retries..." >&2
      sleep 30  # Wait 30 seconds before retry
    else
      echo "$response" | jq -r '.choices[0].message.content'
      return 0
    fi
  done
  
  echo "Max retries exceeded" >&2
  return 1
}
```

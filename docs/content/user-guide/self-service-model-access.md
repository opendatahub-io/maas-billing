# Self Service Model Access

This guide is for **end users** who want to use AI models through the MaaS platform.

## 🎯 What is MaaS?

The Model-as-a-Service (MaaS) platform provides access to AI models through a simple API. Your organization's administrator has set up the platform and configured access for your team.

## Getting Your Access Token

### Step 1: Get Your OpenShift Authentication Token

First, you need your OpenShift token to prove your identity to the maas-api.

```bash
# Log in to your OpenShift cluster if you haven't already
oc login ...

# Get your current OpenShift authentication token
OC_TOKEN=$(oc whoami -t)
```

### Step 2: Request an Access Token from the API

Next, use that OpenShift token to call the maas-api `/v1/tokens` endpoint. You can specify the desired expiration time; the default is 4 hours.

```bash
HOST="https://maas.yourdomain.io"
MAAS_API_URL="${HOST}/maas-api"

TOKEN_RESPONSE=$(curl -sSk \
  -H "Authorization: Bearer ${OC_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"expiration": "15m"}' \
  "${MAAS_API_URL}/v1/tokens")

ACCESS_TOKEN=$(echo $TOKEN_RESPONSE | jq -r .token)

echo $ACCESS_TOKEN
```

!!! note
    Replace `HOST` with the actual route to your `maas-api` instance.

### Token Lifecycle

- **Default lifetime**: 4 hours (configurable when requesting)
- **Maximum lifetime**: Determined by cluster configuration
- **Refresh**: Request a new token before expiration
- **Revocation**: Tokens can be revoked if compromised

## Discovering Models

### List Available Models

Get a list of models available to your tier:

```bash
MODELS=$(curl "${MAAS_API_URL}/v1/models" \
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
MODEL_INFO=$(curl "${MAAS_API_URL}/v1/models" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" | \
    jq --arg model "$MODEL_ID" '.data[] | select(.id == $model)')

echo $MODEL_INFO | jq .
```

## Making Inference Requests

### Basic Chat Completion

Make a simple chat completion request:

```bash
# First, get the model URL from the models endpoint
MODELS=$(curl "${MAAS_API_URL}/v1/models" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")
MODEL_URL=$(echo $MODELS | jq -r '.data[0].url')
MODEL_NAME=$(echo $MODELS | jq -r '.data[0].id')

curl -sSk \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
        \"model\": \"${MODEL_NAME}\",
        \"messages\": [
          {
            \"role\": \"user\",
            \"content\": \"Hello, how are you?\"
          }
        ],
        \"max_tokens\": 100
      }" \
  "${MODEL_URL}/v1/chat/completions"
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
  "${MODEL_URL}/v1/chat/completions"
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
  "${MODEL_URL}/v1/chat/completions" | while IFS= read -r line; do
    if [[ $line == data:* ]]; then
      echo "${line#data: }" | jq -r '.choices[0].delta.content // empty' 2>/dev/null
    fi
  done
```

## Processing Responses

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

### Extract Response Content

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
  "${MODEL_URL}/v1/chat/completions")

# Extract the response content
CONTENT=$(echo $RESPONSE | jq -r '.choices[0].message.content')
echo "Model response: $CONTENT"

# Extract token usage
PROMPT_TOKENS=$(echo $RESPONSE | jq -r '.usage.prompt_tokens')
COMPLETION_TOKENS=$(echo $RESPONSE | jq -r '.usage.completion_tokens')
TOTAL_TOKENS=$(echo $RESPONSE | jq -r '.usage.total_tokens')

echo "Token usage: $TOTAL_TOKENS total ($PROMPT_TOKENS prompt + $COMPLETION_TOKENS completion)"
```

## Understanding Your Access Level

Your access is determined by your **tier**, which controls:

- **Available models** - Which AI models you can use
- **Request limits** - How many requests per minute
- **Token limits** - Maximum tokens per request
- **Features** - Advanced capabilities available

### Common Tiers

| Tier | Requests/min | Tokens/min |
|------|--------------|------------|
| Free | 5 | 100 |
| Premium | 20 | 50,000 |
| Enterprise | 50 | 100,000 |

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

## Monitoring Usage

Check your current usage through response headers:

```bash
# Make a request and check headers
curl -I -sSk \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"model": "simulator", "messages": [{"role": "user", "content": "test"}]}' \
  "${MODEL_URL}/v1/chat/completions" | grep -i "x-ratelimit"
```

## ⚠️ Common Issues

### Authentication Errors

**Problem**: `401 Unauthorized`

**Solution**: Check your token and ensure it's correctly formatted:

```bash
# Correct format
-H "Authorization: Bearer YOUR_TOKEN"

# Wrong format
-H "Authorization: YOUR_TOKEN"
```

### Rate Limit Exceeded

**Problem**: `429 Too Many Requests`

**Solution**: Wait before making more requests, or contact your administrator to upgrade your tier.

### Model Not Available

**Problem**: `404 Model Not Found`

**Solution**: Check which models are available in your tier:

```bash
curl -X GET "${MAAS_API_URL}/v1/models" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

## 💡 Best Practices

1. **Request tokens** with appropriate expiration times for your use case
2. **Refresh tokens** proactively before they expire
3. **Handle errors** gracefully in your scripts
4. **Monitor your usage** to stay within tier limits
5. **Batch requests** when possible to be efficient
6. **Cache responses** when appropriate

## FAQs

**Q: My tier is wrong or shows as "free". How do I fix it?**

A: Your tier is determined by your group membership in OpenShift. Contact your platform administrator to ensure you are in the correct user group.

---

**Q: How long should my tokens be valid for?**

A: It's a balance of security and convenience. For interactive command-line use, 1-8 hours is common. For applications, request shorter-lived tokens (e.g., 15-60 minutes) and refresh them automatically.

---

**Q: Can I have multiple active tokens at once?**

A: Yes. Each call to the `/v1/tokens` endpoint issues a new, independent token. All of them will be valid until they expire or are revoked.

---

**Q: Can I use one token to access multiple different models?**

A: Yes. Your token grants you access based on your tier's RBAC permissions. If your tier is authorized to use multiple models, a single token will work for all of them.

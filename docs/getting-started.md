# Getting Started

This guide helps you get started with the MaaS Platform as an end user. You'll learn how to obtain tokens, access models, and understand the tier-based access system.

## Prerequisites

- Access to a deployed MaaS Platform
- OpenShift cluster access (`oc` command)
- `curl` and `jq` tools installed

## Quick Start

### 1. Get Your Authentication Token

First, you need to obtain a token to access the models:

```bash
# Get your OpenShift token
OC_TOKEN=$(oc whoami -t)

# Set the token expiration time
TOKEN_EXPIRATION="15m"

# Get the MaaS API endpoint (replace with your actual domain)
HOST="$(kubectl get gateway maas-default-gateway -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')"
MAAS_API_URL="${HOST}/maas-api"

# Request an access token
TOKEN_RESPONSE=$(curl -sSk \
  -H "Authorization: Bearer ${OC_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"expiration": "${TOKEN_EXPIRATION}"}' \
  "${MAAS_API_URL}/v1/tokens")

ACCESS_TOKEN=$(echo $TOKEN_RESPONSE | jq -r .token)
echo "Your access token: $ACCESS_TOKEN"
```

### 2. List Available Models

See what models are available for your tier:

```bash
MODELS=$(curl ${HOST}/maas-api/v1/models \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}")

echo $MODELS | jq .
```

### 3. Make Your First Request

Use the token to make a request to a model:

> [!NOTE]
> You can change `data[0]` to `data[1]` or any other index to use a different model.

```bash
# Get model details
MODEL_URL=$(echo $MODELS | jq -r '.data[0].url')
MODEL_NAME=$(echo $MODELS | jq -r '.data[0].id')

# Make a request
curl -sSk \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
        \"model\": \"${MODEL_NAME}\",
        \"prompt\": \"Hello, how are you?\",
        \"max_tokens\": 50
    }" \
  "${MODEL_URL}/v1/chat/completions"
```

## Understanding Tiers

The MaaS Platform uses a tier-based access system, these are the default tiers:

### Free Tier
- **Access**: Basic models
- **Rate Limits**: 5 requests per 2 minutes
- **Token Limits**: 100 tokens per minute
- **Groups**: `system:authenticated`

### Premium Tier
- **Access**: All models including GPU-accelerated ones
- **Rate Limits**: 20 requests per 2 minutes
- **Token Limits**: 50,000 tokens per minute
- **Groups**: `premium-users`

### Enterprise Tier
- **Access**: All models with priority scheduling
- **Rate Limits**: 50 requests per 2 minutes
- **Token Limits**: 100,000 tokens per minute
- **Groups**: `enterprise-users`

## Token Management

### Token Lifecycle

- **Default lifetime**: 4 hours
- **Maximum lifetime**: Determined by cluster configuration
- **Refresh**: Request a new token before expiration

### Token Security

- Tokens are short-lived for security
- Store tokens securely
- Don't share tokens with others
- Revoke tokens if compromised

### Revoking Tokens

To revoke all your active tokens:

```bash
curl -sSk -X DELETE "${MAAS_API_URL}/v1/tokens" \
  -H "Authorization: Bearer $(oc whoami -t)"
```

## Rate Limits and Quotas

### Request Rate Limits

Each tier has different request rate limits:
- **Free**: 5 requests per 2 minutes
- **Premium**: 20 requests per 2 minutes
- **Enterprise**: 50 requests per 2 minutes

### Token Consumption Limits

Track your LLM token usage:
- **Free**: 100 tokens per minute
- **Premium**: 50,000 tokens per minute
- **Enterprise**: 100,000 tokens per minute

### Checking Your Usage

Monitor your current usage through the platform's metrics dashboard or by checking response headers.

## Common Tasks

### Switching Between Models

```bash
# List all available models
curl ${HOST}/maas-api/v1/models \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" | jq '.data[].id'

# Use a specific model
MODEL_NAME="simulator"  # or any other model ID
MODEL_URL=$(curl ${HOST}/maas-api/v1/models \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" | \
    jq -r --arg model "$MODEL_NAME" '.data[] | select(.id == $model) | .url')
```

### Batch Processing

For multiple requests, implement proper rate limiting:

```bash
# Example: Process multiple prompts with rate limiting
prompts=("Hello world" "How are you?" "What's the weather?")

for prompt in "${prompts[@]}"; do
  curl -sSk \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{
          \"model\": \"${MODEL_NAME}\",
          \"prompt\": \"${prompt}\",
          \"max_tokens\": 50
      }" \
    "${MODEL_URL}/v1/chat/completions"
  
  # Wait to respect rate limits
  sleep 10
done
```

## Troubleshooting

### Common Issues

**401 Unauthorized**
- Token has expired - request a new one
- Invalid token format - check token extraction

**403 Forbidden**
- Insufficient permissions for the model
- Check your tier assignment

**429 Too Many Requests**
- Rate limit exceeded
- Wait before making more requests

**Model Not Found**
- Model may not be deployed
- Check available models list

### Getting Help

1. Check the [Token Management](token-management.md) guide for detailed token information
2. Contact your platform administrator for tier-related issues
3. Review the [API Reference](api-reference.md) for complete API documentation

## Next Steps

- Learn about [Token Management](token-management.md) for advanced token handling
- Explore [Model Access](model-access.md) for detailed model interaction patterns
- Check [Samples](samples/) for example configurations and use cases


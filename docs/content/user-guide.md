# User Guide

This guide is for **end users** who want to use AI models through the MaaS platform.

## 🎯 What is MaaS?

The Model-as-a-Service (MaaS) platform provides access to AI models through a simple API. Your organization's administrator has set up the platform and configured access for your team.

## 🚀 Getting Started

### Prerequisites

Before you can use the platform, you need:

- **Access credentials** (provided by your administrator)
- **API endpoint** (provided by your administrator)
- **Basic understanding** of REST APIs

### Getting Your Credentials

Contact your platform administrator to obtain:

1. **API endpoint URL** - Where to send your requests
2. **Authentication token** - Your access key
3. **Available models** - Which AI models you can use
4. **Usage limits** - How many requests you can make

## 📡 Using the API

### Basic Request Format

All requests follow this pattern:

```bash
curl -X POST "https://your-maas-endpoint.com/{model-name}/v1/chat/completions" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
        \"model\": \"${MODEL_NAME}\",
        \"prompt\": \"Input Text Here\",
        \"max_prompts\": 40
    }"
```

### Example: Text Generation

```bash
curl -X POST "https://your-maas-endpoint.com/facebook-opt-125m-cpu/v1/chat/completions" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
        \"model\": \"facebook-opt-125m-cpu\",
        \"prompt\": \"Not really understood prompt\",
        \"max_prompts\": 40
    }"
```

### Example: Question Answering

```bash
curl -X POST "https://your-maas-endpoint.com/qwen3/v1/chat/completions" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
        \"model\": \"qwen3\",
        \"prompt\": \"Not really understood prompt\",
        \"max_prompts\": 40
    }"
```

## 🔧 Understanding Your Access Level

Your access is determined by your **tier**, which controls:

- **Available models** - Which AI models you can use
- **Request limits** - How many requests per minute
- **Token limits** - Maximum tokens per request
- **Features** - Advanced capabilities available

### Common Tiers

- **Basic**: Limited models, lower request limits
- **Premium**: More models, higher limits
- **Enterprise**: All models, highest limits

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
curl -X GET "https://your-maas-endpoint.com/v1/models" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 💡 Best Practices

### Efficient Usage

1. **Batch requests** when possible
2. **Use appropriate token limits** for your needs
3. **Cache responses** when appropriate
4. **Monitor your usage** to stay within limits

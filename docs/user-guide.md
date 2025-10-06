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
curl -X POST "https://your-maas-endpoint.com/v1/models/{model-name}/infer" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": "Your input text here"
  }'
```

### Example: Text Generation

```bash
curl -X POST "https://your-maas-endpoint.com/v1/models/facebook-opt-125m-cpu/infer" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": "The future of artificial intelligence is"
  }'
```

### Example: Question Answering

```bash
curl -X POST "https://your-maas-endpoint.com/v1/models/qwen3/infer" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": "What is machine learning?",
    "parameters": {
      "max_tokens": 100
    }
  }'
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

## 📊 Monitoring Your Usage

### Check Your Limits

```bash
curl -X GET "https://your-maas-endpoint.com/v1/usage" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### View Available Models

```bash
curl -X GET "https://your-maas-endpoint.com/v1/models" \
  -H "Authorization: Bearer YOUR_TOKEN"
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
curl -X GET "https://your-maas-endpoint.com/v1/models" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 💡 Best Practices

### Efficient Usage

1. **Batch requests** when possible
2. **Use appropriate token limits** for your needs
3. **Cache responses** when appropriate
4. **Monitor your usage** to stay within limits

### Error Handling

Always implement proper error handling in your applications:

```python
import requests

def call_maas_api(prompt, model, token):
    try:
        response = requests.post(
            f"https://your-maas-endpoint.com/v1/models/{model}/infer",
            headers={"Authorization": f"Bearer {token}"},
            json={"inputs": prompt}
        )
        response.raise_for_status()
        return response.json()
    except requests.exceptions.HTTPError as e:
        if e.response.status_code == 429:
            print("Rate limit exceeded. Please wait.")
        elif e.response.status_code == 401:
            print("Authentication failed. Check your token.")
        else:
            print(f"API error: {e}")
    except Exception as e:
        print(f"Unexpected error: {e}")
```

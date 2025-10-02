# Sample LLMInferenceService Models

This directory contains  `LLMInferenceService` for deployment of sample models. Please refer to the [deployment guide](../deployment/README.md) for more details of how to test MaaS Platform with these models.

# Deployment

```bash
MODEL_NAME=simulator # or facebook-opt-125m-cpu or qwen3
kustomize build $MODEL_NAME | kubectl apply -f -
```

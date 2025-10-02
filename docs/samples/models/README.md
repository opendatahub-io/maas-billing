# Sample LLMInferenceService Models

This directory contains `LLMInferenceService`s for deploying sample models. Please refer to the [deployment guide](../../deployment/README.md) for more details on how to test the MaaS Platform with these models.
# Deployment

```bash
MODEL_NAME=simulator # or facebook-opt-125m-cpu or qwen3
kustomize build $MODEL_NAME | kubectl apply -f -
```

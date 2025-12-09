# Sample LLMInferenceService Models

This directory contains `LLMInferenceService`s for deploying sample models. Please refer to the [deployment guide](../../content/quickstart.md) for more details on how to test the MaaS Platform with these models.

## Available Models

- **simulator** - Simple simulator for testing
- **facebook-opt-125m-cpu** - Facebook OPT 125M model (CPU-based)
- **qwen3** - Qwen3 model (GPU-based with autoscaling)
- **ibm-granite-2b-gpu** - IBM Granite 2B Instruct model (GPU-based, supports instructions)

# Deployment

```bash
MODEL_NAME=simulator # or facebook-opt-125m-cpu or qwen3 or ibm-granite-2b-gpu
kustomize build $MODEL_NAME | kubectl apply -f -
```

# e2e Smoke Tests

## Setup (one-time per workspace)

**Prereqs:** Python 3.11+, `oc`, `kubectl`, `kustomize`, `jq` (and `yq` optional).

```bash
# create & activate a virtualenv
python3.11 -m venv .venv
source .venv/bin/activate          # Windows PowerShell: .\.venv\Scripts\Activate.ps1

# install Python deps used by the tests
pip install --upgrade pip
pip install -r test/e2e/requirements.txt

## What Prow needs to provide (exports)
- `CLUSTER_DOMAIN` – from cluster (e.g., `oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}'`)
- `HOST` – maas.${CLUSTER_DOMAIN}
- `MAAS_API_BASE_URL` – `https://${HOST}/maas-api` (or `http://` if TLS isn’t ready)
- `MODEL_NAME` – gateway model id (e.g., `facebook/opt-125m`)
- `ISVC_NAME` (optional) – CR name without slashes (e.g., `facebook-opt-125m-cpu`) if you deploy a sample

## Typical run
```bash
export CLUSTER_DOMAIN="$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')"
export HOST="maas.${CLUSTER_DOMAIN}"
export MAAS_API_BASE_URL="https://${HOST}/maas-api"
export MODEL_NAME="facebook/opt-125m"
# optional if you deploy a sample:
# export ISVC_NAME="facebook-opt-125m-cpu"

# Deploy + smoke:
bash test/e2e/run-model-and-smoke.sh
# Or only smoke (if model is already there):
bash test/e2e/smoke.sh

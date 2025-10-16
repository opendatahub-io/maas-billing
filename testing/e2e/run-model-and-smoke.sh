#!/usr/bin/env bash
set -euo pipefail

# -------- Config (overridable) --------
NS="${NS:-llm}"
MODEL_PATH="${MODEL_PATH:-docs/samples/models/facebook-opt-125m-cpu}"
ARTIFACT_DIR="${ARTIFACT_DIR:-testing/e2e/reports}"

# -------- Inputs we require --------
: "${MODEL_NAME:?Export MODEL_NAME (LLMInferenceService name)}"

# One of these must be provided (we'll derive HOST/API if needed)
CLUSTER_DOMAIN="${CLUSTER_DOMAIN:-}"
HOST="${HOST:-}"
MAAS_API_BASE_URL="${MAAS_API_BASE_URL:-}"

echo "[e2e] NS=${NS}"
echo "[e2e] MODEL_NAME=${MODEL_NAME}"
echo "[e2e] MODEL_PATH=${MODEL_PATH}"

command -v oc >/dev/null || { echo "oc missing"; exit 1; }
command -v kustomize >/dev/null || { echo "kustomize missing"; exit 1; }

# -------- Deploy model --------
oc get ns "${NS}" >/dev/null 2>&1 || oc create ns "${NS}"
echo "[e2e] Applying ${MODEL_PATH} to ns ${NS}"
kustomize build "${MODEL_PATH}" | kubectl -n "${NS}" apply -f -
echo "[e2e] Waiting for LLMInferenceService/${MODEL_NAME} Ready…"
ISVC_NAME="${ISVC_NAME:-}"
if [[ -z "${ISVC_NAME}" ]]; then
  if command -v yq >/dev/null 2>&1; then
    ISVC_NAME="$(kustomize build "${MODEL_PATH}" | yq -r 'select(.kind=="LLMInferenceService") | .metadata.name' | head -n1)"
  else
    ISVC_NAME="$(kustomize build "${MODEL_PATH}" | awk '/^kind: LLMInferenceService$/{f=1} f&&/^  name:/{print $2; exit}')"
  fi
fi
oc -n "${NS}" wait --for=condition=Ready "llminferenceservice/${ISVC_NAME}" --timeout=15m

# -------- Work out API base URL (simple rules) --------
if [[ -z "${MAAS_API_BASE_URL}" ]]; then
  if [[ -z "${HOST}" ]]; then
    if [[ -z "${CLUSTER_DOMAIN}" ]]; then
      echo "[e2e] ERROR: set MAAS_API_BASE_URL or HOST or CLUSTER_DOMAIN" >&2
      exit 2
    fi
    HOST="maas.${CLUSTER_DOMAIN}"
  fi
  SCHEME="https"
  if ! curl -skI -m 5 "${SCHEME}://${HOST}/maas-api/healthz" >/dev/null; then
    SCHEME="http"
  fi
  MAAS_API_BASE_URL="${SCHEME}://${HOST}/maas-api"
fi

export HOST
export MAAS_API_BASE_URL
export MODEL_NAME

echo "[e2e] HOST=${HOST}"
echo "[e2e] MAAS_API_BASE_URL=${MAAS_API_BASE_URL}"

# -------- Run smoke --------
mkdir -p "${ARTIFACT_DIR}"
echo "[e2e] Running smoke tests…"
( cd testing/e2e && bash ./smoke.sh )

# Copy artifacts if a different dir was requested
if [[ "testing/e2e/reports" != "${ARTIFACT_DIR}" ]]; then
  cp -r testing/e2e/reports/. "${ARTIFACT_DIR}/"
fi
echo "[e2e] Done. Reports in ${ARTIFACT_DIR}"

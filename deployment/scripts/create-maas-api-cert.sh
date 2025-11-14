#!/usr/bin/env bash
set -euo pipefail

SECRET_NAME=${SECRET_NAME:-maas-api-backend-tls}
NAMESPACE=${NAMESPACE:-maas-api}
CERT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/tls-certs"
CERT_PATH="${CERT_DIR}/maas-api-server.crt"
KEY_PATH="${CERT_DIR}/maas-api-server.key"
DAYS=${DAYS:-365}
FORCE=0

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--force]

Options:
  --force    Regenerate certificates even if files already exist.

Environment variables:
  NAMESPACE   Namespace where the secret should be created (default: maas-api)
  SECRET_NAME Override secret name (default: maas-api-backend-tls)
  DAYS        Validity period for the certificate in days (default: 365)
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --force)
      FORCE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

mkdir -p "$CERT_DIR"

if [[ $FORCE -eq 0 && -f "$CERT_PATH" && -f "$KEY_PATH" ]]; then
  echo "🔒 TLS assets already exist at $CERT_DIR (use --force to regenerate)"
else
  echo "🔐 Generating self-signed certificate for maas-api"
  openssl req \
    -x509 \
    -nodes \
    -sha256 \
    -days "$DAYS" \
    -newkey rsa:4096 \
    -keyout "$KEY_PATH" \
    -out "$CERT_PATH" \
    -subj "/CN=maas-api.maas-api.svc" \
    -addext "subjectAltName = DNS:maas-api.maas-api.svc,DNS:maas-api.maas-api.svc.cluster.local"
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required to create the TLS secret" >&2
  exit 1
fi

if [[ -z "$NAMESPACE" ]]; then
  echo "NAMESPACE must be set" >&2
  exit 1
fi

kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || {
  echo "Namespace $NAMESPACE does not exist" >&2
  exit 1
}

echo "📦 Creating/Updating secret $SECRET_NAME in namespace $NAMESPACE"

kubectl -n "$NAMESPACE" create secret tls "$SECRET_NAME" \
  --cert="$CERT_PATH" \
  --key="$KEY_PATH" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "✅ Secret $SECRET_NAME is ready"

#!/bin/bash
#
# This script deploys a PostgreSQL instance for testing maas-api.
#
# YAML configs are in: deployment/components/database/postgres/
#
# Namespace: Use NAMESPACE env var, default: postgres-maas
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
POSTGRES_DIR="$PROJECT_ROOT/deployment/components/database/postgres"

: "${NAMESPACE:=postgres-maas}"
: "${POSTGRES_USER:=maas}"
: "${POSTGRES_PASSWORD:=maas-secret}"
: "${POSTGRES_DB:=maas_api}"

# Ensure namespace exists
if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  echo "📦 Creating namespace '$NAMESPACE'..."
  kubectl create namespace "$NAMESPACE"
fi

echo "🔧 Deploying PostgreSQL to namespace '$NAMESPACE'..."
echo ""

# Create secret (not in YAML files to avoid committing credentials)
kubectl create secret generic postgres-credentials \
  --from-literal=POSTGRES_USER="$POSTGRES_USER" \
  --from-literal=POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  --from-literal=POSTGRES_DB="$POSTGRES_DB" \
  --namespace="$NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f -

# Apply PostgreSQL resources from YAML files
echo "📄 Applying PostgreSQL manifests from $POSTGRES_DIR..."
kustomize build "$POSTGRES_DIR" | kubectl apply -n "$NAMESPACE" -f -

echo "⏳ Waiting for PostgreSQL to be ready..."
kubectl wait --for=condition=available deployment/postgres -n "$NAMESPACE" --timeout=180s

if [ $? -eq 0 ]; then
  DATABASE_URL="postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres-service.${NAMESPACE}.svc:5432/${POSTGRES_DB}?sslmode=disable"
  echo ""
  echo "✅ PostgreSQL deployment successful."
  echo ""
  echo "📝 Next steps to configure maas-api:"
  echo ""
  echo "1. Create a Secret with the database URL in maas-api namespace:"
  echo ""
  echo "   kubectl create secret generic database-config \\"
  echo "     --from-literal=DATABASE_URL=\"$DATABASE_URL\" \\"
  echo "     --namespace=maas-api"
  echo ""
  echo "2. Restart maas-api deployment to pick up the new configuration:"
  echo ""
  echo "   kubectl rollout restart deployment/maas-api -n maas-api"
  echo ""
  echo "💡 Quick test (port-forward and connect):"
  echo "   kubectl port-forward -n $NAMESPACE svc/postgres-service 5432:5432 &"
  echo "   psql \"$DATABASE_URL\""
  echo ""
else
  echo "❌ PostgreSQL deployment failed or timed out."
  echo "Check the deployment status: kubectl describe deployment/postgres -n $NAMESPACE"
  exit 1
fi

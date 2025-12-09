#!/bin/bash
#
# Deploy a PostgreSQL instance for testing/development with maas-api.
#
# This script creates a simple PostgreSQL deployment for testing purposes.
# For production, use a proper PostgreSQL operator (CloudNativePG, Crunchy, etc.)
#
# Usage:
#   ./setup-postgres.sh                    # Deploy with defaults
#   NAMESPACE=my-ns ./setup-postgres.sh    # Deploy to custom namespace
#   ./setup-postgres.sh --delete           # Remove deployment
#
# Environment variables:
#   NAMESPACE         - Kubernetes namespace (default: postgres-maas)
#   POSTGRES_USER     - Database user (default: maas)
#   POSTGRES_PASSWORD - Database password (default: auto-generated)
#   POSTGRES_DB       - Database name (default: maas_api)
#   STORAGE_SIZE      - PVC size (default: 1Gi)
#

set -e

# Configuration
: "${NAMESPACE:=postgres-maas}"
: "${POSTGRES_USER:=maas}"
: "${POSTGRES_PASSWORD:=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)}"
: "${POSTGRES_DB:=maas_api}"
: "${STORAGE_SIZE:=1Gi}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_header() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Handle --delete flag
if [[ "$1" == "--delete" || "$1" == "-d" ]]; then
    echo -e "${YELLOW}🗑️  Deleting PostgreSQL deployment from namespace '$NAMESPACE'...${NC}"
    kubectl delete deployment postgres -n "$NAMESPACE" --ignore-not-found
    kubectl delete service postgres-service -n "$NAMESPACE" --ignore-not-found
    kubectl delete pvc postgres-data -n "$NAMESPACE" --ignore-not-found
    kubectl delete secret postgres-credentials -n "$NAMESPACE" --ignore-not-found
    echo -e "${GREEN}✅ PostgreSQL resources deleted.${NC}"
    exit 0
fi

print_header "PostgreSQL Deployment for Testing"
echo ""
echo -e "  ${YELLOW}⚠️  This is for testing/development only.${NC}"
echo -e "  ${YELLOW}   For production, use a PostgreSQL operator.${NC}"
echo ""

# Ensure namespace exists
if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    echo -e "${BLUE}📦 Creating namespace '$NAMESPACE'...${NC}"
    kubectl create namespace "$NAMESPACE"
fi

echo -e "${BLUE}🔧 Deploying PostgreSQL to namespace '$NAMESPACE'...${NC}"
echo ""

# Create secret
kubectl create secret generic postgres-credentials \
    --from-literal=POSTGRES_USER="$POSTGRES_USER" \
    --from-literal=POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
    --from-literal=POSTGRES_DB="$POSTGRES_DB" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml | kubectl apply -f -

# Apply all resources using heredoc
kubectl apply -n "$NAMESPACE" -f - <<EOF
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
  labels:
    app: postgres
    app.kubernetes.io/name: postgres
    app.kubernetes.io/component: database
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: ${STORAGE_SIZE}
---
apiVersion: v1
kind: Service
metadata:
  name: postgres-service
  labels:
    app: postgres
    app.kubernetes.io/name: postgres
    app.kubernetes.io/component: database
spec:
  selector:
    app: postgres
  ports:
  - protocol: TCP
    port: 5432
    targetPort: 5432
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  labels:
    app: postgres
    app.kubernetes.io/name: postgres
    app.kubernetes.io/component: database
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
      - name: postgres
        image: postgres:15-alpine
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: postgres-credentials
              key: POSTGRES_USER
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-credentials
              key: POSTGRES_PASSWORD
        - name: POSTGRES_DB
          valueFrom:
            secretKeyRef:
              name: postgres-credentials
              key: POSTGRES_DB
        - name: PGDATA
          value: /var/lib/postgresql/data/pgdata
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
        volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
        readinessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - pg_isready -U "\$POSTGRES_USER" -d "\$POSTGRES_DB"
          initialDelaySeconds: 10
          periodSeconds: 5
        livenessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - pg_isready -U "\$POSTGRES_USER" -d "\$POSTGRES_DB"
          initialDelaySeconds: 30
          periodSeconds: 10
      volumes:
      - name: postgres-data
        persistentVolumeClaim:
          claimName: postgres-data
EOF

echo ""
echo -e "${BLUE}⏳ Waiting for PostgreSQL to be ready...${NC}"

if kubectl wait --for=condition=available deployment/postgres -n "$NAMESPACE" --timeout=180s; then
    # Build connection info
    DATABASE_HOST="postgres-service.${NAMESPACE}.svc.cluster.local"
    DATABASE_PORT="5432"
    DATABASE_URL="postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${DATABASE_HOST}:${DATABASE_PORT}/${POSTGRES_DB}?sslmode=disable"

    echo ""
    print_header "PostgreSQL Deployment Successful"
    echo ""
    echo -e "  ${GREEN}Connection Details:${NC}"
    echo -e "  ─────────────────────────────────────────────────────────────────────"
    echo -e "  ${YELLOW}Host:${NC}     ${DATABASE_HOST}"
    echo -e "  ${YELLOW}Port:${NC}     ${DATABASE_PORT}"
    echo -e "  ${YELLOW}Database:${NC} ${POSTGRES_DB}"
    echo -e "  ${YELLOW}User:${NC}     ${POSTGRES_USER}"
    echo -e "  ${YELLOW}Password:${NC} ${POSTGRES_PASSWORD}"
    echo ""
    echo -e "  ${GREEN}DATABASE_URL:${NC}"
    echo -e "  ${DATABASE_URL}"
    echo ""
    print_header "Configure maas-api"
    echo ""
    echo -e "  Create a secret in the maas-api namespace:"
    echo ""
    echo -e "  ${YELLOW}kubectl create secret generic database-config \\\\${NC}"
    echo -e "  ${YELLOW}  --from-literal=DATABASE_URL=\"${DATABASE_URL}\" \\\\${NC}"
    echo -e "  ${YELLOW}  --namespace=<maas-api-namespace>${NC}"
    echo ""
    echo -e "  Then restart maas-api to pick up the new configuration:"
    echo ""
    echo -e "  ${YELLOW}kubectl rollout restart deployment/maas-api -n <maas-api-namespace>${NC}"
    echo ""
    print_header "Quick Test (Port Forward)"
    echo ""
    echo -e "  ${YELLOW}kubectl port-forward -n ${NAMESPACE} svc/postgres-service 5432:5432 &${NC}"
    echo -e "  ${YELLOW}psql \"${DATABASE_URL//${DATABASE_HOST}/localhost}\"${NC}"
    echo ""

    # Output machine-readable format for scripting
    echo "# Machine-readable output (can be sourced in scripts):"
    echo "export POSTGRES_HOST=\"${DATABASE_HOST}\""
    echo "export POSTGRES_PORT=\"${DATABASE_PORT}\""
    echo "export POSTGRES_USER=\"${POSTGRES_USER}\""
    echo "export POSTGRES_PASSWORD=\"${POSTGRES_PASSWORD}\""
    echo "export POSTGRES_DB=\"${POSTGRES_DB}\""
    echo "export DATABASE_URL=\"${DATABASE_URL}\""
else
    echo ""
    echo -e "${RED}❌ PostgreSQL deployment failed or timed out.${NC}"
    echo -e "${RED}   Check status: kubectl describe deployment/postgres -n ${NAMESPACE}${NC}"
    echo -e "${RED}   Check logs:   kubectl logs -l app=postgres -n ${NAMESPACE}${NC}"
    exit 1
fi

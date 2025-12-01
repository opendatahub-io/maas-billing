#!/bin/bash
#
# This script deploys a basic Redis instance (Deployment + Service) 
# for testing Limitador persistence.
#
# Namespace selection:
#   - Use NAMESPACE environment variable if set
#   - Default: redis-limitador (created if it doesn't exist)
#
# WARNING: This is a basic, non-production Redis instance intended
# for local development and validation only. Redis persistence (RDB/AOF)
# is disabled to avoid disk write issues in ephemeral test environments.
#

set -e

# Determine target namespace:
# Use NAMESPACE env var if set, otherwise default to redis-limitador
: "${NAMESPACE:=redis-limitador}"

# Ensure namespace exists
if ! kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  echo "📦 Creating namespace '$NAMESPACE'..."
  kubectl create namespace "$NAMESPACE"
fi

echo "🔧 Deploying Redis Deployment and Service to namespace '$NAMESPACE'..."
echo ""
echo "⚠️  WARNING: This Redis instance has persistence disabled (no RDB/AOF)."
echo "   It runs in-memory only and is intended for local development and validation."
echo "   Data will be lost if the Redis pod restarts."
echo "   For production, use proper Redis with persistent volumes."
echo ""

# Create the Redis Deployment
kubectl apply -n "$NAMESPACE" -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: $NAMESPACE
  labels:
    app: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        command:
        - redis-server
        - --save ""
        - --appendonly no
        ports:
        - containerPort: 6379
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "200m"
EOF

# Create the Redis Service
kubectl apply -n "$NAMESPACE" -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: redis-service
  namespace: $NAMESPACE
  labels:
    app: redis
spec:
  selector:
    app: redis
  ports:
  - protocol: TCP
    port: 6379
    targetPort: 6379
EOF

echo "⏳ Waiting for Redis to be ready..."
kubectl wait --for=condition=available deployment/redis -n "$NAMESPACE" --timeout=120s

if [ $? -eq 0 ]; then
  REDIS_URL="redis://redis-service.$NAMESPACE.svc:6379"
  echo ""
  echo "✅ Redis deployment successful."
  echo ""
  echo "📝 Next steps to configure Limitador:"
  echo ""
  echo "1. Create a Secret with the Redis URL (replace <target-namespace> with your Limitador's namespace):"
  echo ""
  echo "   kubectl create secret generic redis-config \\"
  echo "     --from-literal=URL=$REDIS_URL \\"
  echo "     --namespace=<target-namespace>"
  echo ""
  echo "2. Add this configuration to your Limitador CR:"
  echo ""
  echo "   spec:"
  echo "     storage:"
  echo "       redis:"
  echo "         configSecretRef:"
  echo "           name: redis-config"
  echo ""
  echo "💡 To edit your Limitador CR:"
  echo "   kubectl edit limitador <your-limitador-instance-name> -n <target-namespace>"
  echo ""
  echo "📚 For more information, see: docs/content/advanced-administration/limitador-persistence.md"
else
  echo "❌ Redis deployment failed or timed out."
  echo "Check the deployment status: kubectl describe deployment/redis -n $NAMESPACE"
  exit 1
fi


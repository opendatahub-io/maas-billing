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
# for local development and validation only.
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
  echo ""
  echo "✅ Redis deployment successful."
  echo ""
  echo "📝 Use this URL in your Limitador CR 'spec.storage.redis.config.url':"
  echo ""
  echo "   redis://redis-service.$NAMESPACE.svc:6379"
  echo ""
  echo "💡 To edit your Limitador CR:"
  echo "   kubectl edit limitador <your-limitador-instance-name>"
  echo ""
  echo "📚 For more information, see: docs/content/advanced-administration/limitador-persistence.md"
else
  echo "❌ Redis deployment failed or timed out."
  echo "Check the deployment status: kubectl describe deployment/redis -n $NAMESPACE"
  exit 1
fi


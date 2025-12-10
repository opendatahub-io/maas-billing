# Deployment Examples

These examples show how to configure maas-api with different storage backends.

## Storage Modes

| Mode | Description | HPA Support | Persistence |
|------|-------------|-------------|-------------|
| **Default** | In-memory SQLite | ❌ | ❌ Data lost on restart |
| **sqlite-persistent** | SQLite with PVC | ❌ Single replica | ✅ Survives restarts |
| **postgresql** | External PostgreSQL | ✅ Multiple replicas | ✅ Full persistence |

## Quick Start (Demo/Testing)

Deploy with no configuration - uses in-memory storage

⚠️ **Warning**: Data is lost when the pod restarts.

## SQLite Persistent Storage

For demos or single-replica deployments where you want data to persist:

```bash
kustomize build deployment/examples/sqlite-persistent | kubectl apply -f -
```

This creates:
- A 1Gi PersistentVolumeClaim
- A secret with `DATABASE_URL=sqlite:///data/maas-api.db`

## PostgreSQL (Production/HA)

For production with high availability (HPA support):

1. **Deploy PostgreSQL** using an operator (recommended: [CloudNativePG](https://cloudnative-pg.io/documentation/current/quickstart/)) or managed service
2. **Create the database secret** (use the example as a template):
   ```bash
   cp deployment/examples/postgresql/secret_example.yaml my-secret.yaml
   # Edit my-secret.yaml with your actual DATABASE_URL
   kubectl apply -f my-secret.yaml -n maas-api
   ```
3. **Deploy maas-api**:
   ```bash
   kustomize build deployment/examples/postgresql | kubectl apply -f -
   ```

### Setting Up PostgreSQL with CloudNativePG

CloudNativePG is a CNCF project that simplifies PostgreSQL deployment on Kubernetes:

```bash
# Install the operator
kubectl apply --server-side -f \
  https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.25/releases/cnpg-1.25.1.yaml

# Create a PostgreSQL cluster
kubectl apply -f - <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: maas-postgres
  namespace: maas-api
spec:
  instances: 1
  storage:
    size: 1Gi
EOF

# Wait for the cluster to be ready
kubectl wait --for=condition=Ready cluster/maas-postgres -n maas-api --timeout=300s
```

Then create the database secret using the auto-generated credentials:
```bash
# Get the generated password
PGPASSWORD=$(kubectl get secret maas-postgres-app -n maas-api -o jsonpath='{.data.password}' | base64 -d)

# Create the database-config secret for maas-api
kubectl create secret generic database-config \
  --from-literal=DATABASE_URL="postgresql://app:${PGPASSWORD}@maas-postgres-rw:5432/app?sslmode=require" \
  -n maas-api
```

## Custom Configuration

To use a custom database URL without the examples:

```bash
# Create the secret directly
kubectl create secret generic database-config \
  --from-literal=DATABASE_URL="postgresql://user:pass@host:5432/db" \
  -n maas-api
```


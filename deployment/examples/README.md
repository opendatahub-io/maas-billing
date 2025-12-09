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

1. **Deploy PostgreSQL** using an operator or managed service
2. **Edit the secret** with your connection string:
   ```bash
   cp deployment/examples/postgresql/secret.yaml my-secret.yaml
   # Edit my-secret.yaml with your actual DATABASE_URL
   kubectl apply -f my-secret.yaml -n maas-api
   ```
3. **Deploy maas-api**:

### Testing PostgreSQL Setup

For testing, use the helper script to deploy a simple PostgreSQL:

```bash
./deployment/scripts/setup-postgres.sh
# Follow the output instructions to create the secret
```

## Custom Configuration

To use a custom database URL without the examples:

```bash
# Create the secret directly
kubectl create secret generic database-config \
  --from-literal=DATABASE_URL="postgresql://user:pass@host:5432/db" \
  -n maas-api
```


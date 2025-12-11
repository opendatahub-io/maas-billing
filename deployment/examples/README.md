# Deployment Examples

These examples show how to configure maas-api with different storage backends.

## Storage Modes

| Mode | Description | HPA Support | Persistence |
|------|-------------|-------------|-------------|
| **Default (base)** | In-memory SQLite | ❌ | ❌ Data lost on restart |
| **sqlite-persistent** | SQLite with PVC | ❌ Single replica | ✅ Survives restarts |
| **postgresql** | External PostgreSQL | ✅ Multiple replicas | ✅ Full persistence |

## Quick Start (Demo/Testing)

Deploy with no configuration - uses in-memory storage

⚠️ **Warning**: Data is lost when the pod restarts. Use one of the overlay examples for persistent storage.

## SQLite Persistent Storage

For demos or single-replica deployments where you want data to persist:

```bash
kustomize build deployment/examples/sqlite-persistent | kubectl apply -f -
```

This creates:
- A 1Gi PersistentVolumeClaim
- A secret with `DATABASE_URL=sqlite:///data/maas-api.db`
- Volume mounts for the database file

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

CloudNativePG is a CNCF project that simplifies PostgreSQL deployment on Kubernetes.

#### OpenShift (Recommended)

Install the **Red Hat certified operator** from OperatorHub:

1. In the OpenShift Console, go to **Operators → OperatorHub**
2. Search for **CloudNativePG**
3. Install the operator (select the `openshift-operators` namespace)

Or via CLI:
```bash
oc apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: cloudnative-pg
  namespace: openshift-operators
spec:
  channel: stable-v1
  name: cloudnative-pg
  source: certified-operators
  sourceNamespace: openshift-marketplace
EOF

# If install plan requires approval:
oc get installplan -n openshift-operators | grep cloudnative
oc patch installplan <plan-name> -n openshift-operators --type merge -p '{"spec":{"approved":true}}'
```

#### Vanilla Kubernetes

```bash
kubectl apply --server-side -f \
  https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.25/releases/cnpg-1.25.1.yaml
```

#### Create PostgreSQL Cluster

```bash
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
PGPASSWORD=$(kubectl get secret maas-postgres-app -n maas-api -o jsonpath='{.data.password}' | base64 -d)

kubectl create secret generic database-config \
  --from-literal=DATABASE_URL="postgresql://app:${PGPASSWORD}@maas-postgres-rw:5432/app?sslmode=require" \
  -n maas-api
```

### PostgreSQL Connection Pool Configuration

For high-availability scenarios with multiple replicas or connection poolers (like PgBouncer), you can tune the connection pool settings via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_MAX_OPEN_CONNS` | 25 | Maximum number of open connections to the database |
| `DB_MAX_IDLE_CONNS` | 5 | Maximum number of idle connections in the pool |
| `DB_CONN_MAX_LIFETIME_SECONDS` | 300 | Maximum time (seconds) a connection can be reused |

Example patch to customize connection pool settings:

```yaml
# connection-pool-patch.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: maas-api
spec:
  template:
    spec:
      containers:
      - name: maas-api
        env:
        - name: DB_MAX_OPEN_CONNS
          value: "50"
        - name: DB_MAX_IDLE_CONNS
          value: "10"
        - name: DB_CONN_MAX_LIFETIME_SECONDS
          value: "600"
```

**Note**: These settings only apply to PostgreSQL. SQLite always uses a single connection to avoid database locking issues.


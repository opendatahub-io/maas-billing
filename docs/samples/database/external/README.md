# External Database Configuration

This is an example guide for setting up Model as a Service with the use of an external database, which is recommended for production deployments.

> [!WARNING]
> **Example Purposes Only**
> This configuration is intended as an example for testing and development. Do not use this in a production environment.

## Configuration

### Command-Line Flags

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--storage` | `STORAGE_MODE` | Storage mode: `in-memory`, `disk`, or `external` |
| `--db-connection-url` | `DB_CONNECTION_URL` | Database URL (required for `external` mode) |

### Connection Pool Settings (External Mode Only)

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_MAX_OPEN_CONNS` | 25 | Maximum number of open connections to the database |
| `DB_MAX_IDLE_CONNS` | 5 | Maximum number of idle connections in the pool |
| `DB_CONN_MAX_LIFETIME_SECONDS` | 300 | Maximum time (seconds) a connection can be reused |

## Setting Up External Database

### Option 1: CloudNativePG Operator

CloudNativePG is a CNCF project that simplifies PostgreSQL deployment on Kubernetes.

#### OpenShift Installation

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

#### Vanilla Kubernetes Installation

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

### Option 2: Managed Database Services

You can also use managed PostgreSQL services:

- **AWS RDS** - Amazon Relational Database Service
- **Google Cloud SQL** - Google Cloud managed PostgreSQL
- **Azure Database for PostgreSQL** - Azure managed PostgreSQL
- **Other providers** - Any PostgreSQL-compatible service

## Configuring maas-api for External Database

### Step 1: Create the Database Secret

Create a secret with your database connection URL:

```yaml
# database-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: database-config
  namespace: maas-api
  labels:
    app.kubernetes.io/name: maas-api
    app.kubernetes.io/component: config
type: Opaque
stringData:
  # Format: postgresql://USER:PASSWORD@HOST:PORT/DATABASE?sslmode=MODE
  DB_CONNECTION_URL: "postgresql://app:YOUR_PASSWORD@maas-postgres-rw:5432/app?sslmode=require"
```

For CloudNativePG, you can extract the auto-generated credentials:

```bash
PGPASSWORD=$(kubectl get secret maas-postgres-app -n maas-api -o jsonpath='{.data.password}' | base64 -d)

kubectl create secret generic database-config \
  --from-literal=DB_CONNECTION_URL="postgresql://app:${PGPASSWORD}@maas-postgres-rw:5432/app?sslmode=require" \
  -n maas-api
```

### Step 2: Configure the Deployment

Add the storage mode and database URL to your maas-api deployment:

```yaml
# deployment-patch.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: maas-api
spec:
  template:
    spec:
      containers:
      - name: maas-api
        command:
        - ./maas-api
        args:
        - --storage=external
        env:
        - name: DB_CONNECTION_URL
          valueFrom:
            secretKeyRef:
              name: database-config
              key: DB_CONNECTION_URL
```

> [!NOTE]
> The `command` must be specified explicitly when using `args`, otherwise the args will replace the container's default command.

## Example: Complete External Database Kustomization

Here's a complete example showing how to create a kustomization for external database mode:

```yaml
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../base/maas-api
# Note: database-secret.yaml should be created separately with your actual credentials

patches:
- patch: |-
    - op: add
      path: /spec/template/spec/containers/0/command
      value:
        - ./maas-api
    - op: add
      path: /spec/template/spec/containers/0/args
      value:
        - --storage=external
    - op: add
      path: /spec/template/spec/containers/0/env/-
      value:
        name: DB_CONNECTION_URL
        valueFrom:
          secretKeyRef:
            name: database-config
            key: DB_CONNECTION_URL
  target:
    kind: Deployment
    name: maas-api
```

## Connection URL Format
The database connection URL follows the standard PostgreSQL URL format:

```
postgresql://USER:PASSWORD@HOST:PORT/DATABASE?sslmode=MODE
```

### Examples

| Environment | Example URL |
|-------------|-------------|
| CloudNativePG (in-cluster) | `postgresql://app:secret@maas-postgres-rw:5432/app?sslmode=require` |
| AWS RDS | `postgresql://username:password@mydb.123456.us-east-1.rds.amazonaws.com:5432/maas?sslmode=require` |
| Local development | `postgresql://postgres:postgres@localhost:5432/maas_dev?sslmode=disable` |

## Troubleshooting

### Connection Errors

If maas-api fails to connect to the database:

1. **Verify the secret exists:**
   ```bash
   kubectl get secret database-config -n maas-api -o yaml
   ```

2. **Check the connection URL format:**
   ```bash
   kubectl get secret database-config -n maas-api -o jsonpath='{.data.DB_CONNECTION_URL}' | base64 -d
   ```

3. **Test connectivity from the pod:**
   ```bash
   kubectl exec -it deployment/maas-api -n maas-api -- nc -zv maas-postgres-rw 5432
   ```

4. **Check maas-api logs:**
   ```bash
   kubectl logs deployment/maas-api -n maas-api
   ```

### Unsupported Database

Currently, only PostgreSQL is supported for external databases

```
Error: unsupported external database URL: "mysql://..."
Currently supported: postgresql://
```

# Grafana and Prometheus Setup for MAAS Observability

Instructions for setting up Grafana and Prometheus monitoring for MaaS on OpenShift.

## Overview

This setup provides:
- OpenShift user workload monitoring (Prometheus)
- Grafana instance for monitoring dashboards  
- Two Prometheus datasources (cluster and user workload monitoring)
- Secure HTTPS access via OpenShift routes
- Proper RBAC for accessing OpenShift monitoring stack

## Installation Steps

### Step 1: Install Grafana Operator and Enable User Workload Monitoring

1. **Install Grafana Operator** (Manual via OpenShift Console):
   - Log into the OpenShift Console
   - Navigate to **Operators** → **OperatorHub** 
   - Search for "Grafana Operator"
   - Click **Install** and follow the installation wizard
   - Install to **openshift-operators** namespace (default)

2. **Run the automated install script**:
   ```bash
   ./install-prometheus-grafana.sh
   ```
   
   This script will:
   - Detect you're on OpenShift/ROSA
   - Enable user workload monitoring
   - Configure cluster monitoring appropriately

### Step 2: Deploy Grafana and Observability Stack

Deploy the complete observability stack:

```bash
# Apply complete observability stack (includes Grafana, datasources, and RBAC)
kubectl apply -k kustomize/observability/

# Apply routes for external access
kubectl apply -f grafana-route.yaml
```

### Step 3: Verify Deployment

Check that all components are running:

```bash
# Check user workload monitoring is enabled
kubectl get pods -n openshift-user-workload-monitoring

# Check Grafana namespace and pods
kubectl get namespace maas-observability
kubectl get pods -n maas-observability

# Check Grafana service and routes
kubectl get svc -n maas-observability
kubectl get routes -n maas-observability

# Check datasource status
kubectl get grafanadatasources -n maas-observability
```

Expected output for routes:
```
NAME            HOST/PORT                                          PATH   SERVICES          PORT      TERMINATION   WILDCARD
grafana-route   grafana.apps.maas2.octo-emerging.redhataicoe.com          grafana-service   grafana   edge          None
```

### Step 4: Access Grafana

**URL**: https://grafana.apps.<your-cluster-domain>

> Demo creds, change them :)

**Login Credentials**:
- Username: `root`
- Password: `secret`

## Configuration Details

### Grafana Configuration

The Grafana instance is configured with:
- **Anonymous access**: Enabled for read-only access
- **Admin user**: `root` with password `secret`
- **TLS termination**: Edge termination for secure HTTPS
- **Route**: Accessible via OpenShift router

### Prometheus Datasources

The setup includes two Prometheus datasources:

1. **prometheus** (Cluster Monitoring):
   - **URL**: `https://thanos-querier.openshift-monitoring.svc.cluster.local:9091`
   - **Authentication**: Bearer token authentication
   - **Metrics**: Cluster-wide monitoring metrics
   - **Default**: No

2. **user-workload-prometheus** (User Workload Monitoring):
   - **URL**: `https://prometheus-user-workload.openshift-user-workload-monitoring.svc.cluster.local:9091`
   - **Authentication**: Bearer token authentication
   - **Metrics**: Application/user workload metrics
   - **Default**: No

### Network Configuration

- **Namespace**: `maas-observability`
- **Service**: `grafana-service` on port 3000
- **Route**: HTTPS via OpenShift router
- **Domain**: `apps.<your-cluster-domain>`

## Monitoring Inference Services

### Available Metrics

With the user workload Prometheus datasource, you can monitor:
- Inference service request rates
- Response times and latency
- Error rates
- Token usage metrics (if Kuadrant token rate limiting is enabled)
- Resource utilization (CPU, memory)

### Creating Dashboards

1. **Log into Grafana**
2. **Import existing dashboards**:
   - Navigate to **+** → **Import**
   - Upload dashboard JSON file or paste JSON content
   - When prompted for datasource mapping, select:
     - `user-workload-prometheus` for application metrics
     - `prometheus` for cluster metrics
   - Click **Import**

3. **Create custom dashboards**:
   - Navigate to **+** → **Dashboard**
   - Add panels with PromQL queries such as:
   ```promql
   # Request rate
   rate(http_requests_total[5m])
   
   # Response time
   histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
   
   # Error rate
   rate(http_requests_total{code!~"2.."}[5m])
   
   # Inference service metrics (if ServiceMonitors are configured)
   rate(inference_requests_total[5m])
   inference_request_duration_seconds
   ```

### Dashboard Import Issues

**"DS_PROMETHEUS not found" Error**:
- This occurs when importing dashboards that reference a datasource by a different name
- **Solution**: During import, map the datasource variable to your actual datasource:
  - `DS_PROMETHEUS` → `user-workload-prometheus` (for application metrics)
  - `DS_PROMETHEUS` → `prometheus` (for cluster metrics)

**Default Datasource**:
- `user-workload-prometheus` is configured as the default datasource
- New panels will automatically use this datasource unless specified otherwise

## File Structure

```
deployment/kuadrant-openshift/
├── install-prometheus-grafana.sh             # Automated install script for ROSA/OpenShift
├── grafana-route.yaml                        # OpenShift route for Grafana external access
├── kustomize/
│   ├── observability/
│   │   ├── grafana-deployment.yaml           # Complete Grafana deployment manifest
│   │   └── kustomization.yaml                # Main observability kustomization
│   └── prometheus/
│       ├── grafana-rbac.yaml                 # Service account and basic RBAC
│       ├── openshift-monitoring-rbac.yaml    # OpenShift cluster monitoring RBAC
│       ├── prometheus-datasource.yaml        # Main cluster Prometheus datasource
│       ├── user-workload-datasource.yaml     # User workload Prometheus datasource
│       ├── kuadrant-servicemonitors.yaml     # ServiceMonitors for Kuadrant
│       └── kustomization.yaml                # Prometheus components kustomization
└── README-observability.md                   # This documentation
```

## Quick Start Summary

For a new deployment, run these commands in order:

```bash
# 1. Install Grafana Operator via OpenShift Console
# 2. Enable monitoring and deploy Grafana
./install-prometheus-grafana.sh

# 3. Deploy complete observability stack
kubectl apply -k kustomize/observability/
kubectl apply -f grafana-route.yaml

# 4. Verify deployment
kubectl get pods -n maas-observability
kubectl get grafanadatasources -n maas-observability

# 5. Access Grafana
echo "https://grafana.apps.<your-cluster-domain>"
echo "Username: root | Password: secret"
```


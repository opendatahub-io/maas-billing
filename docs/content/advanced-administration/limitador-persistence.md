# Persisting Limitador Metric Counts

By default, Limitador stores its rate-limiting counters in memory. This provides high performance but has a significant drawback: if a Limitador pod restarts, scales down, or is rescheduled, all hit counts are lost.

For persistent, production-ready rate limiting where counts are maintained across pod lifecycles, you must configure Limitador to use an external Redis backend.

!!! warning
    **Production Considerations**: The basic Redis setup script provided in this document is intended for local development and validation only. For production deployments, follow the official Red Hat documentation for proper Redis configuration and high availability.

---

## Table of Contents

1. [Requirements for Persistent Counts](#requirements-for-persistent-counts)
2. [Example Limitador CR Configuration](#example-limitador-cr-configuration)
3. [Local Validation Script](#local-validation-script-basic-redis)
4. [How to Validate Persistence](#how-to-validate-persistence)
5. [Related Documentation](#related-documentation)

---

## Requirements for Persistent Counts

To enable persistence, two conditions must be met:

1. **A Running Redis Instance**: A Redis instance must be deployed and network-accessible from within the Kubernetes cluster.

2. **Limitador Custom Resource (CR) Configuration**: The Limitador CR that manages your deployment must be updated to point to the running Redis instance by specifying the storage configuration in its spec.

---

## Example Limitador CR Configuration

You must edit your Limitador CR (`kubectl edit limitador <your-instance-name>`) to include the storage block. The URL should point to the internal Kubernetes service for your Redis deployment.

```yaml
apiVersion: limitador.kuadrant.io/v1alpha1
kind: Limitador
metadata:
  name: my-limitador-instance
spec:
  # ... other spec fields
  storage:
    redis:
      config:
        # This URL must point to your internal Redis service
        url: "redis://<redis-service-name>.<namespace>.svc:6379"
```

!!! tip
    **Internal Service URL Format**: The Redis URL must use the Kubernetes service DNS format: `redis://<service-name>.<namespace>.svc:<port>`. For example, with the default `redis-limitador` namespace: `redis://redis-service.redis-limitador.svc:6379`

For detailed, official instructions, refer to the Red Hat documentation:

- [Red Hat Connectivity Link - Configure Redis](https://docs.redhat.com/en/documentation/red_hat_connectivity_link/1.1/html/installing_connectivity_link_on_openshift/configure-redis_connectivity-link)

---

## Local Validation Script (Basic Redis)

A basic Redis setup script is provided for local development and validation. This script deploys a non-production Redis instance.

**Script Location:** [`deployment/scripts/setup-redis.sh`](https://github.com/opendatahub-io/maas-billing/blob/main/deployment/scripts/setup-redis.sh)

### Namespace Selection

The script uses a simple namespace selection logic:

- **`NAMESPACE` environment variable** (if set)
- **Default: `redis-limitador`** (created automatically if it doesn't exist)

This opinionated default simplifies troubleshooting and ensures consistent deployments.

### Usage

```bash
# Make the script executable
chmod +x deployment/scripts/setup-redis.sh

# Run with default namespace (redis-limitador)
./deployment/scripts/setup-redis.sh

# Or override with environment variable
NAMESPACE=my-namespace ./deployment/scripts/setup-redis.sh
```

The script will:

- Create the namespace if it doesn't exist (for default `redis-limitador` namespace)
- Deploy a Redis Deployment and Service
- Wait for Redis to be ready
- Output the Redis URL for use in your Limitador CR configuration

!!! note
    **Single Source of Truth**: The script content is maintained only in `deployment/scripts/setup-redis.sh`. Any updates to the script are automatically reflected when users download and run it.

---

## How to Validate Persistence

1. **Run the script**: `./deployment/scripts/setup-redis.sh`

   This will deploy Redis to the `redis-limitador` namespace by default (or use your `NAMESPACE` env var).

2. **Copy the output URL** (e.g., `redis://redis-service.redis-limitador.svc:6379`).

3. **Edit your Limitador CR** and add the storage block pointing to that URL:

   ```bash
   kubectl edit limitador <your-limitador-instance-name>
   ```

4. **Send traffic** against a rate-limited route until you have a non-zero hit count.

   You can verify metrics in Prometheus:

   ```bash
   # Port-forward to Prometheus (adjust namespace as needed)
   kubectl port-forward -n monitoring svc/prometheus-k8s 9090:9091

   # Query for authorized_hits metric
   # Open http://localhost:9090 and search for: authorized_hits
   ```

5. **Find your Limitador pod**:

   ```bash
   kubectl get pods -l app=limitador
   ```

6. **Delete the pod** to force a restart:

   ```bash
   kubectl delete pod <limitador-pod-name>
   ```

7. **Wait for the new pod** to become Running:

   ```bash
   kubectl get pods -l app=limitador -w
   ```

8. **Send another request** to the same route. You will see that the metric count continues from its previous value instead of resetting to 1.

---

## Related Documentation

- [Tier Overview](../configuration-and-management/tier-overview.md) - Overview of tier management for access control, rate limits, and quotas
- [Tier Configuration](../configuration-and-management/tier-configuration.md) - Step-by-step guide for configuring subscription tiers
- [Red Hat Connectivity Link - Configure Redis](https://docs.redhat.com/en/documentation/red_hat_connectivity_link/1.1/html/installing_connectivity_link_on_openshift/configure-redis_connectivity-link) - Official Red Hat documentation for production Redis setup

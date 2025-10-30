# Observability

This document covers the observability stack for the MaaS Platform, including metrics collection, monitoring, and visualization.

## Overview

As part of Dev Preview MaaS Platform includes a basic observability stack that provides insights into system performance, usage patterns, and operational health. The observability stack consists of:

!!! note
    The observability stack will be enhanced in the future.

- **Limitador**: Rate limiting service that exposes metrics
- **Prometheus**: Metrics collection and storage
- **Grafana**: Metrics visualization and dashboards
- **Future**: Migration to Perses for enhanced dashboard management

## Metrics Collection

### Limitador Metrics

Limitador exposes several key metrics that are collected through a ServiceMonitor by Prometheus:

#### Rate Limiting Metrics

- `limitador_ratelimit_requests_total`: Total number of rate limit requests
- `limitador_ratelimit_allowed_total`: Number of requests allowed
- `limitador_ratelimit_denied_total`: Number of requests denied
- `limitador_ratelimit_errors_total`: Number of rate limiting errors

#### Performance Metrics

- `limitador_ratelimit_duration_seconds`: Duration of rate limit checks
- `limitador_ratelimit_active_connections`: Number of active connections
- `limitador_ratelimit_cache_hits_total`: Cache hit rate
- `limitador_ratelimit_cache_misses_total`: Cache miss rate

#### Tier-Based Metrics

- `limitador_ratelimit_tier_requests_total`: Requests per tier
- `limitador_ratelimit_tier_allowed_total`: Allowed requests per tier
- `limitador_ratelimit_tier_denied_total`: Denied requests per tier

### ServiceMonitor Configuration

For automatic discovery of services, use ServiceMonitor resources:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: limitador-monitor
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: limitador
  endpoints:
  - port: metrics
    interval: 10s
    path: /metrics
```

## Grafana Dashboards

### MaaS Platform Overview Dashboard

We are providing a basic dashboard for the MaaS Platform that can be used to get a quick
overview of the system. Its definition can be found and imported from the following 
link:
[maas-token-metrics-dashboard.json](https://github.com/opendatahub-io/maas-billing/blob/main/docs/samples/dashboards/maas-token-metrics-dashboard.json)

See more detailed description of the Grafana Dashboard in [its README of the 
repository](https://github.com/opendatahub-io/maas-billing/tree/main/docs/samples/dashboards).

# Storage Configuration

This guide explains the storage modes available for maas-api and how to configure them for different deployment scenarios.

!!! note
    **For External Database Setup Examples**: If you need step-by-step instructions for setting up an external PostgreSQL database, see the [external database samples](https://github.com/opendatahub-io/maas-billing/tree/main/docs/samples/database/external).

---

## Table of Contents

1. [Overview](#overview)
2. [Storage Modes](#storage-modes)
3. [Configuration Reference](#configuration-reference)
4. [Choosing the Right Storage Mode](#choosing-the-right-storage-mode)
5. [Related Documentation](#related-documentation)

---

## Overview

maas-api stores API key metadata and other persistent data. The storage backend can be configured based on your deployment requirements:

- **Development/Testing**: Use in-memory storage for simplicity
- **Single-replica demos**: Use disk storage for persistence without external dependencies
- **Production with High Availability (HA)**: Use external PostgreSQL database

---

## Storage Modes

maas-api supports three storage modes, controlled by the `--storage` flag or `STORAGE_MODE` environment variable:

| Mode | Flag Value | Description | Persistence | HA Support |
|------|------------|-------------|-------------|------------|
| **In-memory** | `in-memory` | Ephemeral storage in application memory | ❌ Data lost on restart | ❌ Single replica only |
| **Disk** | `disk` | SQLite database stored on local filesystem | ✅ Survives restarts | ❌ Single replica only |
| **External** | `external` | External PostgreSQL database | ✅ Full persistence | ✅ Multiple replicas |

### In-Memory Mode (Default)

Data is stored only in application memory. This is the default mode and requires no configuration.

**Use cases:**

- Local development
- Quick testing
- Environments where persistence is not required

**Limitations:**

- All data is lost when the pod restarts
- Cannot scale to multiple replicas

### Disk Mode

Data is persisted to a SQLite database file on the local filesystem.

**Use cases:**

- Single-replica deployments
- Demos and proof-of-concept deployments
- Environments where an external database is not available

**Limitations:**

- Cannot scale to multiple replicas (each replica would have its own database)
- Requires a PersistentVolumeClaim (PVC) for data to survive pod rescheduling

### External Mode

Data is stored in an external PostgreSQL database, enabling full persistence and high availability.

**Use cases:**

- Production deployments
- High availability requirements
- Multi-replica deployments

**Requirements:**

- PostgreSQL database (version 12 or later recommended)
- Network connectivity from maas-api pods to the database

---

## Configuration Reference

Configuration can be set via command-line flags or environment variables. They are interchangeable with the following precedence (highest to lowest):

1. **Command-line flags** - override environment variables
2. **Environment variables** - used if flag not provided
3. **Default values** - used if neither is set

!!! tip
    For Kubernetes deployments, environment variables are typically easier to configure via ConfigMaps or Secrets. Command-line flags are convenient for local development.

| Flag | Environment Variable | Description | Default |
|------|---------------------|-------------|---------|
| `--storage` | `STORAGE_MODE` | Storage mode: `in-memory`, `disk`, or `external` | `in-memory` |
| `--db-connection-url` | `DB_CONNECTION_URL` | Database connection URL (required for `external` mode) | - |
| `--data-path` | `DATA_PATH` | Path to database file (for `disk` mode) | `/data/maas-api.db` |

### Connection Pool Settings (External Mode Only)

These environment variables tune the database connection pool for external mode:

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_MAX_OPEN_CONNS` | Maximum number of open connections to the database | `25` |
| `DB_MAX_IDLE_CONNS` | Maximum number of idle connections in the pool | `5` |
| `DB_CONN_MAX_LIFETIME_SECONDS` | Maximum time (seconds) a connection can be reused | `300` |

### Database Connection URL Format

For external mode, the connection URL follows the standard PostgreSQL format:

```
postgresql://USER:PASSWORD@HOST:PORT/DATABASE?sslmode=MODE
```

**Example:**

```
postgresql://app:mypassword@postgres-service:5432/maas?sslmode=require
```

---

## Choosing the Right Storage Mode

| Scenario | Recommended Mode | Notes |
|----------|-----------------|-------|
| Local development | `in-memory` | No setup required |
| CI/CD pipelines | `in-memory` | Fast, no cleanup needed |
| Single-replica demo | `disk` | Add a PVC for persistence |
| Production | `external` | High availability |

---

## Related Documentation

- [External Database Setup Examples](https://github.com/opendatahub-io/maas-billing/tree/main/docs/samples/database/external) - Step-by-step guides for setting up PostgreSQL

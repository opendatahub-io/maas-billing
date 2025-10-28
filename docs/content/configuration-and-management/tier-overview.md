# Tier Management Overview

This guide explains how to configure and manage subscription tiers for the MaaS Platform. Tiers enable differentiated service levels with varying access permissions, rate limits, and quotas.

## Overview

The tier system is driven by Kubernetes native objects and provides:

- **Group-based access control**: Users are assigned tiers based on their Kubernetes group membership
- **Namespace-scoped RBAC**: Each tier has its own namespace for permission management
- **Dynamic tier resolution**: User tiers are resolved on each request
- **Per-model authorization**: Access control is enforced at the model level
- **Hierarchical precedence**: Users with multiple group memberships get the highest tier

## Documentation Structure

This tier management documentation is organized into three sections:

1. **[Tier Overview](tier-overview.md)** (this document) - High-level overview of the tier system
2. **[Tier Configuration](tier-configuration.md)** - Step-by-step configuration guide
3. **[Tier Concepts](tier-concepts.md)** - Reference material explaining how the tier system works

## Quick Start

To get started with tier management, see the [Configuration Guide](tier-configuration.md).

For detailed information about how the tier system works internally, see the [Tier Concepts](tier-concepts.md) documentation.

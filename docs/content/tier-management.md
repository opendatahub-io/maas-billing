# Tier Management

This guide explains how to configure and manage subscription tiers for the MaaS Platform. Tiers enable differentiated service levels with varying access permissions, rate limits, and quotas.

## Overview

The tier system provides:
- **Group-based access control**: Users are assigned tiers based on their Kubernetes group membership
- **Namespace-scoped RBAC**: Each tier has its own namespace for permission management
- **Dynamic tier resolution**: User tiers are resolved on each request
- **Per-model authorization**: Access control is enforced at the model level
- **Hierarchical precedence**: Users with multiple group memberships get the highest tier

## Core Concepts

### Tier Configuration

Tiers are defined in a Kubernetes ConfigMap that maps user groups to subscription levels:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tier-to-group-mapping
  namespace: maas-api
data:
  tiers: |
    - name: free
      description: Free tier for basic users
      level: 1
      groups:
      - system:authenticated
    - name: premium
      description: Premium tier
      level: 10
      groups:
      - premium-users
    - name: enterprise
      description: Enterprise tier
      level: 20
      groups:
      - enterprise-users
```

### Tier Namespaces

Each tier gets a dedicated namespace following the pattern `<instance-name>-tier-<tier-name>`:
- `maas-default-gateway-tier-free`
- `maas-default-gateway-tier-premium`
- `maas-default-gateway-tier-enterprise`

### Tier Resolution Process

1. User authenticates with JWT token
2. Gateway extracts user groups from token
3. MaaS API resolves tier based on group membership
4. Tier information is cached for 5 minutes
5. Access control and rate limiting are applied based on tier

## Configuration Steps

### 1. Configure Tier Mapping

Create or update the tier mapping ConfigMap:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: tier-to-group-mapping
  namespace: maas-api
data:
  tiers: |
    - name: free
      description: Free tier for basic users
      level: 1
      groups:
      - system:authenticated
    - name: premium
      description: Premium tier
      level: 10
      groups:
      - premium-users
    - name: enterprise
      description: Enterprise tier
      level: 20
      groups:
      - enterprise-users
EOF
```

Restart the MaaS API to pick up the new configuration:

```bash
kubectl rollout restart deployment/maas-api -n maas-api
```

### 2. Configure RBAC

Create Role and RoleBinding to grant tier namespaces's service accounts access to the model:

```bash
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: model-post-access
  namespace: llm
rules:
  - apiGroups: ["serving.kserve.io"]
    resources: ["llminferenceservices"]
    verbs: ["post"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: model-post-access-tier-binding
  namespace: llm
subjects:
  - kind: Group
    name: system:serviceaccounts:maas-default-gateway-tier-free
    apiGroup: rbac.authorization.k8s.io
  - kind: Group
    name: system:serviceaccounts:maas-default-gateway-tier-premium
    apiGroup: rbac.authorization.k8s.io
  - kind: Group
    name: system:serviceaccounts:maas-default-gateway-tier-enterprise
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: model-post-access
  apiGroup: rbac.authorization.k8s.io
EOF
```

> **Note:** To grant access to a specific model only, modify the `resourceNames` field in the Role to include the model's name (e.g., `resourceNames: ["llama2", "gpt-4"]`). This restricts access to only the specified models instead of allowing access to all models in the namespace.


### 3. Configure Rate Limiting

Set up tier-specific rate limits by updating or creating the RateLimitPolicy:

```yaml
apiVersion: kuadrant.io/v1beta2
kind: RateLimitPolicy
metadata:
  name: model-rate-limits
  namespace: llm
spec:
  targetRef:                                          # 1
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: model-route
  limits:                                             # 2
    free:                                             # 3
      rates:
        - limit: 5                                    # 4
          window: 2m                                  # 5
      when:
        - predicate: 'auth.identity.tier == "free"'   # 6
      counters:
        - expression: auth.identity.userid            # 7
    premium:
      rates:
        - limit: 20
          window: 2m
      when:
        - predicate: 'auth.identity.tier == "premium"'
      counters:
        - expression: auth.identity.userid
    enterprise:
      rates:
        - limit: 50
          window: 2m
      when:
        - predicate: 'auth.identity.tier == "enterprise"'
      counters:
        - expression: auth.identity.userid
```

**Rate Limit Policy Configuration Explained:**

1. **Target reference** - Specifies which HTTPRoute this policy applies to
2. **Main limits section** - Container for all tier-specific rate limit definitions
3. **Tier definition** - Each tier (free, premium, enterprise) gets its own configuration block
4. **Request limit** - Maximum number of requests allowed per time window
5. **Time window** - Duration after which the request counter resets
6. **Predicate condition** - Determines when this tier's limits apply based on user authentication
7. **Counter expression** - Tracks requests per user ID for reporting purposes

### 4. Configure Token Consumption Limits

Set up token-based rate limiting:

```yaml
apiVersion: kuadrant.io/v1beta2
kind: TokenRateLimitPolicy
metadata:
  name: model-token-limits
  namespace: llm
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: model-route
  limits:
    free:
      rates:
        - limit: 100
          window: 1m
      when:
        - predicate: 'auth.identity.tier == "free"'
      counters:
        - expression: auth.identity.userid
    premium:
      rates:
        - limit: 50000
          window: 1m
      when:
        - predicate: 'auth.identity.tier == "premium"'
      counters:
        - expression: auth.identity.userid
    enterprise:
      rates:
        - limit: 100000
          window: 1m
      when:
        - predicate: 'auth.identity.tier == "enterprise"'
      counters:
        - expression: auth.identity.userid
```

## User Management

### Adding Users to Tiers

Users are automatically assigned to tiers based on their Kubernetes group membership. To add a user to a tier:

1. **Add user to the appropriate group** in your identity provider (LDAP, OIDC, etc.)
2. **Ensure the group is mapped to the desired tier** in the tier mapping ConfigMap
3. **Users will automatically get the tier** on their next token request

> [!NOTE]
> Authenticated users are automatically assigned to the `free` tier through the `system:authenticated` group.

## Monitoring and Troubleshooting

<TBD>

### Verify Tier Configuration

Test tier lookup:

```bash
kubectl exec -n maas-api deployment/maas-api -- \
  curl -X POST localhost:8080/v1/tiers/lookup \
  -H "Content-Type: application/json" \
  -d '{"groups": ["premium-users"]}'
```

Expected response: `{"tier":"premium"}`

### Check RBAC Configuration

Verify that tier namespaces can access models:

```bash
kubectl auth can-i post llminferenceservices \
  --as=system:serviceaccount:maas-default-gateway-tier-premium:test \
  -n llm
```

Expected response: "yes"

### Monitor Policy Status

Check that all policies are applied correctly:

```bash
kubectl get authpolicy -A
kubectl get ratelimitpolicy -A
kubectl get tokenratelimitpolicy -A
```

### Common Issues

1. **Tier not resolving correctly**
   - Check group membership in identity provider
   - Verify tier mapping ConfigMap
   - Restart MaaS API after configuration changes

2. **Access denied to models**
   - Verify RBAC Role and RoleBinding
   - Check tier namespace names match exactly
   - Ensure model is deployed and accessible

3. **Rate limits not working**
   - Check RateLimitPolicy and TokenRateLimitPolicy status
   - Verify tier predicates match tier names exactly
   - Restart Kuadrant operators if needed

## Advanced Configuration

### Custom Tier Levels

You can create custom tiers with specific levels:

```yaml
- name: custom-tier
  description: Custom tier with specific permissions
  level: 15
  groups:
  - custom-users
```

### Per-Model Access Control

Control access to specific models by creating model-specific RBAC:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: gpu-model-access
  namespace: llm
rules:
  - apiGroups: ["serving.kserve.io"]
    resources: ["llminferenceservices"]
    resourceNames: ["qwen3"]
    verbs: ["post"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: gpu-model-access-premium
  namespace: llm
subjects:
  - kind: Group
    name: system:serviceaccounts:maas-default-gateway-tier-premium
    apiGroup: rbac.authorization.k8s.io
  - kind: Group
    name: system:serviceaccounts:maas-default-gateway-tier-enterprise
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: gpu-model-access
  apiGroup: rbac.authorization.k8s.io
```

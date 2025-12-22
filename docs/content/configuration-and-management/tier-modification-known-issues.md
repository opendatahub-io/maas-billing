# Tier Modification Known Issues

This document describes known issues and side effects related to modifying tier definitions (ConfigMap) during active usage in the MaaS Platform Technical Preview release.

## Tier Configuration Changes During Active Usage

### Issue Description

When the `tier-to-group-mapping` ConfigMap is modified (e.g., changing groups or levels) while users are actively making requests, several side effects may occur due to caching and eventual consistency in the system.

### How Tier Resolution Works

1. **ConfigMap**: Tiers are defined in the `tier-to-group-mapping` ConfigMap.
2. **MaaS API**: Watches the ConfigMap and updates its internal state. Used for token generation.
3. **AuthPolicy (Authorino)**: Caches tier lookup results for authenticated users (default TTL: 5 minutes).
4. **Token**: Contains a Service Account identity associated with a specific tier namespace (e.g., `maas-default-gateway-tier-free`) at the time of issuance.

### Side Effects

#### 1. Propagation Delay for Group Changes

**Impact**: Medium

**Description**:

If a user's group membership changes or a tier's group definition is updated:

- The `AuthPolicy` (Authorino) caches the user's tier for 5 minutes.
- The user will continue to be rate-limited according to their *old* tier until the cache expires.
- After the cache expires, the new tier limits will apply.

**Example Scenario**:

```text
T+0s:  User added to "premium-users" group (was "free")
T+10s: ConfigMap updated in MaaS API
T+1m:  User makes request -> Authorino uses cached "free" tier (Rate Limit: 10/min)
T+5m:  Cache expires
T+6m:  User makes request -> Authorino looks up tier -> "premium" (Rate Limit: 1000/min)
```

**Workaround**:

- Wait for the cache TTL (5 minutes) for changes to fully propagate.
- Restart the Authorino pods to force immediate cache invalidation (disruptive).

#### 2. Tier Names Are Immutable

**Important**: Tier names (the `name` field in the ConfigMap) are expected to be **immutable** and should not be renamed after creation. This design ensures consistency across:

- `RateLimitPolicy` and `TokenRateLimitPolicy` definitions
- Tier namespace naming (e.g., `maas-default-gateway-tier-free`)
- Token claims and Service Account associations

**If you need to change how a tier is displayed to users**, use the `displayName` field instead. The `displayName` can be modified at any time without affecting the underlying tier configuration or policies.

**Example**:

```yaml
# Correct: Change displayName, not name
tiers:
  - name: free           # Immutable - do not change
    displayName: "Starter Plan"  # Can be changed for UI purposes
    level: 1
    groups:
      - "system:authenticated"
```

#### 3. Monitoring Inconsistency

**Impact**: Low

**Description**:

Tokens are issued with a Service Account in a tier-specific namespace (e.g., `maas-default-gateway-tier-free`). This namespace is embedded in the token claims.
If a user moves to a new tier (e.g., `premium`) but continues using a valid token issued under the old tier:

- **Enforcement**: They get the *new* tier's rate limits (after cache expiry).
- **Monitoring**: Their usage metrics in Prometheus will still be attributed to the *old* Service Account/Namespace (`maas-default-gateway-tier-free`).

**Example**:

- User upgrades to Premium.
- Token claim: `system:serviceaccount:maas-default-gateway-tier-free:user-123`
- Rate Limit enforced: Premium (correct)
- Prometheus Metric: `requests_total{namespace="maas-default-gateway-tier-free"}` (incorrect attribution)

**Workaround**:

- Users must request a new token to have their usage correctly attributed to the new tier's namespace.
- This is a monitoring reporting issue only; access control is unaffected.
- **Token Invalidation**: Tokens can be invalidated by removing the old ServiceAccount associated with them. When a user moves to a new tier, their old ServiceAccount in the previous tier namespace remains (it is not automatically deleted). Administrators can manually delete these orphaned ServiceAccounts to invalidate any remaining tokens, but this is not required for normal operation.

#### 4. Service Interruption on Tier Deletion

**Impact**: Medium

**Description**:

If a tier is deleted from the ConfigMap while users are still assigned to it (and have no other matching tier):

- The `TierLookup` endpoint will return an error (e.g., 404 or GroupNotFound).
- The `AuthPolicy` relies on this metadata.
- Requests may fail with `403 Forbidden` or `500 Internal Server Error` depending on how the failure is handled in the policy.

**Workaround**:

- Ensure users are moved to a new tier (via group changes) *before* deleting the old tier definition.

### Recommended Practices

1. **Treat Tier Names as Immutable**: Do not rename tiers after creation. Use `displayName` for UI-facing name changes.
2. **Update Policies First**: When adding new tiers, update the `RateLimitPolicy` first.
3. **Plan for Delays**: Expect a 5-minute delay for tier changes to affect active traffic.
4. **Token Refresh**: Encourage users to refresh their tokens after significant tier changes to ensure correct monitoring attribution.

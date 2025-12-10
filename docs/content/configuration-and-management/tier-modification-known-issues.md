# Tier Modification Known Issues

This document describes known issues and side effects related to modifying tier definitions (ConfigMap) during active usage in the MaaS Platform Technical Preview release.

## Tier Configuration Changes During Active Usage

### Issue Description

When the `tier-to-group-mapping` ConfigMap is modified (e.g., changing groups, levels, or renaming tiers) while users are actively making requests, several side effects may occur due to caching and eventual consistency in the system.

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

#### 2. Rate Limit Policy Mismatch (Tier Renaming)

**Impact**: High

**Description**:

If a tier is renamed (e.g., `free` -> `basic`) in the ConfigMap but the `RateLimitPolicy` or `TokenRateLimitPolicy` is not updated to match:

- The `AuthPolicy` will resolve the user to the new tier name (`basic`).
- The `RateLimitPolicy` matches on `auth.identity.tier == "free"`.
- The user's requests will **not match any rate limit rule**.
- Depending on the policy configuration, this may result in **unlimited access** or **default limits**.

**Workaround**:

- Always update the `RateLimitPolicy` / `TokenRateLimitPolicy` *immediately* after renaming a tier.
- Add the new tier to the policy *before* switching users to it, if possible.

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
- This is a reporting artifact and does not affect access control.

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

1. **Update Policies First**: When adding or renaming tiers, update the `RateLimitPolicy` first.
2. **Plan for Delays**: Expect a 5-minute delay for tier changes to affect active traffic.
3. **Token Refresh**: Encourage users to refresh their tokens after significant tier changes to ensure correct monitoring attribution.

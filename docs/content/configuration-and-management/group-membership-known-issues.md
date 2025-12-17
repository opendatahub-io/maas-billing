# Group Membership Known Issues

This document describes known issues and side effects related to removing group membership from users during active usage in the MaaS Platform Technical Preview release.

## Group Membership Changes During Active Usage

### Issue Description

When a user is removed from a group (e.g., removed from `premium-users` group) while they have active tokens or ongoing requests, several side effects may occur due to the separation between user identity and Service Account identity.

### How Group Membership Affects Access

1. **Token Request**: When a user requests a MaaS token, their group memberships are evaluated to determine their tier.
2. **Service Account Creation**: A Service Account is created in the tier-specific namespace (e.g., `maas-default-gateway-tier-premium`).
3. **Token Issuance**: The token is issued for the Service Account, not the original user.
4. **Request Authorization**: Requests are authorized based on the Service Account's identity and the tier metadata cached in the AuthPolicy.

### Side Effects

#### 1. Existing Tokens Remain Valid

**Impact**: High

**Description**:

When a user is removed from a group, their existing MaaS tokens remain valid until expiration because:

- The token is a Kubernetes Service Account token, not a user token.
- The Service Account continues to exist in the tier namespace.
- Kubernetes TokenReview validates the Service Account, not the original user's group membership.

**Example Scenario**:

```text
T+0h:   User "alice" is in "premium-users" group
T+0h:   Alice requests a token -> Gets SA token in maas-default-gateway-tier-premium namespace
T+1h:   Admin removes Alice from "premium-users" group
T+1h:   Alice's token is STILL VALID (expires at T+24h)
T+1h:   Alice can still make requests using the existing token
T+24h:  Token expires, Alice must request a new one
T+24h:  New token request -> Alice gets "free" tier (or fails if no tier matches)
```

**Workaround**:

- Revoke the user's tokens explicitly using the `DELETE /v1/tokens` endpoint.
- This deletes and recreates the user's Service Account, invalidating all existing tokens.

```bash
curl -X DELETE "${HOST}/maas-api/v1/tokens" \
  -H "Authorization: Bearer ${USER_TOKEN}"
```

Note: The user must authenticate with their own token to revoke their tokens. Administrators cannot revoke tokens on behalf of other users in the current implementation.

#### 2. Rate Limiting Continues at Old Tier

**Impact**: Medium

**Description**:

The AuthPolicy caches the tier lookup result (default TTL: 5 minutes). After a user is removed from a group:

- Requests within the cache window continue to use the old tier's rate limits.
- After cache expiry, the tier is re-evaluated based on current group membership.
- If the user still has a valid token but no longer belongs to any tier group, requests may fail.

**Example Timeline**:

```text
T+0m:   User removed from "premium-users" group
T+1m:   Request made -> Cached tier "premium" used -> Rate limit: 1000 tokens/min
T+5m:   Cache expires
T+6m:   Request made -> Tier lookup fails (no matching group) -> Request may fail with 403
```

**Workaround**:

- Wait for cache TTL (5 minutes) for rate limiting to reflect the new group membership.
- For immediate effect, restart Authorino pods (disruptive).

#### 3. Service Account Persists After Group Removal

**Impact**: Low

**Description**:

When a user is removed from a group, their Service Account in the tier namespace is not automatically deleted:

- The Service Account remains in the tier namespace.
- No new tokens can be issued for the old tier (tier lookup fails).
- Old tokens continue to work until expiration.
- This is a cleanup artifact, not a security issue (access is controlled by RBAC and rate limiting).

**Workaround**:

- Service Accounts can be manually cleaned up if needed.
- The Service Account name is derived from the username: special characters are replaced with dashes, converted to lowercase, and an 8-character hash suffix is appended (e.g., `alice-example-com-a1b2c3d4` for `alice@example.com`).
- To find the Service Account for a specific user, list and filter by the username pattern:

```bash
# List all Service Accounts in the tier namespace
kubectl get serviceaccount -n maas-default-gateway-tier-<old-tier>

# Filter by username pattern (e.g., for user "alice@example.com")
kubectl get serviceaccount -n maas-default-gateway-tier-<old-tier> | grep alice

# Delete the identified Service Account
kubectl delete serviceaccount <sa-name> -n maas-default-gateway-tier-<old-tier>
```

#### 4. User Downgrade Creates New Service Account

**Impact**: Low

**Description**:

When a user is moved to a lower tier (e.g., removed from `premium-users`, now only matching the `free` tier group, such as `system:authenticated` in the default configuration):

- A new Service Account is created in the new tier namespace (e.g., `maas-default-gateway-tier-free`).
- The old Service Account in the premium tier namespace remains.
- Old premium tokens continue to work until expiration.
- New token requests create tokens in the free tier namespace.

**Example**:

```text
Before: Alice in "premium-users" -> SA in maas-default-gateway-tier-premium
After:  Alice removed from "premium-users" (still matches "free" tier group)
        -> Old SA still exists in premium namespace
        -> New token request creates SA in maas-default-gateway-tier-free
        -> Alice now has SAs in both namespaces
```

**Workaround**:

- Revoke tokens before changing group membership to ensure clean transition.
- Delete the user's Service Account manually from the old tier namespace when they change groups.

#### 5. Monitoring Shows Split Metrics

**Impact**: Low

**Description**:

If a user has tokens from multiple tiers (before and after group change):

- Metrics are attributed to the Service Account's namespace.
- Usage appears split across tier namespaces.
- This is a reporting artifact and does not affect access control.

**Workaround**:

- Aggregate metrics by username label if available.
- Encourage users to revoke old tokens after tier changes.

### Recommended Practices

1. **Revoke Before Removing**: When removing a user from a group, revoke their tokens first to ensure immediate access termination.

2. **Communicate Changes**: Notify users before group membership changes so they can plan for re-authentication.

3. **Use Short Token Expiration**: Shorter token lifetimes reduce the window of continued access after group removal.

4. **Clean Up Service Accounts**: When a user changes groups, manually delete their Service Account from the old tier namespace to prevent orphaned resources.

### Related Documentation

- [Tier Configuration](./tier-configuration.md) - How to configure tier-to-group mappings
- [Token Management](./token-management.md) - Understanding token lifecycle and revocation

# Model Tier Access Known Issues

This document describes known issues and limitations related to model tier access changes in the MaaS Platform Technical Preview release.

## Model Tier Access Changes During Active Usage

### Issue Description

When a model is removed from a tier's access list (by updating the `alpha.maas.opendatahub.io/tiers` annotation on an `LLMInferenceService` resource) while users from that tier have active requests or sessions, several side effects may occur.

### How Model Access Removal Works

1. **Annotation Update**: The administrator updates the `alpha.maas.opendatahub.io/tiers` annotation to remove a tier from the allowed list
2. **ODH Controller Processing**: The ODH Controller detects the annotation change and updates RBAC resources
3. **RBAC Update**: The RoleBinding for the removed tier is deleted, revoking POST permissions for that tier's service accounts
4. **Access Revocation**: Users from the removed tier lose access to the model

### Side Effects

#### 1. Active Requests May Fail Mid-Execution

**Impact**: Medium

**Description**:

- Requests that are already in progress when the RBAC change takes effect may fail if they require re-authorization
- Long-running inference requests (e.g., streaming responses) may be interrupted
- The exact behavior depends on when the authorization check occurs in the request lifecycle

**Example Scenario**:

```text
1. User starts a long-running inference request (e.g., 2-minute generation)
2. Administrator removes the tier from model annotation at 30 seconds
3. ODH Controller updates RBAC at 45 seconds
4. Request may fail at next authorization checkpoint (if any)
```

**Workaround**:

- Avoid removing tier access during peak usage periods
- Monitor active requests before making changes
- Consider using maintenance windows for tier access changes

#### 2. RBAC Propagation Delay Causes Inconsistent Behavior

**Impact**: Medium

**Description**:

- There is a delay between annotation update and RBAC resource update by the ODH Controller
- During this window (typically seconds to minutes), access behavior is inconsistent:
  - Some requests may still succeed (if authorization was cached)
  - New requests may fail immediately
  - Model may still appear in user's model list but be inaccessible

**Example Timeline**:

```text
T+0s:  Annotation updated (remove "premium" tier)
T+5s:  ODH Controller detects change
T+10s: RoleBinding deleted
T+15s: RBAC fully propagated to API server
```

**Workaround**:

- Wait 1-2 minutes after annotation update before verifying access changes
- Monitor ODH Controller logs to confirm RBAC updates are complete
- Use `kubectl get rolebinding -n <model-namespace>` to verify RoleBinding removal

#### 3. Model List Visibility vs. Access Mismatch

**Impact**: Low

**Description**:

- The `/v1/models` endpoint lists all models that are part of the MaaS instance (via gateway references)
- The endpoint does not filter models by tier access permissions
- Users may see models in the list that they can no longer access after tier removal
- Attempts to use these models will fail with `403 Forbidden` or `401 Unauthorized`

**Example**:

```json
// GET /v1/models returns:
{
  "data": [
    {"id": "model-a", "ready": true},  // Still accessible
    {"id": "model-b", "ready": true}   // No longer accessible after tier removal
  ]
}

// POST to model-b fails with 403
```

**Workaround**:

- Users should handle `403` errors gracefully in their applications
- Consider implementing client-side filtering based on access errors
- Future enhancement: Filter model list by tier permissions

#### 4. Existing Tokens Remain Valid But Lose Model Access

**Impact**: Low

**Description**:

- Service Account tokens issued before tier removal remain valid until expiration
- However, these tokens immediately lose access to models removed from the tier
- Users do not need to request new tokens, but they cannot access the removed models
- This is expected behavior but may be confusing to users

**Example**:

```text
1. User receives token at T+0 (valid for 1 hour)
2. User has access to models A, B, C
3. Model B removed from tier at T+30min
4. Token still valid, but:
   - Model A: ✅ Accessible
   - Model B: ❌ No longer accessible (403 Forbidden)
   - Model C: ✅ Accessible
```

**Workaround**:

- Document this behavior for end users
- Provide clear error messages when access is denied
- Consider implementing access validation in token issuance (future enhancement)

#### 5. No Graceful Shutdown Mechanism

**Impact**: Low

**Description**:

- There is no mechanism to wait for active requests to complete before removing tier access
- Administrators cannot "drain" active connections before revoking access
- All access revocation is immediate once RBAC is updated

**Workaround**:

- Monitor active requests before making changes:

  ```bash
  # Check for active connections (example)
  kubectl top pods -n <model-namespace>
  ```

- Use maintenance windows for tier access changes
- Consider implementing request draining in future releases

### Recommended Practices

1. **Plan Tier Access Changes**:
   - Schedule changes during low-usage periods
   - Notify affected users in advance when possible
   - Monitor active requests before making changes

2. **Verify Changes**:

   - Wait 1-2 minutes after annotation update
   - Verify RoleBinding removal:

     ```bash
     kubectl get rolebinding -n <model-namespace> | grep <tier-name>
     ```

   - Test access with a token from the affected tier

3. **Monitor for Issues**:
   - Check ODH Controller logs for RBAC update errors
   - Monitor API server logs for authorization failures
   - Watch for increased error rates in user applications

4. **Handle Errors Gracefully**:
   - Implement retry logic with exponential backoff
   - Provide clear error messages to end users
   - Log access denials for troubleshooting

### Future Enhancements

The following improvements are planned for future releases:

1. **Graceful Shutdown**: Implement request draining before access revocation
2. **Model List Filtering**: Filter `/v1/models` by tier permissions
3. **Access Validation**: Validate tier access during token issuance
4. **Real-time Notifications**: Notify users when tier access changes
5. **Audit Logging**: Enhanced logging for tier access changes

### Related Documentation

- [Tier Configuration](./tier-configuration.md) - How to configure tier access
- [Model Setup](./model-setup.md) - How to configure model tier annotations
- [Token Management](./token-management.md) - Understanding token lifecycle

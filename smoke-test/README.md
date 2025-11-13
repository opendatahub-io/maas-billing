# Smoke Tests

This directory contains smoke tests for the MaaS Billing API.

## Architecture

The smoke tests use a common framework (`common.sh`) that provides:
- Consistent logging and error handling
- API helper functions (GET, POST, DELETE)
- Token management utilities
- Kubernetes secret inspection helpers
- Auto-detection of cluster configuration

All test scripts source `common.sh` to leverage these shared utilities, similar to how pytest uses `conftest.py`.

## Named Token Test

The `named-token` test validates the named token functionality including:

1. **Token Creation**: Creates a named token via the API
2. **Token Validation**: Verifies the token works for authenticated API calls
3. **Negative Testing**: Confirms invalid tokens return 401 Unauthorized
4. **Metadata Verification**: Checks that a Kubernetes secret was created with the correct metadata

### Prerequisites

- `kubectl` installed and configured with cluster access
- `jq` installed for JSON parsing (version 1.5+)
- `curl` installed for API calls
- `bash` version 4.0 or higher

All prerequisites are automatically checked by the test framework.

### Environment Variables

| Variable | Required | Description | Example | Auto-Detection |
|----------|----------|-------------|---------|----------------|
| `MAAS_API_BASE_URL` | No* | Base URL for the MaaS API | `https://maas.example.com/maas-api` | Auto-detected from OpenShift cluster domain |
| `OC_TOKEN` | No* | OpenShift/Kubernetes authentication token | `sha256~...` | Auto-detected via `oc whoami -t` |
| `NAMESPACE_PREFIX` | No | Namespace prefix for tier namespaces | `maas` (default) | N/A |
| `CLEANUP_ON_EXIT` | No | Whether to cleanup test tokens on exit | `true` (default) | N/A |
| `KEEP_SECRETS` | No | Whether to keep test secrets for inspection | `false` (default - secrets are deleted) | N/A |

\* These variables are auto-detected if not set. Manual setting is only required if auto-detection fails or for non-OpenShift clusters.

### Running the Tests

#### Bash Script

```bash
# Using Podman (Recommended)
export MAAS_API_BASE_URL="https://maas-api.example.com"
export OC_TOKEN="$(oc whoami -t)"
./named-token.sh

# Keep secrets for inspection (don't delete after test)
./named-token.sh --keep-secrets

# Or use environment variable
KEEP_SECRETS=true ./named-token.sh
```

#### Docker Alternative

```bash
# If using Docker instead of Podman, the script works the same:
export MAAS_API_BASE_URL="https://maas-api.example.com"
export OC_TOKEN="$(kubectl config view --raw -o jsonpath='{.users[0].user.token}')"
./named-token.sh
```

#### Python Script

```bash
# Install dependencies
pip install -r requirements.txt

# Run the test
export MAAS_API_BASE_URL="https://maas-api.example.com"
export OC_TOKEN="$(oc whoami -t)"
python test_named_token.py

# Keep secrets for inspection
KEEP_SECRETS=true python test_named_token.py
```

### What the Test Validates

#### 1. Token Creation
- POST request to `/v1/tokens` with a name field
- Response contains a valid JWT token
- Response includes expiration timestamp

#### 2. Token Functionality
- Token can be used to access protected endpoints
- GET request to `/v1/models` succeeds with status 200

#### 3. Invalid Token Handling
- Invalid/malformed tokens return 401 Unauthorized
- API properly rejects unauthenticated requests

#### 4. Kubernetes Secret Metadata
The test verifies a Kubernetes secret was created with:
- **username**: Token owner
- **creationDate**: Creation timestamp (RFC3339 format)
- **expirationDate**: Expiration timestamp (RFC3339 format)
- **name**: User-provided token name
- **status**: Current status (should be "active")

**Security Check**: The test also verifies that the actual token value is **NOT** stored in the secret (only metadata).

### Expected Output

```
==========================================
  Named Token Smoke Test
==========================================

[INFO] Checking prerequisites...
[✓] All prerequisites met

[INFO] Test 1: Creating named token 'smoke-test-1234567890'...
[✓] Named token created successfully
[INFO] Token: eyJhbGciOiJSUzI1Ni...
[INFO] Expires at: 1234567890

[INFO] Test 2: Validating token works for API calls...
[✓] Token works correctly for authenticated requests

[INFO] Test 3: Testing invalid token returns 401...
[✓] Invalid token correctly returned 401 Unauthorized

[INFO] Test 4: Verifying Kubernetes secret was created with metadata...
[INFO] Looking for token metadata secret...
[✓] Found token metadata secret: token-user-smoke-test-a1b2c3d4 in namespace: maas-tier-free
[INFO] Validating secret metadata fields...
[INFO]   username: testuser
[INFO]   creationDate: 2024-11-13T12:00:00Z
[INFO]   expirationDate: 2024-11-13T13:00:00Z
[INFO]   name: smoke-test-1234567890
[INFO]   status: active
[✓] All required metadata fields present and valid
[✓] Confirmed: Actual token value is NOT stored in the secret (as expected)

[INFO] Cleaning up: Revoking test token...
==========================================
[✓] All tests passed!
==========================================
```

### Troubleshooting

#### Secret Not Found
If the test can't find the secret:
1. Check that the maas-api has RBAC permissions for secrets
2. Verify tier namespaces exist (e.g., `maas-tier-free`)
3. Check the maas-api logs for errors during secret creation

#### Token Creation Failed
If token creation fails:
1. Verify `OC_TOKEN` is valid: `oc whoami`
2. Check maas-api is running: `kubectl get pods -n <namespace>`
3. Review maas-api logs: `kubectl logs -n <namespace> <pod-name>`

#### 401 on Valid Token
If the valid token returns 401:
1. Check token hasn't expired
2. Verify the service account wasn't deleted
3. Check Istio/gateway authentication policies

### Cleanup

The script automatically cleans up by default:
- **Tokens**: Revoked when the test exits (unless `CLEANUP_ON_EXIT=false`)
- **Secrets**: Deleted when the test exits (unless `--keep-secrets` flag or `KEEP_SECRETS=true`)

You can keep secrets for inspection:

```bash
# Keep secrets (don't delete after test)
./named-token.sh --keep-secrets

# Or use environment variable
KEEP_SECRETS=true ./named-token.sh

# Manually view a secret after the test
kubectl get secret -n maas-tier-free -l maas.opendatahub.io/token-secret=true
kubectl get secret <secret-name> -n maas-tier-free -o yaml

# Manually delete test secrets when done
kubectl delete secrets -l maas.opendatahub.io/token-secret=true -n maas-tier-free
```

### Integration with CI/CD

This test can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run Named Token Smoke Test
  env:
    MAAS_API_BASE_URL: ${{ secrets.MAAS_API_URL }}
    OC_TOKEN: ${{ secrets.OC_TOKEN }}
  run: |
    cd smoke-test
    ./named-token.sh
```

## Common Test Framework

The `common.sh` file provides a shared test framework inspired by the validation-deployment script. It includes:

### Core Features

- **Auto-Configuration**: Automatically detects cluster domain and obtains OC token when possible
- **Logging Functions**: Consistent colored output with `log_info`, `log_success`, `log_error`, `log_warning`
- **API Helpers**: Wrapper functions for GET, POST, DELETE requests with authentication
- **Token Management**: Functions to mint and revoke tokens
- **Secret Inspection**: Utilities to find and validate Kubernetes secrets
- **Prerequisites Checking**: Validates required tools and environment variables

### Creating New Tests

To create a new test using the common framework:

```bash
#!/bin/bash
# Source the common framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

set -euo pipefail

# Your test function
test_my_feature() {
    print_subheader "Test: My Feature"
    
    log_check "Testing something"
    
    # Use API helpers
    local response=$(api_get "/v1/models")
    parse_response "$response"
    
    local http_code=$(get_response_code)
    local body=$(get_response_body)
    
    if [ "$http_code" = "200" ]; then
        log_success "Feature works!"
        return 0
    else
        log_error "Feature failed" "$body"
        return 1
    fi
}

# Main function
main() {
    print_header "🧪 My Test Suite"
    
    check_prerequisites || exit 1
    
    test_my_feature || TEST_FAILED=1
    
    print_summary "My Test Suite"
}

main "$@"
```

### Available Common Functions

#### Logging
- `log_info "message"` - Info message
- `log_success "message"` - Success message (increments PASSED counter)
- `log_error "message" "reason" "suggestion"` - Error message (increments FAILED counter)
- `log_warning "message" "note"` - Warning message
- `log_check "message"` - Checking message
- `print_header "title"` - Print section header
- `print_subheader "title"` - Print subsection header

#### API Calls
- `api_get "/endpoint" "token"` - GET request
- `api_post "/endpoint" "json_data" "token"` - POST request
- `api_delete "/endpoint" "token"` - DELETE request
- `parse_response "$response"` - Parse response into body and code
- `get_response_body` - Get response body
- `get_response_code` - Get HTTP status code

#### Token Management
- `mint_token "1h" "optional-name"` - Create a MaaS token
- `revoke_tokens "token"` - Revoke all tokens

#### Secret Management
- `find_tier_namespaces` - Get all tier namespaces
- `find_secret_by_token_name "token-name"` - Find secret for a token
- `get_secret_field "namespace" "secret" "field"` - Get decoded secret field
- `validate_secret_metadata "namespace" "secret"` - Validate all required fields

#### Prerequisites
- `check_prerequisites` - Check all required tools and env vars
- `check_command "cmd" "install_hint"` - Check if command exists

#### Utilities
- `print_summary "Test Name"` - Print test results summary
- `is_openshift` - Check if running on OpenShift
- `get_cluster_domain` - Get cluster domain
- `auto_detect_base_url` - Auto-detect MaaS API URL


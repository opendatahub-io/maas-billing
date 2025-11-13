# Smoke Test Directory Structure

```
smoke-test/
├── common.sh              # Common test framework (utilities and setup)
├── named-token.sh         # Named token smoke test (the main test)
├── example-test.sh        # Example showing how to use the framework
├── README.md              # Documentation
└── STRUCTURE.md           # This file
```

## Quick Start

### Auto-detect everything (OpenShift)

```bash
cd smoke-test
./named-token.sh
```

The framework will automatically:
- Detect the cluster domain
- Get your OC token via `oc whoami -t`
- Construct the MaaS API URL

### Manual configuration

```bash
cd smoke-test
export MAAS_API_BASE_URL="https://maas.example.com/maas-api"
export OC_TOKEN="$(oc whoami -t)"
./named-token.sh
```

### Keep secrets for inspection

```bash
./named-token.sh --keep-secrets
```

## Files

### `common.sh`
Common test framework that provides:
- Configuration and environment setup
- Logging functions with counters and colors
- API helper functions (GET, POST, DELETE)
- Token management utilities
- Kubernetes secret utilities
- Prerequisite checking
- Auto-detection of cluster configuration

### `named-token.sh`
Complete smoke test for named token functionality:
1. Creates a named token via API
2. Validates the token works for authentication
3. Tests that invalid tokens return 401
4. Verifies Kubernetes secret was created with correct metadata
5. Revokes tokens and verifies expiredAt timestamp is added
6. Confirms actual token is NOT stored in secret

### `example-test.sh`
Simple example showing how to use the common framework:
- Tests health endpoint
- Lists available models
- Mints and revokes a token

Perfect template for creating new tests!

## What the Named Token Test Validates

### 1. Token Creation
- POST to `/v1/tokens` with a name field
- Receives valid JWT token
- Gets expiration timestamp

### 2. Token Works
- Can authenticate with the token
- GET to `/v1/models` returns 200

### 3. Invalid Token Rejected
- Invalid tokens return 401
- Security is enforced

### 4. Secret Metadata
- Secret created in correct namespace
- Contains: username, creationDate, expirationDate, name, status
- **Does NOT** contain the actual token value (security check)

### 5. Token Revocation
- Tokens can be revoked via DELETE `/v1/tokens`
- Secret status changes from "active" to "expired"
- expiredAt timestamp is added (RFC3339 format)

## Creating New Tests

To create a new test, copy this pattern:

```bash
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

set -euo pipefail

test_my_feature() {
    print_subheader "My Test"
    log_check "Testing something"
    
    local response=$(api_get "/v1/endpoint")
    parse_response "$response"
    
    if [ "$(get_response_code)" = "200" ]; then
        log_success "Test passed"
        return 0
    else
        log_error "Test failed"
        return 1
    fi
}

main() {
    print_header "My Test Suite"
    check_prerequisites || exit 1
    test_my_feature || TEST_FAILED=1
    print_summary "My Test Suite"
}

main "$@"
```

All the hard work (authentication, API calls, secret inspection, logging) is handled by `common.sh`!

## Container Runtime

Scripts are Podman-first but Docker compatible:

```bash
# Works the same with Docker
export CONTAINER_ENGINE=docker
./named-token.sh
```

## CI/CD Integration

GitHub Actions example:

```yaml
- name: Run Smoke Test
  env:
    MAAS_API_BASE_URL: https://maas.${{ env.CLUSTER_DOMAIN }}/maas-api
    OC_TOKEN: ${{ secrets.OC_TOKEN }}
  run: |
    cd smoke-test
    ./named-token.sh
```

## Next Steps

1. Run the test: `./named-token.sh`
2. Inspect a secret: `./named-token.sh --keep-secrets`
3. Create your own test based on `example-test.sh`
4. All utilities are in `common.sh` - just source it and go!

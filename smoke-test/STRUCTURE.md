# Smoke Test Directory Structure

```
smoke-test/
├── common.sh              # Common test framework (source this in all tests)
├── named-token.sh         # Named token functionality test
├── example-test.sh        # Example test showing framework usage
├── test_named_token.py    # Python version of named token test
├── run-tests.sh           # Quick start script to run tests
├── requirements.txt       # Python dependencies
├── README.md              # Main documentation
└── .gitignore             # Git ignore patterns
```

## Quick Start

### Option 1: Auto-detect everything (OpenShift only)

```bash
cd smoke-test
./named-token.sh
```

The framework will automatically:
- Detect the cluster domain
- Get your OC token via `oc whoami -t`
- Construct the MaaS API URL

### Option 2: Manual configuration

```bash
cd smoke-test
export MAAS_API_BASE_URL="https://maas.example.com/maas-api"
export OC_TOKEN="$(oc whoami -t)"
./named-token.sh
```

### Option 3: Use the quick start runner

```bash
cd smoke-test
export MAAS_API_BASE_URL="https://maas.example.com/maas-api"
export OC_TOKEN="$(oc whoami -t)"
./run-tests.sh bash      # Run bash tests
./run-tests.sh python    # Run Python tests
./run-tests.sh all       # Run all tests
```

## Key Features

### Common Framework (`common.sh`)

The common framework provides reusable utilities inspired by `validation-deployment.sh`:

- ✅ **Auto-Configuration**: Detects cluster domain and OC token
- ✅ **Consistent Logging**: Color-coded output with counters
- ✅ **API Helpers**: Simple wrappers for authenticated API calls
- ✅ **Token Management**: Mint and revoke tokens easily
- ✅ **Secret Inspection**: Find and validate Kubernetes secrets
- ✅ **Prerequisites Check**: Validates all required tools

### Test Structure

All tests follow this pattern:

1. Source `common.sh`
2. Define test functions
3. Call `check_prerequisites()`
4. Run tests
5. Call `print_summary()`

Example:

```bash
#!/bin/bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

test_my_feature() {
    print_subheader "My Test"
    log_check "Testing something"
    
    local response=$(api_get "/v1/models")
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

## Files

### `common.sh`
Common test framework that provides:
- Configuration and environment setup
- Logging functions with counters
- API helper functions (GET, POST, DELETE)
- Token management utilities
- Kubernetes secret inspection
- Prerequisite checking
- Auto-detection of cluster configuration

### `named-token.sh`
Tests the named token functionality:
1. Creates a named token via API
2. Validates the token works for authentication
3. Tests that invalid tokens return 401
4. Verifies Kubernetes secret was created with correct metadata
5. Confirms actual token is NOT stored in secret

### `example-test.sh`
Simple example showing how to use the common framework:
- Tests health endpoint
- Lists available models
- Mints and revokes a token

### `test_named_token.py`
Python version of the named token test for teams that prefer Python.

### `run-tests.sh`
Quick start script that:
- Checks environment configuration
- Runs bash or Python tests
- Can run all tests sequentially

## Container Runtime

All scripts are Podman-first but Docker compatible. To use Docker:

```bash
# Scripts work the same with Docker
export CONTAINER_ENGINE=docker
./named-token.sh
```

The scripts don't directly use container runtimes, but documentation follows the Podman-first pattern.

## CI/CD Integration

GitHub Actions example:

```yaml
- name: Run Smoke Tests
  env:
    MAAS_API_BASE_URL: https://maas.${{ env.CLUSTER_DOMAIN }}/maas-api
    OC_TOKEN: ${{ secrets.OC_TOKEN }}
  run: |
    cd smoke-test
    ./named-token.sh
```

GitLab CI example:

```yaml
smoke-tests:
  stage: test
  script:
    - export MAAS_API_BASE_URL="https://maas.${CLUSTER_DOMAIN}/maas-api"
    - export OC_TOKEN="${OC_TOKEN}"
    - cd smoke-test
    - ./named-token.sh
```

## Next Steps

To create a new test:

1. Copy `example-test.sh` to `your-test.sh`
2. Source `common.sh` at the top
3. Write your test functions
4. Use the API helpers from `common.sh`
5. Add to `run-tests.sh` if desired

All the hard work (authentication, API calls, secret inspection, logging) is handled by `common.sh`!


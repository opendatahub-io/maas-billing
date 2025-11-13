#!/bin/bash
# Container runtime: Podman (Docker compatible)
# Replace 'podman' with 'docker' if using Docker

# Quick start script for running smoke tests

set -euo pipefail

# Color codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  MaaS Smoke Tests - Quick Start${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Check prerequisites
if [ -z "${MAAS_API_BASE_URL:-}" ]; then
    echo "Error: MAAS_API_BASE_URL not set"
    echo ""
    echo "Usage:"
    echo "  export MAAS_API_BASE_URL='https://maas-api.example.com'"
    echo "  export OC_TOKEN=\$(oc whoami -t)"
    echo "  ./run-tests.sh"
    exit 1
fi

if [ -z "${OC_TOKEN:-}" ]; then
    echo "Error: OC_TOKEN not set"
    echo ""
    echo "Usage:"
    echo "  export OC_TOKEN=\$(oc whoami -t)"
    echo "  ./run-tests.sh"
    exit 1
fi

echo -e "${GREEN}✓${NC} Environment configured"
echo "  MAAS_API_BASE_URL: $MAAS_API_BASE_URL"
echo ""

# Determine which test to run
TEST_TYPE="${1:-bash}"

case "$TEST_TYPE" in
    bash|sh)
        echo "Running bash tests..."
        echo ""
        ./named-token.sh
        ;;
    python|py)
        echo "Running Python tests..."
        echo ""
        # Check if virtualenv exists
        if [ ! -d "venv" ]; then
            echo "Creating virtual environment..."
            python3 -m venv venv
            source venv/bin/activate
            pip install -q -r requirements.txt
        else
            source venv/bin/activate
        fi
        python test_named_token.py
        ;;
    all)
        echo "Running all tests..."
        echo ""
        echo "=== Bash Test ==="
        ./named-token.sh
        echo ""
        echo ""
        echo "=== Python Test ==="
        if [ ! -d "venv" ]; then
            python3 -m venv venv
            source venv/bin/activate
            pip install -q -r requirements.txt
        else
            source venv/bin/activate
        fi
        python test_named_token.py
        ;;
    *)
        echo "Unknown test type: $TEST_TYPE"
        echo ""
        echo "Usage: $0 [bash|python|all]"
        echo "  bash    - Run bash script tests (default)"
        echo "  python  - Run Python tests"
        echo "  all     - Run all tests"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Tests completed${NC}"
echo -e "${GREEN}========================================${NC}"


#!/usr/bin/env bash
set -euo pipefail

# Where this script lives (…/maas_billing_tests_independent/tests)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Project root for this test suite (…/maas_billing_tests_independent)
SUITE_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Artifacts live inside the test suite folder
ARTIFACTS_DIR="${SUITE_ROOT}/artifacts"

# Timestamped run folder so each run is isolated
RUN_DIR="${ARTIFACTS_DIR}/$(date +%F_%H-%M-%S)"

mkdir -p "${RUN_DIR}"

echo "👉 Writing reports to: ${RUN_DIR}"

# Optional: honour a venv if you're already inside one
if [[ -z "${VIRTUAL_ENV:-}" ]]; then
  echo "ℹ️  No active virtualenv detected. Using system Python."
else
  echo "✅ Using virtualenv: ${VIRTUAL_ENV}"
fi

# You can pass extra pytest args, e.g. -k "smoke" or -x
EXTRA_ARGS=("$@")

# Run only this suite’s tests
pytest \
  "${SCRIPT_DIR}" \
  --maxfail=0 \
  --html="${RUN_DIR}/maas-test-report.html" --self-contained-html \
  --junitxml="${RUN_DIR}/maas-test-report.xml" \
  -o log_cli=true --log-cli-level=INFO \
  "${EXTRA_ARGS[@]}"

echo
echo "✅ Done."
echo "📄 HTML:  ${RUN_DIR}/maas-test-report.html"
echo "🧾 JUnit: ${RUN_DIR}/maas-test-report.xml"

#!/bin/bash
set -euo pipefail

SECRET_NAME="kuadrant-pull-secret"

echo "🔍 Fetching existing global pull secret..."
TMPFILE=$(mktemp)
oc get secret/pull-secret -n openshift-config -o json > "$TMPFILE"

echo "🧩 Merging $SECRET_NAME into the global pull secret..."
NEW_SECRET_JSON=$(oc get secret "$SECRET_NAME" -n openshift-config -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d)

# Clean merge with proper JSON formatting
UPDATED_SECRET=$(jq --argjson new "$NEW_SECRET_JSON" \
  '.data[".dockerconfigjson"] = (
    (.data[".dockerconfigjson"] | @base64d | fromjson) as $orig |
    ($orig.auths + $new.auths) | {auths: .} | tojson | @base64
  )' "$TMPFILE")

echo "$UPDATED_SECRET" | oc apply -f -

# Cleanup
rm -f "$TMPFILE"

echo "✅ Global pull secret updated successfully with $SECRET_NAME."
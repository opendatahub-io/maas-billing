#!/bin/bash

# Usage: ./get-token.sh <username> [password]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/create-sa-token.sh"

access_token=$(get_access_token ${1:-}) 

echo "✅ Token retrieved successfully!"
echo ""
echo "🔗 Access Token:"
echo "$access_token"
echo ""
echo "📋 Test API call:"
echo "curl -H 'Authorization: Bearer $access_token' \\"
echo "     -H 'Content-Type: application/json' \\"
echo "     -d '{\"model\":\"simulator-model\",\"messages\":[{\"role\":\"user\",\"content\":\"Hello from $USERNAME!\"}]}' \\"
echo "     http://simulator.maas.local:8000/v1/chat/completions"
echo ""

if command -v jq >/dev/null 2>&1; then
    echo "📋 Token Claims:"
    echo "$access_token" | cut -d. -f2 | base64 -d 2>/dev/null | jq . || echo "Failed to decode token"
fi

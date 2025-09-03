#!/bin/bash

set -euo pipefail

# A function to get a Kubernetes service account token for a given user.
#
# @param {string} username The username for which to generate a token.
# @returns {string} The access token, printed to standard output.
#                   Returns a non-zero exit code on failure.
function get_access_token() {
  local USERNAME="${1:-}"

  if [[ -z "$USERNAME" ]]; then
    # Print usage info to standard error (>&2)
    echo "Usage: get_access_token <username>" >&2
    echo "" >&2
    echo "Available users:" >&2
    echo "  freeuser1, freeuser2 (Free tier)" >&2
    echo "  premiumuser1, premiumuser2 (Premium tier)" >&2
    echo "  enterpriseuser1 (Enterprise tier)" >&2
    echo "" >&2
    return 1 # Return a non-zero code to indicate failure
  fi

  # --- Determine User Tier ---
  local TIER=""
  case "$USERNAME" in
    freeuser1|freeuser2)
      TIER="free"
      ;;
    premiumuser1|premiumuser2)
      TIER="premium"
      ;;
    enterpriseuser1)
      TIER="enterprise"
      ;;
    *)
      echo "⚠️  Error: User '$USERNAME' not found." >&2
      return 1
      ;;
  esac

  # --- Ensure "Tier Namespace" and Service Account Exist ---
  kubectl create ns "inference-gateway-tier-${TIER}" >/dev/null 2>&1 || true
  kubectl create sa "$USERNAME" -n "inference-gateway-tier-${TIER}" >/dev/null 2>&1 || true

  kubectl create token "${USERNAME}" -n "inference-gateway-tier-${TIER}" --audience=maas
}

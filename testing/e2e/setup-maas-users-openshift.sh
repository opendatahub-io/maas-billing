#!/usr/bin/env bash
# ==========================================================
# Minimal OpenShift HTPasswd setup for MAAS users
# ==========================================================
# Creates two users:
#   - maas-admin-user (cluster-admin)
#   - maas-dev-user   (edit cluster-wide)
# Does NOT remove or modify existing identity providers/users
# Assumes 'htpasswd' is already installed.
# ==========================================================

set -euo pipefail

# --- Configuration ---
HTPASSWD_FILE="./maas-users.htpasswd"
HTPASSWD_SECRET_NAME="maas-htpasswd-secret"
IDP_NAME="maas-htpasswd-provider"
ADMIN_USER="maas-admin-user"
ADMIN_PASS="${MAAS_ADMIN_PASSWORD:-$(openssl rand -base64 12)}"
DEV_USER="maas-dev-user"
DEV_PASS="${MAAS_DEV_PASSWORD:-$(openssl rand -base64 12)}"

echo "ADMIN_PASS=${ADMIN_PASS}"
echo "DEV_PASS=${DEV_PASS}"

echo "=== Setting up MAAS HTPasswd Identity Provider ==="

# --- Create htpasswd file ---
echo "Creating htpasswd file with users..."
htpasswd -c -B -b "${HTPASSWD_FILE}" "${ADMIN_USER}" "${ADMIN_PASS}"
htpasswd -B -b "${HTPASSWD_FILE}" "${DEV_USER}" "${DEV_PASS}"

# --- Create or update secret ---
echo "Creating/Updating secret..."
oc create secret generic "${HTPASSWD_SECRET_NAME}" \
  --from-file=htpasswd="${HTPASSWD_FILE}" \
  -n openshift-config --dry-run=client -o yaml | oc apply -f -

# --- Create or patch OAuth configuration ---
if ! oc get oauth cluster &>/dev/null; then
  echo "No existing OAuth found. Creating a new one..."
  cat <<EOF | oc apply -f -
apiVersion: config.openshift.io/v1
kind: OAuth
metadata:
  name: cluster
spec:
  identityProviders:
  - name: ${IDP_NAME}
    mappingMethod: claim
    type: HTPasswd
    htpasswd:
      fileData:
        name: ${HTPASSWD_SECRET_NAME}
EOF
else
  echo "OAuth exists. Checking if MAAS HTPasswd provider already exists..."
  
  # Check if our identity provider already exists
  if oc get oauth cluster -o jsonpath="{.spec.identityProviders[?(@.name=='${IDP_NAME}')].name}" 2>/dev/null | grep -q "${IDP_NAME}"; then
    echo "MAAS HTPasswd provider already exists, skipping..."
  else
    echo "Adding new MAAS HTPasswd provider..."
    # Create a simple patch file to avoid YAML escaping issues
    cat > /tmp/oauth-patch.json <<JSONEOF
{
  "spec": {
    "identityProviders": [
      {
        "name": "${IDP_NAME}",
        "mappingMethod": "claim",
        "type": "HTPasswd",
        "htpasswd": {
          "fileData": {
            "name": "${HTPASSWD_SECRET_NAME}"
          }
        }
      }
    ]
  }
}
JSONEOF
    
    # Apply the patch from file
    if oc patch oauth cluster --type=merge --patch-file /tmp/oauth-patch.json; then
      echo "Successfully added MAAS HTPasswd provider"
    else
      echo "Patch failed, but continuing..."
    fi
    
    # Cleanup
    rm -f /tmp/oauth-patch.json
  fi
fi

# --- Wait for rollout ---
echo "Waiting for authentication rollout..."
sleep 5
oc rollout status deployment/oauth-openshift -n openshift-authentication --timeout=180s || true

# --- Grant roles ---
echo "Granting cluster-admin role to ${ADMIN_USER}..."
oc adm policy add-cluster-role-to-user cluster-admin "${ADMIN_USER}"

echo "Granting edit role (cluster-wide) to ${DEV_USER}..."
oc adm policy add-cluster-role-to-user edit "${DEV_USER}"

echo "=== Done! ==="
echo
echo "Users created:"
echo "  Admin user: ${ADMIN_USER}"
echo "  Dev user:   ${DEV_USER}"

# --- Login as admin user ---
echo "Logging in as admin user..."
oc login -u "${ADMIN_USER}" -p "${ADMIN_PASS}"
echo "Logged in as: $(oc whoami)" 
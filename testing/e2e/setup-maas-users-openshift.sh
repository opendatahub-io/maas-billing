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
ADMIN_PASS="AdminPass123!"
DEV_USER="maas-dev-user"
DEV_PASS="DevPass123!"

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
  echo "OAuth exists. Adding/patching MAAS HTPasswd provider..."
  oc patch oauth cluster --type='json' -p="[
    {
      \"op\": \"add\",
      \"path\": \"/spec/identityProviders/-\",
      \"value\": {
        \"name\": \"${IDP_NAME}\",
        \"mappingMethod\": \"claim\",
        \"type\": \"HTPasswd\",
        \"htpasswd\": {
          \"fileData\": {
            \"name\": \"${HTPASSWD_SECRET_NAME}\"
          }
        }
      }
    }
  ]" || true
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
echo "Login credentials:"
echo "  Admin user: ${ADMIN_USER} / ${ADMIN_PASS}"
echo "  Dev user:   ${DEV_USER} / ${DEV_PASS}"
echo
echo "You can now log in with:"
echo "  oc login -u ${ADMIN_USER} -p ${ADMIN_PASS}"
echo "  oc login -u ${DEV_USER} -p ${DEV_PASS}"

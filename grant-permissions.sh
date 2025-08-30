#!/bin/bash

# Commands to grant Kuadrant policy read permissions
# Must be run by a cluster administrator

echo "🔐 Granting Kuadrant policy read permissions to user 'noyitz'..."

# Apply the RBAC configuration
oc apply -f kuadrant-rbac-permissions.yaml

# Verify the permissions were granted
echo "✅ Verifying permissions..."
oc auth can-i list authpolicies --as=noyitz
oc auth can-i list ratelimitpolicies --as=noyitz

echo "📋 Testing policy access..."
oc get authpolicies -A --as=noyitz
oc get ratelimitpolicies -A --as=noyitz

echo "🎉 Permissions granted successfully!"
echo ""
echo "The backend should now be able to read real Kuadrant policies instead of using mock data."
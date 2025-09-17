#!/bin/bash

# create-my-env.sh - Auto-generate environment files from OpenShift cluster
# Prerequisites: User must be logged into oc (OpenShift CLI)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 MaaS Environment File Generator${NC}"
echo "================================================"

# Check if oc is installed and user is logged in
if ! command -v oc &> /dev/null; then
    echo -e "${RED}❌ Error: OpenShift CLI (oc) is not installed${NC}"
    echo "Please install the OpenShift CLI and try again"
    exit 1
fi

# Check if user is logged in
if ! oc whoami &> /dev/null; then
    echo -e "${RED}❌ Error: Not logged into OpenShift cluster${NC}"
    echo "Please run 'oc login' first and try again"
    exit 1
fi

echo -e "${GREEN}✅ OpenShift CLI detected and user is logged in${NC}"

# Extract cluster information
echo -e "${YELLOW}🔍 Extracting cluster information...${NC}"

# Get cluster server URL and extract domain
CLUSTER_SERVER=$(oc whoami --show-server)
BASE_DOMAIN=$(echo "$CLUSTER_SERVER" | sed 's/https:\/\/api\.//' | sed 's/:6443//')
CLUSTER_DOMAIN="apps.${BASE_DOMAIN}"
CLUSTER_API_URL="$CLUSTER_SERVER"

# Build other URLs
OAUTH_URL="https://oauth-openshift.${BASE_DOMAIN}"
CONSOLE_URL=$(oc whoami --show-console)

echo "  📍 Cluster Domain: $CLUSTER_DOMAIN"
echo "  🌐 API URL: $CLUSTER_API_URL"
echo "  🔐 OAuth URL: $OAUTH_URL"
echo "  🖥️  Console URL: $CONSOLE_URL"

# Get Key Manager route
echo -e "${YELLOW}🔍 Looking for Key Manager route...${NC}"
KEY_MANAGER_ROUTE=$(oc get routes -n platform-services --no-headers 2>/dev/null | grep key-manager | awk '{print $2}' | head -1)

if [ -z "$KEY_MANAGER_ROUTE" ]; then
    echo -e "${YELLOW}⚠️  Warning: Key Manager route not found in platform-services namespace${NC}"
    echo "  Using default pattern: key-manager-route-platform-services.${CLUSTER_DOMAIN}"
    KEY_MANAGER_BASE_URL="https://key-manager-route-platform-services.${CLUSTER_DOMAIN}"
else
    KEY_MANAGER_BASE_URL="https://${KEY_MANAGER_ROUTE}"
    echo "  🔑 Key Manager URL: $KEY_MANAGER_BASE_URL"
fi

# Try to extract admin key
echo -e "${YELLOW}🔍 Extracting admin key...${NC}"
ADMIN_KEY=$(oc get secret key-manager-admin -n platform-services -o jsonpath='{.data.admin-key}' 2>/dev/null | base64 -d 2>/dev/null)

if [ -z "$ADMIN_KEY" ]; then
    echo -e "${YELLOW}⚠️  Warning: Could not extract admin key from secret key-manager-admin${NC}"
    echo "  You may need to manually set ADMIN_KEY in the .env files"
    ADMIN_KEY="admin-key-placeholder"
    ADMIN_KEY_STATUS="⚠️  Not found - needs manual setup"
else
    echo -e "${GREEN}  ✅ Admin key extracted successfully${NC}"
    ADMIN_KEY_STATUS="✅ Extracted"
fi

# Generate backend .env file
echo -e "${YELLOW}📝 Generating backend .env file...${NC}"
cat > apps/backend/.env << EOF
# Backend Environment Configuration
# Generated automatically by create-my-env.sh on $(date)
# Cluster: $CLUSTER_DOMAIN

# =============================================================================
# HOW TO EXTRACT CLUSTER INFORMATION AND KEYS:
# =============================================================================
# 1. Get cluster domain:
#    oc whoami --show-server | sed 's/api\\.//' | sed 's/:6443//'
#    Example: https://api.your-cluster.example.com:6443
#    Result: apps.your-cluster.example.com
#
# 2. Find Key Manager route:
#    kubectl get routes -n platform-services | grep key-manager
#    oc get routes -n platform-services
#
# 3. Extract admin key from secret:
#    kubectl get secret key-manager-admin -n platform-services -o jsonpath='{.data.admin-key}' | base64 -d
#    OR: oc get secret key-manager-admin -n platform-services -o yaml
# =============================================================================

# Cluster Domain Configuration (Required for Prometheus metrics access)
# Extract from: oc whoami --show-server | sed 's/api\\.//' | sed 's/:6443//'
CLUSTER_DOMAIN=$CLUSTER_DOMAIN

# Service URLs  
# Extract from: oc whoami --show-server and replace 'api' with 'oauth-openshift'
OAUTH_URL=$OAUTH_URL

# Extract from: oc whoami --show-console
CONSOLE_URL=$CONSOLE_URL

# Extract from: oc whoami --show-server
CLUSTER_API_URL=$CLUSTER_API_URL

# Key Manager / MaaS API Base URL
# Extract from: kubectl get routes -n platform-services | grep key-manager
KEY_MANAGER_BASE_URL=$KEY_MANAGER_BASE_URL

# Key Manager Authentication (Required for token management)
# Extract from: kubectl get secret key-manager-admin -n platform-services -o jsonpath='{.data.admin-key}' | base64 -d
ADMIN_KEY=$ADMIN_KEY

# Development settings
NODE_ENV=development
PORT=3001
LOG_LEVEL=info
QOS_SERVICE_URL=http://localhost:3005

# Frontend Configuration (for CORS and redirects)
FRONTEND_URL=http://localhost:3000
EOF

echo -e "${GREEN}  ✅ Created apps/backend/.env${NC}"

# Generate frontend .env.local file
echo -e "${YELLOW}📝 Generating frontend .env.local file...${NC}"
cat > apps/frontend/.env.local << EOF
# Frontend Environment Configuration
# Generated automatically by create-my-env.sh on $(date)
# Cluster: $CLUSTER_DOMAIN

# =============================================================================
# HOW TO EXTRACT CLUSTER INFORMATION:
# =============================================================================
# 1. Get your cluster domain:
#    kubectl get ingresses -A | grep console
#    OR: oc whoami --show-server (then extract domain from URL)
#    Example: https://api.your-cluster.example.com:6443
#    Result: apps.your-cluster.example.com
#
# 2. Find Key Manager route:
#    kubectl get routes -n platform-services | grep key-manager
#    OR: oc get routes -n platform-services
# =============================================================================

# Cluster Domain Configuration
# Extract from: kubectl cluster-info | grep "Kubernetes control plane"
REACT_APP_CLUSTER_DOMAIN=$CLUSTER_DOMAIN

# Service URLs
# Extract from: oc whoami --show-server and replace 'api' with 'oauth-openshift'
REACT_APP_OAUTH_URL=$OAUTH_URL

# Extract from: oc whoami --show-console
REACT_APP_CONSOLE_URL=$CONSOLE_URL

# Extract from: oc whoami --show-server
REACT_APP_CLUSTER_API_URL=$CLUSTER_API_URL

# Key Manager / MaaS API Base URL
# Extract from: kubectl get routes -n platform-services | grep key-manager
REACT_APP_KEY_MANAGER_BASE_URL=$KEY_MANAGER_BASE_URL

# Backend Service URL (for local development)
REACT_APP_BACKEND_URL=http://localhost:3001
EOF

echo -e "${GREEN}  ✅ Created apps/frontend/.env.local${NC}"

# Summary
echo ""
echo -e "${GREEN}🎉 Environment files generated successfully!${NC}"
echo "================================================"
echo -e "${BLUE}Generated files:${NC}"
echo "  📁 apps/backend/.env"
echo "  📁 apps/frontend/.env.local"
echo ""
echo -e "${BLUE}Cluster Information:${NC}"
echo "  📍 Domain: $CLUSTER_DOMAIN"
echo "  🔑 Key Manager: $KEY_MANAGER_BASE_URL"
echo "  🔐 Admin Key: $ADMIN_KEY_STATUS"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "  1. Review the generated .env files"
echo "  2. If admin key is missing, extract it manually:"
echo "     oc get secret key-manager-admin -n platform-services -o jsonpath='{.data.admin-key}' | base64 -d"
echo "  3. Start the services:"
echo "     cd apps/backend && npm run dev"
echo "     cd apps/frontend && npm start"
echo ""
echo -e "${GREEN}Happy coding! 🚀${NC}"
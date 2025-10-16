#!/bin/bash

# MaaS Platform Deployment Validation Script
# This script validates that the MaaS platform is correctly deployed and functional

# Note: We don't use 'set -e' because we want to continue validation even if some checks fail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
PASSED=0
FAILED=0
WARNINGS=0

# Helper functions
print_header() {
    echo ""
    echo "========================================="
    echo "$1"
    echo "========================================="
    echo ""
}

print_check() {
    echo -e "${BLUE}🔍 Checking: $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ PASS: $1${NC}"
    ((PASSED++))
}

print_fail() {
    echo -e "${RED}❌ FAIL: $1${NC}"
    if [ -n "$2" ]; then
        echo -e "${RED}   Reason: $2${NC}"
    fi
    if [ -n "$3" ]; then
        echo -e "${YELLOW}   Suggestion: $3${NC}"
    fi
    if [ -n "$4" ]; then
        echo -e "${YELLOW}   Suggestion: $4${NC}"
    fi
    ((FAILED++))
}

print_warning() {
    echo -e "${YELLOW}⚠️  WARNING: $1${NC}"
    if [ -n "$2" ]; then
        echo -e "${YELLOW}   Note: $2${NC}"
    fi
    if [ -n "$3" ]; then
        echo -e "${YELLOW}   $3${NC}"
    fi
    ((WARNINGS++))
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

# Check if running on OpenShift
if ! kubectl api-resources | grep -q "route.openshift.io"; then
    print_fail "Not running on OpenShift" "This validation script is designed for OpenShift clusters" "Use a different validation approach for vanilla Kubernetes"
    exit 1
fi

print_header "🚀 MaaS Platform Deployment Validation"

# ==========================================
# 1. Component Status Checks
# ==========================================
print_header "1️⃣ Component Status Checks"

# Check MaaS API pods
print_check "MaaS API pods"
if kubectl get pods -n maas-api &>/dev/null; then
    MAAS_PODS=$(kubectl get pods -n maas-api --no-headers 2>/dev/null | grep -c "Running" || echo "0")
    if [ "$MAAS_PODS" -gt 0 ]; then
        print_success "MaaS API has $MAAS_PODS running pod(s)"
    else
        print_fail "No MaaS API pods running" "Pods may be starting or failed" "Check: kubectl get pods -n maas-api"
    fi
else
    print_fail "MaaS API namespace not found" "Deployment may not be complete" "Check: kubectl get namespaces"
fi

# Check Kuadrant pods
print_check "Kuadrant system pods"
if kubectl get namespace kuadrant-system &>/dev/null; then
    KUADRANT_PODS=$(kubectl get pods -n kuadrant-system --no-headers 2>/dev/null | grep -c "Running" || echo "0")
    if [ "$KUADRANT_PODS" -gt 0 ]; then
        print_success "Kuadrant has $KUADRANT_PODS running pod(s)"
    else
        print_fail "No Kuadrant pods running" "Kuadrant operators may not be installed" "Check: kubectl get pods -n kuadrant-system"
    fi
else
    print_fail "Kuadrant namespace not found" "Kuadrant may not be installed" "Run: ./deployment/scripts/install-dependencies.sh --kuadrant"
fi

# Check OpenDataHub/KServe pods
print_check "OpenDataHub/KServe pods"
ODH_FOUND=false
ODH_TOTAL_PODS=0

if kubectl get namespace opendatahub &>/dev/null; then
    ODH_PODS=$(kubectl get pods -n opendatahub --no-headers 2>/dev/null | grep -c "Running" || echo "0")
    ODH_TOTAL_PODS=$((ODH_TOTAL_PODS + ODH_PODS))
    ODH_FOUND=true
    if [ "$ODH_PODS" -gt 0 ]; then
        print_info "  opendatahub namespace: $ODH_PODS running pod(s)"
    fi
fi

if kubectl get namespace redhat-ods-applications &>/dev/null; then
    RHOAI_PODS=$(kubectl get pods -n redhat-ods-applications --no-headers 2>/dev/null | grep -c "Running" || echo "0")
    ODH_TOTAL_PODS=$((ODH_TOTAL_PODS + RHOAI_PODS))
    ODH_FOUND=true
    if [ "$RHOAI_PODS" -gt 0 ]; then
        print_info "  redhat-ods-applications namespace: $RHOAI_PODS running pod(s)"
    fi
fi

if [ "$ODH_FOUND" = true ]; then
    if [ "$ODH_TOTAL_PODS" -gt 0 ]; then
        print_success "OpenDataHub/RHOAI has $ODH_TOTAL_PODS total running pod(s)"
    else
        print_warning "No OpenDataHub/RHOAI pods running" "KServe may not be installed or still starting"
    fi
else
    print_warning "OpenDataHub/RHOAI namespaces not found" "KServe may not be installed yet"
fi

# Check LLM namespace
print_check "LLM namespace and models"
if kubectl get namespace llm &>/dev/null; then
    LLM_PODS=$(kubectl get pods -n llm --no-headers 2>/dev/null | grep -c "Running" || echo "0")
    LLM_SERVICES=$(kubectl get llminferenceservices -n llm --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$LLM_SERVICES" -gt 0 ]; then
        print_success "Found $LLM_SERVICES LLMInferenceService(s) with $LLM_PODS running pod(s)"
    else
        print_warning "Models endpoint accessible but no models found" "You may need to deploy a model a simulated model can be deployed with the following command:" "kustomize build docs/samples/models/simulator | kubectl apply --server-side=true --force-conflicts -f -"
    fi
else
    print_warning "LLM namespace not found" "No models have been deployed yet"
fi

# ==========================================
# 2. Gateway Status
# ==========================================
print_header "2️⃣ Gateway Status"

print_check "Gateway resource"
if kubectl get gateway maas-default-gateway -n openshift-ingress &>/dev/null; then
    GATEWAY_ACCEPTED=$(kubectl get gateway maas-default-gateway -n openshift-ingress -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' 2>/dev/null || echo "Unknown")
    GATEWAY_PROGRAMMED=$(kubectl get gateway maas-default-gateway -n openshift-ingress -o jsonpath='{.status.conditions[?(@.type=="Programmed")].status}' 2>/dev/null || echo "Unknown")
    
    if [ "$GATEWAY_ACCEPTED" = "True" ] && [ "$GATEWAY_PROGRAMMED" = "True" ]; then
        print_success "Gateway is Accepted and Programmed"
    elif [ "$GATEWAY_ACCEPTED" = "True" ]; then
        print_warning "Gateway is Accepted but not Programmed yet" "Gateway may still be initializing"
    else
        print_fail "Gateway not ready" "Accepted: $GATEWAY_ACCEPTED, Programmed: $GATEWAY_PROGRAMMED" "Check: kubectl describe gateway maas-default-gateway -n openshift-ingress"
    fi
else
    print_fail "Gateway not found" "Gateway may not be deployed" "Check: kubectl get gateway -A"
fi

print_check "HTTPRoute for maas-api"
if kubectl get httproute maas-api-route -n maas-api &>/dev/null; then
    # Check if any parent has an Accepted condition with status True
    # HTTPRoutes can have multiple parents (Kuadrant policies + gateway controller)
    HTTPROUTE_ACCEPTED=$(kubectl get httproute maas-api-route -n maas-api -o jsonpath='{.status.parents[*].conditions[?(@.type=="Accepted")].status}' 2>/dev/null | grep -q "True" && echo "True" || echo "False")
    if [ "$HTTPROUTE_ACCEPTED" = "True" ]; then
        print_success "HTTPRoute maas-api-route is configured and accepted"
    else
        # Be lenient - if the route exists, that's usually good enough
        print_warning "HTTPRoute maas-api-route exists but acceptance status unclear" "This is usually fine if other checks pass"
    fi
else
    print_fail "HTTPRoute maas-api-route not found" "API routing may not be configured" "Check: kubectl get httproute -n maas-api"
fi

print_check "Gateway hostname"
# Get cluster domain and construct the MaaS gateway hostname
CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}' 2>/dev/null || echo "")
if [ -n "$CLUSTER_DOMAIN" ]; then
    HOST="maas.${CLUSTER_DOMAIN}"
    print_success "Gateway hostname: $HOST"
else
    print_fail "Could not determine cluster domain" "Cannot test API endpoints" "Check: kubectl get ingresses.config.openshift.io cluster"
    HOST=""
fi

# ==========================================
# 3. Policy Status
# ==========================================
print_header "3️⃣ Policy Status"

print_check "AuthPolicy"
AUTHPOLICY_COUNT=$(kubectl get authpolicy -A --no-headers 2>/dev/null | wc -l || echo "0")
if [ "$AUTHPOLICY_COUNT" -gt 0 ]; then
    AUTHPOLICY_STATUS=$(kubectl get authpolicy -n openshift-ingress gateway-auth-policy -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' 2>/dev/null || echo "NotFound")
    if [ "$AUTHPOLICY_STATUS" = "True" ]; then
        print_success "AuthPolicy is configured and accepted"
    else
        print_warning "AuthPolicy found but status: $AUTHPOLICY_STATUS" "Policy may still be reconciling. Try deleting the kuadrant operator pod:" "kubectl delete pod -n kuadrant-system -l control-plane=controller-manager"
    fi
else
    print_fail "No AuthPolicy found" "Authentication may not be enforced" "Check: kubectl get authpolicy -A"
fi

print_check "TokenRateLimitPolicy"
RATELIMIT_COUNT=$(kubectl get tokenratelimitpolicy -A --no-headers 2>/dev/null | wc -l || echo "0")
if [ "$RATELIMIT_COUNT" -gt 0 ]; then
    RATELIMIT_STATUS=$(kubectl get tokenratelimitpolicy -n openshift-ingress gateway-token-rate-limits -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' 2>/dev/null || echo "NotFound")
    if [ "$RATELIMIT_STATUS" = "True" ]; then
        print_success "TokenRateLimitPolicy is configured and accepted"
    else
        print_warning "TokenRateLimitPolicy found but status: $RATELIMIT_STATUS" "Policy may still be reconciling. Try deleting the kuadrant operator pod:" "kubectl delete pod -n kuadrant-system -l control-plane=controller-manager"
    fi
else
    print_fail "No TokenRateLimitPolicy found" "Rate limiting may not be enforced" "Check: kubectl get tokenratelimitpolicy -A"
fi

# ==========================================
# 4. API Endpoint Tests
# ==========================================
print_header "4️⃣ API Endpoint Tests"

if [ -z "$HOST" ]; then
    print_fail "Cannot test API endpoints" "No gateway route found" "Fix gateway route issues first"
else
    print_info "Using gateway endpoint: $HOST"
    
    # Test authentication endpoint
    print_check "Authentication endpoint"
    ENDPOINT="${HOST}/maas-api/v1/tokens"
    print_info "Testing: curl -sSk -X POST $ENDPOINT -H 'Authorization: Bearer \$(oc whoami -t)' -H 'Content-Type: application/json' -d '{\"expiration\": \"10m\"}'"
    
    if command -v oc &> /dev/null; then
        OC_TOKEN=$(oc whoami -t 2>/dev/null || echo "")
        if [ -n "$OC_TOKEN" ]; then
            TOKEN_RESPONSE=$(curl -sSk --connect-timeout 10 --max-time 30 -w "\n%{http_code}" \
                -H "Authorization: Bearer ${OC_TOKEN}" \
                -H "Content-Type: application/json" \
                -X POST \
                -d '{"expiration": "10m"}' \
                "${ENDPOINT}" 2>/dev/null || echo "")
            
            HTTP_CODE=$(echo "$TOKEN_RESPONSE" | tail -n1)
            RESPONSE_BODY=$(echo "$TOKEN_RESPONSE" | sed '$d')
            
            # Handle timeout/connection failure
            if [ -z "$HTTP_CODE" ] || [ "$HTTP_CODE" = "000" ]; then
                print_fail "Connection timeout or failed to reach endpoint" \
                    "The endpoint is not reachable. This is likely because:" \
                    "1) The endpoint is behind a VPN or firewall, 2) DNS resolution failed, 3) Gateway/Route not properly configured. Check: kubectl get gateway -n openshift-ingress && kubectl get httproute -n maas-api"
                TOKEN=""
            elif [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "201" ]; then
                TOKEN=$(echo "$RESPONSE_BODY" | jq -r '.token' 2>/dev/null || echo "")
                if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
                    print_success "Authentication successful (HTTP $HTTP_CODE)"
                else
                    print_fail "Authentication response invalid" "Received HTTP $HTTP_CODE but no token in response" "Check MaaS API logs: kubectl logs -n maas-api -l app=maas-api"
                fi
            elif [ "$HTTP_CODE" = "404" ]; then
                print_fail "Endpoint not found (HTTP 404)" \
                    "Traffic is reaching the Gateway/pods but the path is incorrect" \
                    "Check HTTPRoute configuration: kubectl describe httproute maas-api-route -n maas-api"
                TOKEN=""
            elif [ "$HTTP_CODE" = "502" ] || [ "$HTTP_CODE" = "503" ]; then
                print_fail "Gateway/Service error (HTTP $HTTP_CODE)" \
                    "The Gateway is not able to reach the backend service" \
                    "Check: 1) MaaS API pods are running: kubectl get pods -n maas-api, 2) Service exists: kubectl get svc maas-api -n maas-api, 3) HTTPRoute is configured: kubectl describe httproute maas-api-route -n maas-api"
                TOKEN=""
            else
                print_fail "Authentication failed (HTTP $HTTP_CODE)" "Response: $(echo $RESPONSE_BODY | head -c 100)" "Check AuthPolicy and MaaS API service"
                TOKEN=""
            fi
        else
            print_warning "Cannot get OpenShift token" "Not logged into oc CLI" "Run: oc login"
            TOKEN=""
        fi
    else
        print_warning "oc CLI not found" "Cannot test authentication" "Install oc CLI or use kubectl with token"
        TOKEN=""
    fi
    
    # Test models endpoint
    print_check "Models endpoint"
    if [ -n "$TOKEN" ]; then
        ENDPOINT="${HOST}/maas-api/v1/models"
        print_info "Testing: curl -sSk $ENDPOINT -H 'Content-Type: application/json' -H 'Authorization: Bearer \$TOKEN'"
        
        MODELS_RESPONSE=$(curl -sSk --connect-timeout 10 --max-time 30 -w "\n%{http_code}" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${TOKEN}" \
            "${ENDPOINT}" 2>/dev/null || echo "")
        
        HTTP_CODE=$(echo "$MODELS_RESPONSE" | tail -n1)
        RESPONSE_BODY=$(echo "$MODELS_RESPONSE" | sed '$d')
        
        # Handle timeout/connection failure
        if [ -z "$HTTP_CODE" ] || [ "$HTTP_CODE" = "000" ]; then
            print_fail "Connection timeout or failed to reach endpoint" \
                "The endpoint is not reachable (VPN/firewall/DNS issue)" \
                "Check Gateway and HTTPRoute configuration"
            MODEL_NAME=""
            MODEL_URL=""
        elif [ "$HTTP_CODE" = "200" ]; then
            MODEL_COUNT=$(echo "$RESPONSE_BODY" | jq -r '.data | length' 2>/dev/null || echo "0")
            if [ "$MODEL_COUNT" -gt 0 ]; then
                print_success "Models endpoint accessible, found $MODEL_COUNT model(s)"
                MODEL_NAME=$(echo "$RESPONSE_BODY" | jq -r '.data[0].id' 2>/dev/null || echo "")
                MODEL_URL=$(echo "$RESPONSE_BODY" | jq -r '.data[0].url' 2>/dev/null || echo "")
            else
                print_warning "Models endpoint accessible but no models found" "You may need to deploy a model a simulated model can be deployed with the following command:" "kustomize build docs/samples/models/simulator | kubectl apply --server-side=true --force-conflicts -f -"
                MODEL_NAME=""
                MODEL_URL=""
            fi
        elif [ "$HTTP_CODE" = "404" ]; then
            print_fail "Endpoint not found (HTTP 404)" \
                "Path is incorrect - traffic reaching pods but wrong path" \
                "Check HTTPRoute: kubectl describe httproute maas-api-route -n maas-api"
            MODEL_NAME=""
            MODEL_URL=""
        elif [ "$HTTP_CODE" = "502" ] || [ "$HTTP_CODE" = "503" ]; then
            print_fail "Gateway/Service error (HTTP $HTTP_CODE)" \
                "Gateway cannot reach backend service" \
                "Check MaaS API pods and service: kubectl get pods,svc -n maas-api"
            MODEL_NAME=""
            MODEL_URL=""
        else
            print_fail "Models endpoint failed (HTTP $HTTP_CODE)" "Response: $(echo $RESPONSE_BODY | head -c 100)" "Check MaaS API service and logs"
            MODEL_NAME=""
            MODEL_URL=""
        fi
    else
        print_warning "Skipping models endpoint test" "No authentication token available"
        MODEL_NAME=""
        MODEL_URL=""
    fi
    
    # Test model inference endpoint (if model exists)
    if [ -n "$TOKEN" ] && [ -n "$MODEL_NAME" ] && [ -n "$MODEL_URL" ]; then
        print_check "Model inference endpoint"
        print_info "Testing: curl -sSk -X POST ${MODEL_URL} -H 'Authorization: Bearer \$TOKEN' -H 'Content-Type: application/json' -d '{\"model\": \"${MODEL_NAME}\", \"prompt\": \"Hello\", \"max_tokens\": 5}'"
        
        INFERENCE_RESPONSE=$(curl -sSk --connect-timeout 10 --max-time 30 -w "\n%{http_code}" \
            -H "Authorization: Bearer ${TOKEN}" \
            -H "Content-Type: application/json" \
            -d "{\"model\": \"${MODEL_NAME}\", \"prompt\": \"Hello\", \"max_tokens\": 5}" \
            "${MODEL_URL}" 2>/dev/null || echo "")
        
        HTTP_CODE=$(echo "$INFERENCE_RESPONSE" | tail -n1)
        RESPONSE_BODY=$(echo "$INFERENCE_RESPONSE" | sed '$d')
        
        # Handle timeout/connection failure
        if [ -z "$HTTP_CODE" ] || [ "$HTTP_CODE" = "000" ]; then
            print_fail "Connection timeout or failed to reach endpoint" \
                "Model endpoint is not reachable (VPN/firewall/DNS issue)" \
                "Check Gateway and model HTTPRoute: kubectl get httproute -n llm"
        elif [ "$HTTP_CODE" = "200" ]; then
            print_success "Model inference endpoint working"
        elif [ "$HTTP_CODE" = "404" ]; then
            print_fail "Model inference endpoint not found (HTTP 404)" \
                "Path is incorrect - traffic reaching but wrong path" \
                "Check model HTTPRoute configuration: kubectl get httproute -n llm && kubectl describe llminferenceservice -n llm"
        elif [ "$HTTP_CODE" = "502" ] || [ "$HTTP_CODE" = "503" ]; then
            print_fail "Gateway/Service error (HTTP $HTTP_CODE)" \
                "Gateway cannot reach model service" \
                "Check: 1) Model pods running: kubectl get pods -n llm, 2) Model service exists, 3) HTTPRoute configured: kubectl get httproute -n llm"
        else
            print_fail "Model inference failed (HTTP $HTTP_CODE)" "Response: $(echo $RESPONSE_BODY | head -c 200)" "Check model pod logs and HTTPRoute configuration"
        fi
    fi
    
    # Test rate limiting
    if [ -n "$TOKEN" ] && [ -n "$MODEL_NAME" ] && [ -n "$MODEL_URL" ]; then
        print_check "Rate limiting"
        print_info "Sending 10 rapid requests to test rate limiting..."
        
        SUCCESS_COUNT=0
        RATE_LIMITED_COUNT=0
        
        for i in {1..10}; do
            HTTP_CODE=$(curl -sSk --connect-timeout 5 --max-time 15 -o /dev/null -w "%{http_code}" \
                -H "Authorization: Bearer ${TOKEN}" \
                -H "Content-Type: application/json" \
                -d "{\"model\": \"${MODEL_NAME}\", \"prompt\": \"Test\", \"max_tokens\": 1}" \
                "${MODEL_URL}" 2>/dev/null || echo "000")
            
            if [ "$HTTP_CODE" = "200" ]; then
                ((SUCCESS_COUNT++))
            elif [ "$HTTP_CODE" = "429" ]; then
                ((RATE_LIMITED_COUNT++))
            fi
        done
        
        if [ "$RATE_LIMITED_COUNT" -gt 0 ]; then
            print_success "Rate limiting is working ($SUCCESS_COUNT successful, $RATE_LIMITED_COUNT rate limited)"
        elif [ "$SUCCESS_COUNT" -gt 0 ]; then
            print_warning "Rate limiting may not be enforced" "All $SUCCESS_COUNT requests succeeded without rate limiting"
        else
            print_fail "Rate limiting test failed" "All requests failed" "Check TokenRateLimitPolicy and Limitador"
        fi
    fi
    
    # Test unauthorized access
    print_check "Authorization enforcement (401 without token)"
    if [ -n "$MODEL_NAME" ] && [ -n "$MODEL_URL" ]; then
        UNAUTH_CODE=$(curl -sSk --connect-timeout 5 --max-time 15 -o /dev/null -w "%{http_code}" \
            -H "Content-Type: application/json" \
            -d "{\"model\": \"${MODEL_NAME}\", \"prompt\": \"Test\", \"max_tokens\": 1}" \
            "${MODEL_URL}" 2>/dev/null || echo "000")
        
        if [ "$UNAUTH_CODE" = "401" ]; then
            print_success "Authorization is enforced (got 401 without token)"
        elif [ "$UNAUTH_CODE" = "403" ]; then
            print_success "Authorization is enforced (got 403 without token)"
        else
            print_warning "Authorization may not be enforced" "Got HTTP $UNAUTH_CODE instead of 401/403 without token"
        fi
    fi
fi

# ==========================================
# Summary
# ==========================================
print_header "📊 Validation Summary"

echo "Results:"
echo -e "  ${GREEN}✅ Passed: $PASSED${NC}"
echo -e "  ${RED}❌ Failed: $FAILED${NC}"
echo -e "  ${YELLOW}⚠️  Warnings: $WARNINGS${NC}"
echo ""

if [ "$FAILED" -eq 0 ]; then
    print_success "All critical checks passed! 🎉"
    echo ""
    echo "Next steps:"
    echo "  1. Deploy a model: kustomize build docs/samples/models/simulator | kubectl apply -f -"
    echo "  2. Access the API at: ${HOST:-https://maas.\${CLUSTER_DOMAIN}}"
    echo "  3. Check documentation: docs/README.md"
    exit 0
else
    print_fail "Some checks failed. Please review the errors above."
    echo ""
    echo "Common fixes:"
    echo "  - Wait for pods to start: kubectl get pods -A | grep -v Running"
    echo "  - Check operator logs: kubectl logs -n kuadrant-system -l app.kubernetes.io/name=kuadrant-operator"
    echo "  - Re-run deployment: ./deployment/scripts/deploy-openshift.sh"
    echo ""
    exit 1
fi


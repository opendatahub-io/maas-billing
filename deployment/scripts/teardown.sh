#!/usr/bin/env bash

set -eu

current_cluster=$(oc whoami --show-server 2>/dev/null || echo "Unable to determine cluster")
current_user=$(oc whoami 2>/dev/null || echo "Unable to determine user")

echo "Current cluster: $current_cluster"
echo "Current user: $current_user"
echo ""
echo "This will delete the following namespaces and all their resources:"
echo "  - llm"
echo "  - llm-observability" 
echo "  - kuadrant-system"
echo ""
read -p "Are you sure you want to proceed? (y/N): " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Teardown cancelled."
    exit 0
fi

echo "Commencing teardown of MaaS deployment"

namespaces="llm llm-observability kuadrant-system"
for ns in $namespaces; do
    echo "Deleting namespace: $ns"
    if [ "$ns" = "kuadrant-system" ]; then
        # kuadrant-system often hangs due to finalizers, remove them first
        echo "Removing finalizers from kuadrant-system resources"
        kubectl patch authorino authorino -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
        kubectl patch kuadrant kuadrant -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
        kubectl patch limitador limitador -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
        
        # Also check for any other operator resources that might have finalizers
        kubectl get kuadrants.kuadrant.io -n "$ns" -o name 2>/dev/null | xargs -r -I {} kubectl patch {} -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge || true
        kubectl get limitadors.limitador.kuadrant.io -n "$ns" -o name 2>/dev/null | xargs -r -I {} kubectl patch {} -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge || true
        kubectl get authorinos.operator.authorino.kuadrant.io -n "$ns" -o name 2>/dev/null | xargs -r -I {} kubectl patch {} -n "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge || true
        
        # Force deletion with timeout
        oc delete project "$ns" --force --grace-period=0 --timeout=60s || true
        
        # Wait a moment and check if still exists
        sleep 5
        if oc get project "$ns" >/dev/null 2>&1; then
            echo "Force removing finalizers from kuadrant-system namespace"
            oc patch project "$ns" -p '{"metadata":{"finalizers":[]}}' --type=merge || true
            
            # Wait again and force delete if still stuck
            sleep 5
            if oc get project "$ns" >/dev/null 2>&1; then
                echo "Namespace still stuck, attempting direct deletion"
                oc delete namespace "$ns" --force --grace-period=0 --timeout=30s || true
            fi
        fi
    else
        oc delete project "$ns" || true
    fi
done

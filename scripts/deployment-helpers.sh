#!/bin/bash

# Deployment helper functions for MaaS Platform
# Source this file in deployment scripts: source "$(dirname "$0")/deployment-helpers.sh"

# Helper function to wait for CRD to be established
wait_for_crd() {
  local crd="$1"
  local timeout="${2:-60}"  # timeout in seconds
  local interval=2
  local elapsed=0

  echo "⏳ Waiting for CRD ${crd} to appear (timeout: ${timeout}s)…"
  while [ $elapsed -lt $timeout ]; do
    if kubectl get crd "$crd" &>/dev/null; then
      echo "✅ CRD ${crd} detected, waiting for it to become Established..."
      kubectl wait --for=condition=Established --timeout="${timeout}s" "crd/$crd" 2>/dev/null
      return 0
    fi
    sleep $interval
    elapsed=$((elapsed + interval))
  done

  echo "❌ Timed out after ${timeout}s waiting for CRD $crd to appear." >&2
  return 1
}

# Helper function to wait for CSV to reach Succeeded state
wait_for_csv() {
  local csv_name="$1"
  local namespace="${2:-kuadrant-system}"
  local timeout="${3:-180}"  # timeout in seconds
  local interval=5
  local elapsed=0
  local last_status_print=0

  echo "⏳ Waiting for CSV ${csv_name} to succeed (timeout: ${timeout}s)..."
  while [ $elapsed -lt $timeout ]; do
    local phase=$(kubectl get csv -n "$namespace" "$csv_name" -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")

    case "$phase" in
      "Succeeded")
        echo "✅ CSV ${csv_name} succeeded"
        return 0
        ;;
      "Failed")
        echo "❌ CSV ${csv_name} failed" >&2
        kubectl get csv -n "$namespace" "$csv_name" -o jsonpath='{.status.message}' 2>/dev/null
        return 1
        ;;
      *)
        if [ $((elapsed - last_status_print)) -ge 30 ]; then
          echo "   CSV ${csv_name} status: ${phase} (${elapsed}s elapsed)"
          last_status_print=$elapsed
        fi
        ;;
    esac

    sleep $interval
    elapsed=$((elapsed + interval))
  done

  echo "❌ Timed out after ${timeout}s waiting for CSV ${csv_name}" >&2
  return 1
}

# Helper function to wait for pods in a namespace to be ready
wait_for_pods() {
  local namespace="$1"
  local timeout="${2:-120}"
  
  kubectl get namespace "$namespace" &>/dev/null || return 0
  
  echo "⏳ Waiting for pods in $namespace to be ready..."
  local end=$((SECONDS + timeout))
  while [ $SECONDS -lt $end ]; do
    local not_ready=$(kubectl get pods -n "$namespace" --no-headers 2>/dev/null | grep -v -E 'Running|Completed|Succeeded' | wc -l)
    [ "$not_ready" -eq 0 ] && return 0
    sleep 5
  done
  echo "⚠️  Timeout waiting for pods in $namespace" >&2
  return 1
}

# Helper function to wait for a resource to exist
# Usage: wait_for_resource <resource_type> <resource_name> <namespace> [timeout]
wait_for_resource() {
  local resource_type="$1"
  local resource_name="$2"
  local namespace="$3"
  local timeout="${4:-60}"
  local interval=2
  local elapsed=0

  echo "⏳ Waiting for ${resource_type}/${resource_name} in ${namespace} (timeout: ${timeout}s)..."
  while [ $elapsed -lt $timeout ]; do
    if kubectl get "$resource_type" "$resource_name" -n "$namespace" &>/dev/null; then
      echo "✅ ${resource_type}/${resource_name} exists"
      return 0
    fi
    sleep $interval
    elapsed=$((elapsed + interval))
  done

  echo "⚠️  Timed out waiting for ${resource_type}/${resource_name}" >&2
  return 1
}

# version_compare <version1> <version2>
#   Compares two version strings in semantic version format (e.g., "4.19.9")
#   Returns 0 if version1 >= version2, 1 otherwise
version_compare() {
  local version1="$1"
  local version2="$2"
  
  local v1=$(echo "$version1" | awk -F. '{printf "%d%03d%03d", $1, $2, $3}')
  local v2=$(echo "$version2" | awk -F. '{printf "%d%03d%03d", $1, $2, $3}')
  
  [ "$v1" -ge "$v2" ]
}

wait_for_validating_webhooks() {
    local namespace="$1"
    local timeout="${2:-60}"
    local interval=2
    local end=$((SECONDS+timeout))

    echo "⏳ Waiting for validating webhooks in namespace $namespace (timeout: $timeout sec)..."

    while [ $SECONDS -lt $end ]; do
        local not_ready=0

        local services
        services=$(kubectl get validatingwebhookconfigurations \
          -o jsonpath='{range .items[*].webhooks[*].clientConfig.service}{.namespace}/{.name}{"\n"}{end}' \
          | grep "^$namespace/" | sort -u)

        if [ -z "$services" ]; then
            echo "⚠️  No validating webhooks found in namespace $namespace"
            return 0
        fi

        for svc in $services; do
            local ns name ready
            ns=$(echo "$svc" | cut -d/ -f1)
            name=$(echo "$svc" | cut -d/ -f2)

            ready=$(kubectl get endpoints -n "$ns" "$name" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || true)
            if [ -z "$ready" ]; then
                echo "🔴 Webhook service $ns/$name not ready"
                not_ready=1
            else
                echo "✅ Webhook service $ns/$name has ready endpoints"
            fi
        done

        if [ "$not_ready" -eq 0 ]; then
            echo "🎉 All validating webhook services in $namespace are ready"
            return 0
        fi

        sleep $interval
    done

    echo "❌ Timed out waiting for validating webhooks in $namespace"
    return 1
}


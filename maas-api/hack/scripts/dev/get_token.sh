#!/usr/bin/env bash

: "${HOST:=$(kubectl get gateway openshift-ai-inference -n openshift-ingress -o jsonpath='{.status.addresses[0].value}')}"

curl -sSk \
  -H "Authorization: Bearer $(oc whoami -t)" \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{
    "ttyl": "10m"
  }' \
  ${HOST}/maas-api/v1/tokens | jq -r .token

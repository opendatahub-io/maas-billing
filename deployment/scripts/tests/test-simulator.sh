#!/usr/bin/env bash

set -eux

SIMULATOR_ROUTE=$(oc get routes simulator-route -n llm | tail -1 | awk '{print $2}')

curl -s -w "HTTP Status: %{http_code}\n" \
    -H 'Authorization: APIKEY freeuser1_key' \
    -H 'Content-Type: application/json' \
    -d '{"model":"simulator-model","messages":[{"role":"user","content":"Hello!"}],"max_tokens":10}' \
    "http://${SIMULATOR_ROUTE}/v1/chat/completions" 

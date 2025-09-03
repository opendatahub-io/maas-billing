#!/usr/bin/env bash

set -eux

QWEN3_ROUTE=$(oc get routes qwen3-route | tail -1 | awk '{print $2}')

curl -s -w "HTTP Status: %{http_code}\n" \
    -H 'Authorization: APIKEY premiumuser1_key' \
    -H 'Content-Type: application/json' \
    -d '{"model":"simulator-model","messages":[{"role":"user","content":"Please write a python function."}],"max_tokens":10}' \
    "http://${QWEN3_ROUTE}/v1/chat/completions" 

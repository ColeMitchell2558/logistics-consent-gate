#!/bin/sh
set -eu

base_url="${BASE_URL:-http://localhost:8080}"
user_id="${USER_ID:-user_logistics_demo}"

curl --fail-with-body --request PUT "${base_url}/users/${user_id}/consents" \
  --header 'Content-Type: application/json' \
  --data '{"category":"third_party","source":"dispatch_portal","idempotency_key":"grant-demo-pod-001"}'

curl --fail-with-body --request POST "${base_url}/shipment-events" \
  --header 'Content-Type: application/json' \
  --data "{\"user_id\":\"${user_id}\",\"shipment_id\":\"shp_demo_101\",\"kind\":\"proof_of_delivery\",\"occurred_at\":\"2026-09-30T09:00:00Z\",\"proof_object_key\":\"pod/shp_demo_101.jpg\"}"

curl --fail-with-body --request DELETE "${base_url}/users/${user_id}/consents" \
  --header 'Content-Type: application/json' \
  --data '{"category":"third_party","idempotency_key":"revoke-demo-pod-001"}'

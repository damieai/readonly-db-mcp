#!/bin/sh
set -eu

# Run with a disposable provisioning identity. ES_CURL_CONFIG is a private
# curl config file containing credentials; ES_CA is the trusted CA certificate.
: "${ES_URL:?set ES_URL to the pinned Elasticsearch endpoint}"
: "${ES_CA:?set ES_CA to the trusted CA certificate}"
: "${ES_CURL_CONFIG:?set ES_CURL_CONFIG to a private curl config file}"

request() {
  curl --fail-with-body --silent --show-error --cacert "$ES_CA" --config "$ES_CURL_CONFIG" "$@"
}

request -X PUT "$ES_URL/agent-incidents" -H 'Content-Type: application/json' \
  --data-binary '{"mappings":{"properties":{"incident_id":{"type":"integer"},"service":{"type":"keyword"},"summary":{"type":"text"}}}}'

request -X PUT "$ES_URL/agent-incidents/_doc/101?refresh=wait_for" -H 'Content-Type: application/json' \
  --data-binary '{"incident_id":101,"service":"checkout","summary":"Checkout timeout during payment authorization"}'
request -X PUT "$ES_URL/agent-incidents/_doc/102?refresh=wait_for" -H 'Content-Type: application/json' \
  --data-binary '{"incident_id":102,"service":"checkout","summary":"Checkout latency after cache expiry"}'
request -X PUT "$ES_URL/agent-incidents/_doc/103?refresh=wait_for" -H 'Content-Type: application/json' \
  --data-binary '{"incident_id":103,"service":"catalog","summary":"Catalog indexing delay"}'

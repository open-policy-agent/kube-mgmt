#!/usr/bin/env sh
set -ex

OPA="http --ignore-stdin :8080/v1"

kubectl apply -f "$(dirname $0)/../fixture-replication.yaml"

${OPA}/data/kubernetes/services/ignore-me | jq -e '.result==null'

${OPA}/data/kubernetes/services/dont-ignore-me | jq -e '.result|keys==["dont-ignore-me"]'

${OPA}/data/kubernetes/services/default | jq -e '.result|length==2'

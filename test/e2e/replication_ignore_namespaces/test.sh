#!/usr/bin/env sh
set -ex

TOKEN=$(kubectl exec deploy/kube-mgmt-opa-kube-mgmt -c mgmt -- cat /bootstrap/mgmt-token)
OPA="http --ignore-stdin --default-scheme=https --verify=no -A bearer -a ${TOKEN} :8443/v1"

kubectl apply -f "$(dirname $0)/../fixture-replication.yaml"

${OPA}/data/kubernetes/configmaps/ignore-me  | jq -e '.result|length==0'

${OPA}/data/kubernetes/configmaps/dont-ignore-me  | jq -e '.result|length>=1'

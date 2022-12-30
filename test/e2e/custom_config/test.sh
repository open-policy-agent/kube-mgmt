#!/usr/bin/env sh
set -ex

TOKEN=$(kubectl exec deploy/kube-mgmt-opa-kube-mgmt -c mgmt -- cat /bootstrap/mgmt-token)
OPA="http --ignore-stdin --default-scheme=https --verify=no -A bearer -a ${TOKEN} :8443/v1"

${OPA}/data | jq -e '.result.test_helm_kubernetes_quickstart|keys|length==3'


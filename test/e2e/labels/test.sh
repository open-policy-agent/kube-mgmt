#!/bin/sh
set -e
set -x

OPA="http --ignore-stdin :8080/v1"

${OPA}/data | jq -e '.result.default//{}|keys|length==0'

kubectl apply -f "$(dirname $0)/../fixture-labels.yaml"

${OPA}/policies | jq -e '.result[].id=="default/policy-include/include.rego"'
${OPA}/data/example/include/allow | jq -e '.result==true'

${OPA}/data/default | jq -e '.result|keys==["data-include"]'
${OPA}/data/default/data-include | jq -e '.result["include.json"].inKey=="inValue"'

kubectl get cm -l qweqwe/policy=111 -ojson | \
  jq -e '.items[].metadata.annotations["openpolicyagent.org/kube-mgmt-status"]|fromjson|.status=="ok"'

kubectl get cm -l asdasd/data=222 -ojson | \
  jq -e '.items[].metadata.annotations["openpolicyagent.org/kube-mgmt-status"]|fromjson|.status=="ok"'

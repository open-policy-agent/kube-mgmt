#!/bin/sh
set -e
set -x

TOKEN=$(kubectl exec deploy/kube-mgmt-opa-kube-mgmt -c mgmt -- cat /bootstrap/mgmt-token)
OPA="http --ignore-stdin --default-scheme=https --verify=no -A bearer -a ${TOKEN} :8443/v1"

${OPA}/data | jq -e '.result.test_helm_kubernetes_quickstart|keys|length==3'

kubectl apply -f "$(dirname $0)/../fixture.yaml"

${OPA}/policies | jq -e '.result|any(.id=="default/policy-include/include.rego")==true'
${OPA}/data/example/include/allow | jq -e '.result==true'

${OPA}/data/default | jq -e '.result|keys==["data-include"]'
${OPA}/data/default/data-include | jq -e '.result["include.json"].inKey=="inValue"'

kubectl get cm -l openpolicyagent.org/policy=rego -ojson | \
  jq -e '.items[].metadata.annotations["openpolicyagent.org/kube-mgmt-status"]|fromjson|.status=="ok"'

kubectl get cm -l openpolicyagent.org/data=opa -ojson | \
  jq -e '.items[].metadata.annotations["openpolicyagent.org/kube-mgmt-status"]|fromjson|.status=="ok"'

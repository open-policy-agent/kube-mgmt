#!/bin/bash
set -ex

OPA="http --ignore-stdin :8080/v1"
FX_OK="$(dirname $0)/../fixture-multi.yaml"
FX_KO="$(dirname $0)/../fixture-multi-fail.yaml"

${OPA}/data/my_pkg/my_rule | jq -e '.result==null'

kubectl apply -f ${FX_OK}
sleep 3

${OPA}/policies | jq -e '.result|any(.id=="default/multi-file-policy/a.rego")==true'
${OPA}/policies | jq -e '.result|any(.id=="default/multi-file-policy/b.rego")==true'

kubectl get cm multi-file-policy -ojson | \
  jq -e '.metadata.annotations["openpolicyagent.org/kube-mgmt-status"]|fromjson|.status=="ok"'
kubectl get cm multi-file-policy -ojson | \
  jq -e '.metadata.annotations["openpolicyagent.org/kube-mgmt-retries"]=="0"'

${OPA}/data/my_pkg/my_rule input[hello]=world | jq -e '.result==true'
${OPA}/data/my_pkg/my_rule input[hello]=incorrect | jq -e '.result==false'

######
#
######

kubectl apply -f ${FX_KO}
sleep 3

${OPA}/policies | jq -e '.result|length==2'

kubectl get cm multi-file-fail-policy -ojson | \
  jq -e '.metadata.annotations["openpolicyagent.org/kube-mgmt-status"]|fromjson|.status=="error"'
kubectl get cm multi-file-fail-policy -ojson | \
  jq -e '.metadata.annotations["openpolicyagent.org/kube-mgmt-retries"]=="0"'

######
#
######

cat ${FX_OK} | \
  yq '.metadata.labels["openpolicyagent.org/policy"]=""' | \
  yq '.metadata.annotations["openpolicyagent.org/kube-mgmt-retries"]="0"' | \
  kubectl apply -f -
sleep 3

${OPA}/data/my_pkg/my_rule | jq -e '.result==null'

kubectl label --overwrite cm multi-file-policy openpolicyagent.org/policy=rego
sleep 3

${OPA}/policies | jq -e '.result|any(.id=="default/multi-file-policy/a.rego")==true'
${OPA}/policies | jq -e '.result|any(.id=="default/multi-file-policy/b.rego")==true'

kubectl get cm multi-file-policy -ojson | \
  jq -e '.metadata.annotations["openpolicyagent.org/kube-mgmt-status"]|fromjson|.status=="ok"'
kubectl get cm multi-file-policy -ojson | \
  jq -e '.metadata.annotations["openpolicyagent.org/kube-mgmt-retries"]=="0"'

${OPA}/data/my_pkg/my_rule input[hello]=world | jq -e '.result==true'
${OPA}/data/my_pkg/my_rule input[hello]=incorrect | jq -e '.result==false'


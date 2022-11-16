#!/bin/bash

helm lint charts/opa-kube-mgmt --strict
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set mgmt.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set sar.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set certManager.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set prometheus.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set admissionController.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set authz.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set useHttps=false
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set "extraArgs={aa,bb,cc}"
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set authz.enabled=true --set authz.mgmtToken.secretName=secretName --set authz.mgmtToken.secretKey=secretKey
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa-kube-mgmt --strict --set extraVolumeMounts\[0\].name=storage --set extraVolumeMounts\[0\].mountPath=/storage
if [ $? -ne 0 ]; then
  exit 1
fi

echo "=================================================================================="
echo "                                LINT PASSED"
echo "=================================================================================="

#!/bin/bash

helm lint charts/opa --strict
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa --strict --set mgmt.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa --strict --set sar.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa --strict --set certManager.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa --strict --set prometheus.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa --strict --set admissionController.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa --strict --set authz.enabled=true
if [ $? -ne 0 ]; then
  exit 1
fi

helm lint charts/opa --strict --set useHttps=false
if [ $? -ne 0 ]; then
  exit 1
fi

echo "=================================================================================="
echo "                                LINT PASSED"
echo "=================================================================================="

#!/usr/bin/env sh
set -ex

opa build -b $(dirname $0)/bundle -o $(dirname $0)/bundle.tar.gz
kubectl delete configmap bundle || true
kubectl create configmap bundle --from-file $(dirname $0)/bundle.tar.gz
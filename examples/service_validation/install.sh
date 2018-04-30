#!/bin/bash

set -ex

OUT_DIR=/tmp/opa
rm -rf ${OUT_DIR}; mkdir -p ${OUT_DIR}

openssl genrsa -out ${OUT_DIR}/ca.key 2048
openssl req -x509 -new -nodes -key ${OUT_DIR}/ca.key -days 100000 -out ${OUT_DIR}/ca.crt -subj "/CN=admission_ca"
cat >${OUT_DIR}/server.conf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
EOF
openssl genrsa -out ${OUT_DIR}/server.key 2048
openssl req -new -key ${OUT_DIR}/server.key -out ${OUT_DIR}/server.csr -subj "/CN=opa.opa.svc" -config ${OUT_DIR}/server.conf
openssl x509 -req -in ${OUT_DIR}/server.csr -CA ${OUT_DIR}/ca.crt -CAkey ${OUT_DIR}/ca.key -CAcreateserial -out ${OUT_DIR}/server.crt -days 100000 -extensions v3_req -extfile ${OUT_DIR}/server.conf

install_namespace=opa
caBundle=$(base64 ${OUT_DIR}/ca.crt)
cp admission_controller.yaml ${OUT_DIR}/admission_controller.yaml
gsed -i "s/REPLACE_WITH_SECRET/${caBundle}/" ${OUT_DIR}/admission_controller.yaml

kubectl create namespace ${install_namespace}
kubectl create secret tls opa-server --cert=${OUT_DIR}/server.crt --key=${OUT_DIR}/server.key --namespace ${install_namespace}
kubectl apply -f ${OUT_DIR}/admission_controller.yaml --namespace ${install_namespace}


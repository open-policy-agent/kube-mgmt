# Kubernetes Admission Control for preventing open AWS LoadBalancers

Kubernetes Service objects of type [LoadBalancer](https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/) on AWS create an Elastic LoadBalancer or Network LoadBalancer. However, if not properly configured, these LoadBalancers can be open to the world exposing EC2 instances behind them to security breaches.

OPA can provide a good ValidationWebhook for ensuring that Service objects of type LoadBalancer do not accidentally create a LoadBalancer open to the world.

## Goals

This tutorial shows how to create validation webhooks for Service objects and enforcing the LoadBalancer policies.

- Kubernetes Service objects of type LoadBalancer that do not have `spec.loadBalancerSourceRanges` are rejected.
- Users are required to explicitly set `spec.loadBalancerSourceRanges`. If users want to create LoadBalancers that are actually open to the world, they should explicitly set `spec.loadBalancerSourceRanges` to `0.0.0.0/0`.

## Prerequisites

This tutorial has been tested with Kubernetes 1.10 running on AWS with RBAC enabled. But it should work with Kubernetes 1.9 or higher.

## Steps

### The simplest way to setup opa and policies would be to run the install.sh script.

```bash
$ ./install.sh
```

Otherwise, here are the detailed steps:

### 1. Start Kubernetes with ValidatingAdmissionWebhook admission controller enabled.

### 2. Create the namespace called `opa` in it.

```bash
kubectl create namespace opa
```

### 3. Create the SSL certs required for the webhook. Same as [this](https://github.com/open-policy-agent/opa/blob/master/docs/book/kubernetes-admission-control.md#3-deploy-opa-on-top-of-kubernetes)

```bash
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -days 100000 -out ca.crt -subj "/CN=admission_ca"
```

Generate the TLS key and certificate for OPA:

```bash
cat >server.conf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
EOF
```

```bash
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr -subj "/CN=opa.opa.svc" -config server.conf
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 100000 -extensions v3_req -extfile server.conf
```

> Note: the Common Name value you give to openssl MUST match the name of the OPA service created below.

Create a Secret to store the TLS credentials for OPA:

```bash
kubectl create secret tls opa-server --cert=server.crt --key=server.key
```

In the admission_controller.yaml file in this example, replace the REPLACE_WITH_SECRET with the base64 encoded 

```bash
kubectl apply -f ./examples/service_validation/admission-controller.yaml
```

This creates the OPA deployment, the validation webhook as well as the config map which has the policy.

### 4. Exercise the policy

Create a service object and ensure that it is enforcing the policy.

**service_invalid.yaml**:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: no-whitelist-ips
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 80
  selector:
    run: nginx
  type: LoadBalancer
```

**service_valid.yaml**:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: whitelist-ips
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 80
  selector:
    run: nginx
  type: LoadBalancer
  loadBalancerSourceRanges:
  - 10.0.0.0/8
```

```bash
kubectl create -f service_invalid.yaml
kubectl create -f service_valid.yaml
```

This tutorial showed how you can leverage OPA to enforce admission control of Service objects to prevent accidentally exposing AWS resources to the world.


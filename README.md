# Kubernetes Management Sidecar

This project provides a sidecar container to manage the [Open Policy Agent](http://openpolicyagent.org) on top of Kubernetes.

## Deployment Guide

### Hello World

1. Create a new Deployment that includes OPA and the kube-mgmt sidecar (`manifests/deployment-opa.yml`):

    ```bash
    kubectl -n opa create -f https://raw.githubusercontent.com/open-policy-agent/kube-mgmt/master/manifests/deployment-opa.yml
    ```

1. Define a simple policy (`example.rego`) with the following content:

    ```
    package kubernetes

    example = "Hello, Kubernetes!"
    ```

1. Create a ConfigMap containing the policy:

    ```bash
    kubectl -n opa create configmap hello-world --from-file example.rego
    ```

1. Add a label to the ConfigMap:

    ```bash
    kubectl -n opa label configmap hello-world openpolicyagent.org/policy=rego
    ```

    > By default, the sidecar synchronizes policies stored in ConfigMaps labeled with `openpolicyagent.org/policy=rego`.

1. Create a Service to expose OPA:

    ```bash
    kubectl -n opa expose deployment opa --type=NodePort
    ```

1. Execute a policy query against OPA:

    ```bash
    OPA_URL=$(minikube service -n opa opa --url)
    curl $OPA_URL/v1/data/kubernetes/example
    ```

## Admission Control

To use OPA as an [Admission
Controller](https://kubernetes.io/docs/admin/admission-controllers/#what-are-they)
in Kubernetes 1.7 or later, follow the steps in [External Admission
Webhooks](https://kubernetes.io/docs/admin/extensible-admission-controllers/#external-admission-webhooks)
to enable webhooks in the Kubernetes API server. Once you have configured the
Kubernetes API server and generated the necessary certificates you can start
`kube-mgmt` with the following options:

```bash
-enable-admission-control
-admission-ca-cert-file=/path/to/ca/cert.pem
-admission-service-name=<name-of-opa-service>
-admission-service-namespace=<namespace-of-opa-service>
```

You will need to create Secrets containing the server certificate and private
key as well as the CA certificate:

```bash
kubectl create secret generic opa-ca --from-file=ca.pem
kubectl create secret tls opa-server --cert=server.pem --key=server.key
```

> See [Generating TLS Certificates](#generating-tls-certificates) below for
> examples of how to generate the certificate files.

The example below shows how to deploy OPA and enable admission control:

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: opa
  name: opa
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: opa
      name: opa
    spec:
      containers:
        - name: opa
          image: openpolicyagent/opa:0.5.2
          args:
            - "run"
            - "--server"
            - "--tls-cert-file=/certs/tls.crt"
            - "--tls-private-key-file=/certs/tls.key"
            - "--addr=0.0.0.0:443"
            - "--insecure-addr=127.0.0.1:8181"
          volumeMounts:
            - readOnly: true
              mountPath: /certs
              name: opa-server
        - name: kube-mgmt
          image: openpolicyagent/kube-mgmt:0.3
          args:
            - "-opa=http://127.0.0.1:8181/v1"
            - "-enable-admission-control"
            - "-admission-ca-cert-file=/certs/ca.pem"
            - "-admission-service-name=opa"
            - "-admission-service-namespace=$(MY_POD_NAMESPACE)"
          volumeMounts:
            - readOnly: true
              mountPath: /certs
              name: opa-ca
          env:
            - name: MY_POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
      volumes:
        - name: opa-server
          secret:
            secretName: opa-server
        - name: opa-ca
          secret:
            secretName: opa-ca
---
kind: Service
apiVersion: v1
metadata:
  name: opa
spec:
  clusterIP: 10.0.0.222
  selector:
    app: opa
  ports:
  - name: https
    protocol: TCP
    port: 443
    targetPort: 443
```

You must create a policy that produces a document at `/data/system/main`. The
following policy will allow all operations:

```ruby
package system

main = {
  "apiVersion": "admission.k8s.io/v1alpha1",
  "kind": "AdmissionReview",
  "status": status
}

default status = {
  "allowed": true
}
```

### Example Policy

To test that the policy is working, define a status rule that rejects the
request if the `test-reject` label is found:

```
package system

status = reject {
  input.spec.operation = "CREATE"
  input.spec.object.labels["test-reject"]
}

reject = {
  "allowed": false,
  "status": {
    "reason": "testing rejection"
  }
}
```

### <a name="generating-tls-certificates" />Generating TLS Certificates

External Admission Controllers must be secured with TLS. At a minimum you must:

- Provide the Kubernetes API server with a client key to use for
  webhook calls.

- Provide OPA with a server key so that the Kubernetes API server can
  authenticate it.

- Provide `kube-mgmt` with the CA certificate to register with the Kubernetes
  API server.

Follow the steps below to generate the necessary files for test purposes.

First, generate create the required OpenSSL configuration files:

**client.conf**:

```
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = @alt_names
[alt_names]
IP.1 = 127.0.0.1
```

**server.conf**:

```
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = @alt_names
[alt_names]
IP.1 = 10.0.0.222
```

> The subjectAltName/IP address in the certificate MUST match the one configured
> on the Kubernetes Service.

Finally, generate the CA and client/server key pairs.

```bash
# Create a certificate authority
openssl genrsa -out caKey.pem 2048
openssl req -x509 -new -nodes -key caKey.pem -days 100000 -out caCert.pem -subj
"/CN=admission_ca"

# Create a server certiticate
openssl genrsa -out serverKey.pem 2048
openssl req -new -key serverKey.pem -out server.csr -subj "/CN=admission_server"
-config server.conf
openssl x509 -req -in server.csr -CA caCert.pem -CAkey caKey.pem -CAcreateserial
-out serverCert.pem -days 100000 -extensions v3_req -extfile server.conf

# Create a client certiticate
openssl genrsa -out clientKey.pem 2048
openssl req -new -key clientKey.pem -out client.csr -subj "/CN=admission_client"
-config client.conf
openssl x509 -req -in client.csr -CA caCert.pem -CAkey caKey.pem -CAcreateserial
-out clientCert.pem -days 100000 -extensions v3_req -extfile client.conf
```

## Initializers

Follow the steps in [Initializers]() to use OPA as an initialization controller
in Kubernetes 1.7 or later.

Once you have configured the Kubernetes API server, you can start `kube-mgmt`
with the following options:

```bash
# Enable initializer for given namespace-level resource.
# May be specified multiple times.
-initializer-namespace=<[group/]version/resource>

# Enable initializer for given cluster-level resource.
# May be specified multiple times.
-initializer-cluster=<[group/]version/resource>

# Set path of initialization doucment to query.
# Defaults to /kubernetes/admission/initialize
-initialization-path=<path-relative-to-/data>
```

The example below shows how to deploy OPA and enable initializers for
Deployments and Services:

```
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: opa
  name: opa
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: opa
      name: opa
    spec:
      containers:
        - name: opa
          image: openpolicyagent/opa:0.5.2
          args:
            - "run"
            - "--server"
        - name: kube-mgmt
          image: openpolicyagent/kube-mgmt:0.3
          args:
            - "-initializer-namespace=v1/services"
            - "-initializer-namespace=apps/v1beta1/deployments"
      volumes:
        - name: opa-server
          secret:
            secretName: opa-server
        - name: opa-ca
          secret:
            secretName: opa-ca
```

If initializers are enabled, `kube-mgmt` will register itself as an
initialization controller on the specified resource type (you do not have to
create a initializer configuration yourself.)

### Example Policy

The policy below will inject OPA into Deployments that indicate they **require OPA**:

```ruby
package kubernetes.admission

initialize = merge {
  input.kind = "Deployment"
  input.metadata.annotations["requires-opa"]
  merge = {
    "spec": {
      "template": {
        "spec": {
          "containers": [
            {
              "name": "opa",
              "image": "openpolicyagent/opa:0.5.2",
              "args": [
                "run",
                "--server",
              ]
            }
          ]
        }
      }
    }
  }
}
```

## Development Guide

To run all of the tests and build the Docker image run `make` in this directory.

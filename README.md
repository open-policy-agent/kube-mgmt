# ![logo](./logo/logo.png) kube-mgmt

Policy-based control for Kubernetes deployments.

## About

`kube-mgmt` manages instances of the [Open Policy Agent](https://github.com/open-policy-agent/opa) on top of Kubernetes. Use `kube-mgmt` to:

- Load policies into OPA (via Kubernetes APIs)
- Replicate Kubernetes resources into OPA
- Deploy OPA as an Admission Controller
- Deploy OPA as an Initializer

**NOTE**: `kube-mgmt` is currently in alpha. Join the discussion on [slack.openpolicyagent.org](http://slack.openpolicyagent.org).

## Deployment Guide

### Hello World

1. Create a new Namespace to deploy OPA into:

    ```bash
    kubectl create namespace opa
    ```

1. Create a new Deployment that includes OPA and `kube-mgmt` (`manifests/deployment.yml`):

    ```bash
    kubectl -n opa create -f https://raw.githubusercontent.com/open-policy-agent/kube-mgmt/master/manifests/deployment.yml
    ```

1. Define a simple policy (`example.rego`) with the following content:

    ```ruby
    package kubernetes

    example = "Hello, Kubernetes!"
    ```

1. Create a ConfigMap containing the policy:

    ```bash
    kubectl -n opa create configmap hello-world --from-file example.rego
    ```

1. Create a Service to expose OPA:

    ```bash
    kubectl -n opa expose deployment opa --type=NodePort
    ```

1. Execute a policy query against OPA:

    ```bash
    OPA_URL=$(minikube service -n opa opa --url)
    curl $OPA_URL/v1/data/kubernetes/example
    ```

### Policies

`kube-mgmt` automatically discovers policies stored in ConfigMaps in Kubernetes
and loads them into OPA. `kube-mgmt` assumes a ConfigMap contains policies if
the ConfigMap is:

- Created in a namespace listed in the --policies option.
- Labelled with `openpolicyagent.org/policy=rego`.

When a policy has been successfully loaded into OPA, the
`openpolicyagent.org/policy-status` annotation is set to `{"status": "ok"}`.

If loading fails for some reason (e.g., because of a parse error), the
`openpolicyagent.org/policy-status` annotation is set to `{"status": "error",
"error": ...}` where the `error` field contains details about the failure.

### Caching

`kube-mgmt` can be configured to replicate Kubernetes resources into OPA so that
you can express policies over an eventually consistent cache of Kubernetes
state.

Replication is enabled with the following options:

```bash
# Replicate namespace-level resources. May be specified multiple times.
--replicate=<[group/]version/resource>

# Replicate cluster-level resources. May be specified multiple times.
--replicate-cluster=<[group/]version/resource>
```

#### Example Options

The example below would replicate Deployments, Services, and Nodes into OPA:

```bash
--replicate=apps/v1beta/deployments
--replicate=v1/services
--replicate-cluster=v1/nodes
```

### Admission Control

To use OPA as an [Admission
Controller](https://kubernetes.io/docs/admin/admission-controllers/#what-are-they)
in Kubernetes 1.7 or later, follow the steps in [External Admission
Webhooks](https://kubernetes.io/docs/admin/extensible-admission-controllers/#external-admission-webhooks)
to enable webhooks in the Kubernetes API server. Once you have configured the
Kubernetes API server and generated the necessary certificates you can start
`kube-mgmt` with the following options:

```bash
--register-admission-controller
--admission-controller-ca-cert-file=/path/to/ca/cert.pem
--admission-controller-service-name=<name-of-opa-service>
--admission-controller-service-namespace=<namespace-of-opa-service>
```

You will need to create Secrets containing the server certificate and private
key as well as the CA certificate:

```bash
kubectl create secret generic opa-ca --from-file=ca.crt
kubectl create secret tls opa-server --cert=server.crt --key=server.key
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
          image: openpolicyagent/kube-mgmt:0.4
          args:
            - "--register-admission-controller"
            - "--admission-controller-ca-cert-file=/certs/ca.crt"
            - "--admission-controller-service-name=opa"
            - "--admission-controller-service-namespace=$(MY_POD_NAMESPACE)"
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

Admission control policies must produce a document at `/system/main` that
represents the admission control decision (i.e., allow or deny).

#### Example Policy

To test that admission control is working, define a policy that rejects the
request if the `test-reject` label is found:

```ruby
package system

main = {
  "apiVersion": "admission.k8s.io/v1alpha1",
  "kind": "AdmissionReview",
  "status": status,
}

default status = {"allowed": true}

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

#### <a name="generating-tls-certificates" />Generating TLS Certificates

External Admission Controllers must be secured with TLS. At a minimum you must:

- Provide the Kubernetes API server with a client key to use for
  webhook calls (`client.key` and `client.crt` below).

- Provide OPA with a server key so that the Kubernetes API server can
  authenticate it (`server.key` and `server.crt` below).

- Provide `kube-mgmt` with the CA certificate to register with the Kubernetes
  API server (`ca.crt` below).

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
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -days 100000 -out ca.crt -subj "/CN=admission_ca"

# Create a server certiticate
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr -subj "/CN=admission_server" -config server.conf
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days 100000 -extensions v3_req -extfile server.conf

# Create a client certiticate
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr -subj "/CN=admission_client" -config client.conf
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 100000 -extensions v3_req -extfile client.conf
```

If you are using minikube, you can specify the client TLS credentials with the following `minikube start` options:

```
--extra-config=apiserver.ProxyClientCertFile=/path/to/client.crt  # in VM
--extra-config=apiserver.ProxyClientKeyFile=/path/to/client.key   # in VM
```

### Initializers

To use OPA as an [Initializer](https://kubernetes.io/docs/admin/extensible-admission-controllers/#initializers) you must be running Kubernetes 1.7 or later.

Once you have configured the Kubernetes API server to enable initialization
controllers, you can start `kube-mgmt` with the following options:

```bash
# Enable initializer for given namespace-level resource.
# May be specified multiple times.
--initialize=<[group/]version/resource>

# Enable initializer for given cluster-level resource. May be specified multiple times.
--initialize-cluster=<[group/]version/resource>

# Set path of initialization document to query. Defaults to /kubernetes/admission/initialize.
--initialize-path=<path-relative-to-/data>
```

The example below shows how to deploy OPA and enable initializers for
Deployments and Services:

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
        - name: kube-mgmt
          image: openpolicyagent/kube-mgmt:0.4
          args:
            - "--initialize=v1/services"
            - "--initialize=apps/v1beta1/deployments"
```

If initializers are enabled, `kube-mgmt` will register itself as an
initialization controller on the specified resource type (you do not have to
create a initializer configuration yourself.)

#### Example Policy

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
              "image": "openpolicyagent/opa",
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

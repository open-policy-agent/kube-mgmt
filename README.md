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
            - "--addr=0.0.0.0:8181"
            - "--insecure-addr=127.0.0.1:8282"
          volumeMounts:
            - readOnly: true
              mountPath: /certs
              name: opa-server
        - name: kube-mgmt
          image: openpolicyagent/kube-mgmt:0.3
          args:
            - "-opa=http://127.0.0.1:8282/v1"
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
  selector:
    app: opa
  ports:
  - name: https
    protocol: TCP
    port: 8181
    targetPort: 8181
```

### <a name="generating-tls-certificates" />Generating TLS Certificates

<!-- TODO-->

## Development Guide

To run all of the tests and build the Docker image run `make` in this directory.

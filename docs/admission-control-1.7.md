# Admission Control (1.7 and 1.8)

**Note: Admission Control has undergone changes in Kubernetes 1.7 through 1.9. If you are running Kubernetes 1.9, see [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) instead.**

To use OPA as an [Admission Controller](https://kubernetes.io/docs/admin/admission-controllers/#what-are-they) in Kubernetes 1.7 or 1.8, follow the steps in [External Admission Webhooks](https://kubernetes.io/docs/admin/extensible-admission-controllers/#external-admission-webhooks) to enable webhooks in the Kubernetes API server. Once you have configured the Kubernetes API server and generated the necessary certificates you can start `kube-mgmt` with the following options:

```bash
--register-admission-controller
--admission-controller-ca-cert-file=/path/to/ca/cert.pem
--admission-controller-service-name=<name-of-opa-service>
--admission-controller-service-namespace=<namespace-of-opa-service>
```

In addition to the command line arguments above, you must provide `--pod-name` and `--pod-namespace` using [Kubernetes' Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/). The example manifest below shows how to set these.

You will need to create Secrets containing the server certificate and private key as well as the CA certificate:

```bash
kubectl create secret generic opa-ca --from-file=ca.crt
kubectl create secret tls opa-server --cert=server.crt --key=server.key
```

> See [Generating TLS Certificates](./tls-1.7.md) below for examples of how to generate the certificate files.

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
          image: openpolicyagent/opa
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
          image: openpolicyagent/kube-mgmt:0.6
          args:
            - "--pod-name=$(MY_POD_NAME)"
            - "--pod-namespace=$(MY_POD_NAMESPACE)"
            - "--register-admission-controller"
            - "--admission-controller-ca-cert-file=/certs/ca.crt"
            - "--admission-controller-service-name=opa"
            - "--admission-controller-service-namespace=$(MY_POD_NAMESPACE)"
          volumeMounts:
            - readOnly: true
              mountPath: /certs
              name: opa-ca
          env:
            - name: MY_POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
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


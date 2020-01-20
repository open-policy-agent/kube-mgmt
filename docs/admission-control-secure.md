# Admission Control Secure

In the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial we have seen how OPA can be deployed as an admission controller. In that tutorial, OPA is not configured to `authenticate` and `authorize` client requests.

## Goal

This tutorial will show how to securely deploy OPA as an admission controller. The additional steps that need to be taken to achieve this are:

1. Start `OPA` with authentication and authorization enabled using the `--authentication` and `--authorization` options respectively.
2. Volume mount OPA's startup authorization policy into the OPA container.
3. Start `kube-mgmt` with `Bearer` token flag using the `--opa-auth-token-file` option.
4. Configure `kube-mgmt` to load polices stored in ConfigMaps that are created in the `opa` namespace and are labelled `openpolicyagent.org/policy=rego`. This is enforced using the `--require-policy-label=true` option.
5. Configure the Kubernetes API server to use `Bearer` token.


## Prerequisites

Same as the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial.

## Steps

### 1. Configure Kubernetes API server

OPA will `authenticate` clients by extracting the `Bearer` token from the incoming API requests. Hence the Kubernetes API server needs to be configured to send a `Bearer` token in all requests to OPA.
To do this, the API server must be provided with an admission control configuration file via the `--admission-control-config-file` flag during startup. This means the configuration file should be present inside the minikube VM at a location which is accessible to the API server pod.

Start minikube:

```bash
minikube start
```

`ssh` into the minikube VM and place the configuration files (**admission-control-config.yaml** and **kube-config.yaml**) below inside `/var/lib/minikube/certs`. This directory is accessible inside the API server pod.

**admission-control-config.yaml**

```yaml
apiVersion: apiserver.k8s.io/v1alpha1
kind: AdmissionConfiguration
plugins:
- name: ValidatingAdmissionWebhook
  configuration:
    apiVersion: apiserver.config.k8s.io/v1alpha1
    kind: WebhookAdmission
    kubeConfigFile: /var/lib/minikube/certs/kube-config.yaml
```

**kube-config.yaml**

```yaml
apiVersion: v1
kind: Config
users:
# '*' is the default match.
- name: '*'
  user:
    token: <apiserver_secret_token>
```

With the above configuration, all requests the API server makes to OPA will include a `Bearer` token. You will need to generate the `Bearer` token (`<apiserver_secret_token>`) and later include it in OPA's startup authorization policy so that OPA can verify the identity of the API server.

Now exit the minikube VM and stop it:

```bash
minikube stop
```

Start minikube by passing information about the admission control configuration file to the API server:

```bash
minikube start --extra-config=apiserver.admission-control-config-file=/var/lib/minikube/certs/admission-control-config.yaml
```

Make sure that the minikube ingress addon is enabled:

```bash
minikube addons enable ingress
```

Follow the steps in the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial to create the `opa` namespace and configure TLS. Now use the **admission-controlller.yaml** file from the tutorial to deploy OPA as an admission controller with the following changes:

1. Use the below `opa` and `kube-mgmt` container spec which enables OPA's security features and configures `kube-mgmt` to include a `Bearer` token in calls to OPA. We also volume mount OPA's startup authorization policy `authz.rego` inside the OPA container in the `/policies` directory.

```yaml
spec:
  containers:
    - name: opa
      image: openpolicyagent/opa:0.10.0
      args:
        - "run"
        - "--server"
        - "--tls-cert-file=/certs/tls.crt"
        - "--tls-private-key-file=/certs/tls.key"
        - "--addr=0.0.0.0:443"
        - "--addr=http://127.0.0.1:8181"
        - "--authentication=token"
        - "--authorization=basic"
        - "/policies/authz.rego" # authorization policy used on startup
        - "--ignore=.*"          # exclude hidden dirs created by Kubernetes
      volumeMounts:
        - readOnly: true
          mountPath: /certs
          name: opa-server
        - readOnly: true
          mountPath: /policies
          name: inject-policy
    - name: kube-mgmt
      image: openpolicyagent/kube-mgmt:0.8
      args:
        - "--replicate-cluster=v1/namespaces"
        - "--replicate=extensions/v1beta1/ingresses"
        - "--opa-auth-token-file=/policies/token"
        - "--require-policy-label=true"
      volumeMounts:
        - readOnly: true
          mountPath: /policies
          name: inject-policy
  volumes:
    - name: opa-server
      secret:
        secretName: opa-server
    - name: inject-policy
      secret:
        secretName: inject-policy
```

2. Include the Secret that contains OPA's startup authorization policy.

```bash
cat > authz.rego <<EOF
package system.authz

default allow = false

allow {
  "kube-mgmt" = input.identity
}

allow {
  <apiserver_secret_token> = input.identity
}
EOF

kubectl create secret generic inject-policy -n opa --from-file=authz.rego --from-literal=token=kube-mgmt

```

If you have liveness or readiness probes configured on the OPA server for `/health` you will need to add the following `allow` rule to ensure Kubernetes can still access these endpoints. 

```
# Allow anonymouse access to /health otherwise K8s get 403 and kills pod. 
allow {
    input.path = ["health"]
}
```

3. Label the `opa-default-system-main` ConfigMap.

```yaml
---
kind: ConfigMap
apiVersion: v1

metadata:
  name: opa-default-system-main
  namespace: opa
  labels:
    openpolicyagent.org/policy: rego
data:
  main: |
    package system

    import data.kubernetes.admission

    main = {
      "apiVersion": "admission.k8s.io/v1beta1",
      "kind": "AdmissionReview",
      "response": response,
    }

    default response = {"allowed": true}

    response = {
        "allowed": false,
        "status": {
            "reason": reason,
        },
    } {
        reason = concat(", ", admission.deny)
        reason != ""
    }
```

When OPA starts, the `kube-mgmt` container will load Kubernetes Namespace and Ingress objects into OPA. `kube-mgmt` will automatically discover policies stored in ConfigMaps in Kubernetes
and load them into OPA. `kube-mgmt` assumes a ConfigMap contains policies if
the ConfigMap is:

- Created in a namespace listed in the --policies option. Default namespace is `opa`.
- Labelled with `openpolicyagent.org/policy=rego`.

`kube-mgmt` is started with the `--opa-auth-token-file` flag and hence all requests made to OPA will include a `Bearer` token (`kube-mgmt` in this case).

You can now follow the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial to deploy OPA on top of Kubernetes and test admission control. **Make sure to label the ConfigMap when you store a policy inside it.**

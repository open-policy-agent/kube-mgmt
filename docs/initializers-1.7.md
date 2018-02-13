# Initializers (1.7)

To use OPA as an [Initializer](https://kubernetes.io/docs/admin/extensible-admission-controllers/#initializers) you must be running Kubernetes 1.7. Keep in mind that Initializers are currently in **alpha**.

Once you have configured the Kubernetes API server to enable initialization controllers, you can start `kube-mgmt` with the following options:

```bash
# Enable initializer for given namespace-level resource.
# May be specified multiple times.
--initialize=<[group/]version/resource>

# Enable initializer for given cluster-level resource. May be specified multiple times.
--initialize-cluster=<[group/]version/resource>

# Set path of initialization document to query. Defaults to /kubernetes/admission/initialize.
--initialize-path=<path-relative-to-/data>
```

In addition to the command line arguments above, you must provide `--pod-name` and `--pod-namespace` using [Kubernetes' Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/). The example manifest below shows how to set these.

The example below shows how to deploy OPA and enable initializers for Deployments and Services:

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
        - name: kube-mgmt
          image: openpolicyagent/kube-mgmt:0.6
          args:
            - "--pod-name=$(MY_POD_NAME)"
            - "--pod-namespace=$(MY_POD_NAMESPACE)"
            - "--initialize=v1/services"
            - "--initialize=apps/v1beta1/deployments"
          env:
            - name: MY_POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: MY_POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
```

If initializers are enabled, `kube-mgmt` will register itself as an initialization controller on the specified resource type (you do not have to create a initializer configuration yourself.)

## Example Policy

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

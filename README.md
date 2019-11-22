# ![logo](./logo/logo.png) kube-mgmt

Policy-based control for Kubernetes deployments.

## About

`kube-mgmt` manages instances of the [Open Policy Agent](https://github.com/open-policy-agent/opa) on top of Kubernetes. Use `kube-mgmt` to:

- Load policies into OPA via Kubernetes (see [Policies](#policies) below.)
- Replicate Kubernetes resources including [CustomResourceDefinitions (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions) into OPA (see [Caching](#caching) below.)

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

## Policies

`kube-mgmt` automatically discovers policies stored in ConfigMaps in Kubernetes
and loads them into OPA. `kube-mgmt` assumes a ConfigMap contains policies if
the ConfigMap is:

- Created in a namespace listed in the `--policies` option. If you specify `--policies=*` then `kube-mgmt` will look for policies in ALL namespaces.
- Labelled with `openpolicyagent.org/policy=rego`.

When a policy has been successfully loaded into OPA, the
`openpolicyagent.org/policy-status` annotation is set to `{"status": "ok"}`.

If loading fails for some reason (e.g., because of a parse error), the
`openpolicyagent.org/policy-status` annotation is set to `{"status": "error",
"error": ...}` where the `error` field contains details about the failure.

### JSON Loading

`kube-mgmt` can be configured to load arbitrary JSON out of ConfigMaps into
OPA's data namespace. This is useful for providing contextual information to
your policies.

Enable data loading by specifying the `--enable-data` command-line flag to
`kube-mgmt`. If data loading is enabled `kube-mgmt` will load JSON out of
ConfigMaps labelled with `openpolicyagent.org/data=opa`.

Data loaded out of ConfigMaps is layed out as follows:

```
<namespace>/<name>/<key>
```

For example, if the following ConfigMap was created:

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: hello-data
  namespace: opa
  labels:
    openpolicyagent.org/data: opa
data:
  x.json: |
    {"a": [1,2,3,4]}
```

You could refer to the data inside your policies as follows:

```ruby
data.opa["hello-data"]["x.json"].a[0]  # evaluates to 1
```

## Caching

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

Kubernetes resources replicated into OPA are layed out as follos:

```
<replicate-path>/<resource>/<namespace>/<name> # namespace scoped
<replicate-path>/<resource>/<name>             # cluster scoped
```

- `<replicate-path>` is configurable (via `--replicate-path`) and
  defaults to `kubernetes`.
- `<resource>` is the Kubernetes resource plural, e.g., `nodes`,
  `pods`, `services`, etc.
- `<namespace>` is the namespace of the Kubernetes resource.
- `<name>` is the name of the Kubernetes resource.

For example, to search for services with the label `"foo"` you could write:

```
some namespace, name
service := data.kubernetes.services[namespace][name]
service.metadata.labels["foo"]
```

An alternative way to visualize the layout is as single JSON document:

```
{
	"kubernetes": {
		"services": {
			"default": {
				"example-service": {...},
				"another-service": {...},
				...
			},
			...
		},
		...
}
```

The example below would replicate Deployments, Services, and Nodes into OPA:

```bash
--replicate=apps/v1beta/deployments
--replicate=v1/services
--replicate-cluster=v1/nodes
```

### Custom Resource Definitions (CRDs)

`kube-mgmt` can also be configured to replicate [Kubernetes Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) using the `--replicate` and `--replicate-cluster` options. For an example of how OPA can be used to enforce admission control polices on Kubernetes custom resources see [Admission Control For Custom Resources](./docs/admission-control-crd.md)

## Admission Control

To get started with admission control policy enforcement in Kubernetes 1.9 or later see the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial. For older versions of Kubernetes, see [Admission Control (1.7)](./docs/admission-control-1.7.md).

In the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial, OPA is **NOT** running with an authorization policy configured and hence clients can read and write policies in OPA. When deploying OPA in an insecure environment, it is recommended to configure `authentication` and `authorization` on the OPA daemon. For an example of how OPA can be securely deployed as an admission controller see [Admission Control Secure](./docs/admission-control-secure.md).

## Development Guide

To run all of the tests and build the Docker image run `make` in this directory.

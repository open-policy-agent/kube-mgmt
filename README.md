# ![logo](./logo/logo.png) kube-mgmt

Policy-based control for Kubernetes deployments.

## About

`kube-mgmt` manages instances of the [Open Policy Agent](https://github.com/open-policy-agent/opa) on top of Kubernetes. Use `kube-mgmt` to:

- Load policies into OPA via Kubernetes (see [Policies](#policies) below.)
- Replicate Kubernetes resources into OPA (see [Caching](#caching) below.)

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

## Policies

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

#### Example Options

The example below would replicate Deployments, Services, and Nodes into OPA:

```bash
--replicate=apps/v1beta/deployments
--replicate=v1/services
--replicate-cluster=v1/nodes
```

## Admission Control

To get started with admission control policy enforcement in Kubernetes 1.9 or later see the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial. For older versions of Kubernetes, see [Admission Control (1.7)](./docs/admission-control-1.7.md).

## Development Guide

To run all of the tests and build the Docker image run `make` in this directory.

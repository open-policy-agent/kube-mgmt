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
    kubectl -n opa label configmap hello-world org.openpolicyagent/policy=rego
    ```

    > By default, the sidecar synchronizes policies stored in ConfigMaps labeled with `org.openpolicyagent/policy=rego`.

1. Create a Service to expose OPA:

    ```bash
    kubectl -n opa expose deployment opa --type=NodePort
    ```

1. Execute a policy query against OPA:

    ```bash
    OPA_URL=$(minikube service -n opa opa --url)
    curl $OPA_URL/v1/data/kubernetes/example
    ```

## Development Guide

To run all of the tests and build the Docker image just run `make` in this directory.

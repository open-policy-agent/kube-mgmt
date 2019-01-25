# Admission Control For Custom Resources

In the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial we have seen how OPA can be deployed as an admission controller to enforce custom policies on Kubernetes objects. In that tutorial, policies were enforced on native Kubernetes objects such as ingresses.

## Goal

This tutorial will show how OPA can be used to enforce polices on custom resources. A custom resource is an extension of the Kubernetes API that is not necessarily available on every Kubernetes cluster. More inforation on Kubernetes custom resources is available [here](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

The additional steps that need to be taken to achieve this are:

1. Define a role for reading Kubernetes custom resources.
2. Grant OPA/kube-mgmt permissions to read Kubernetes custom resources.
3. Configure `kube-mgmt` to load Kubernetes custom resources into OPA.

## Prerequisites

Same as the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial.

## Steps

### 1. Start minikube

```bash
minikube start
```

Follow the steps in the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial to create the `opa` namespace and configure TLS.

### 2. Create a CustomResourceDefinition

Save the following CustomResourceDefinition to **resourcedefinition.yaml**:

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: cats.opa.example.com
spec:
  group: opa.example.com
  version: "v1"
  scope: Namespaced
  names:
    plural: cats
    singular: cat
    kind: Cat
    shortNames:
    - ct
```

And create it:

```bash
kubectl create -f resourcedefinition.yaml
```

### 3. Deploy OPA on top of Kubernetes

Use the **admission-controlller.yaml** file from the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial to deploy OPA as an admission controller with the following changes:

1. Define a role for reading the Kubernetes custom resource created in the previous step.

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: crd-reader
rules:
- apiGroups: ["opa.example.com"]
  resources: ["cats"]
  verbs: ["get", "list", "watch"]
```

2. Grant OPA/kube-mgmt permissions to the above role.

```yaml
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: opa-crd-reader
roleRef:
  kind: ClusterRole
  name: crd-reader
  apiGroup: rbac.authorization.k8s.io
subjects:
- kind: Group
  name: system:serviceaccounts:opa
  apiGroup: rbac.authorization.k8s.io
```

3. Update the `kube-mgmt` container spec to load the Kubernetes custom resources into OPA.

```yaml
name: kube-mgmt
args:
    - "--replicate=opa.example.com/v1/cats"        # replicate custom resources
```

Now follow the [Kubernetes Admission Control](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html) tutorial to deploy OPA on top of Kubernetes and register OPA as an admission controller.

### 4. Define a policy and load it into OPA via Kubernetes

Create a policy that rejects objects of kind `Cat` from sharing the same cat name.

**name-conflicts.rego**:

```ruby
package kubernetes.admission

import data.kubernetes.cats

# Cat names must be unique.
deny[msg] {
    input.request.kind.kind = "Cat"
    input.request.operation = "CREATE"
    name := input.request.object.spec.name
    cat := cats[other_ns][other_cat]
    cat.spec.name == name
    msg = sprintf("duplicate cat name %q (conflicts with %v/%v)", [name, other_ns, other_cat])
}
```

```bash
kubectl create configmap name-conflicts --from-file=name-conflicts.rego
```

### 5. Exercise the policy

Define two objects of kind `Cat`. The first one will be permitted and the second will be rejected.

**cat.yaml**:

```yaml
apiVersion: "opa.example.com/v1"
kind: Cat
metadata:
  name: my-new-cat-object
spec:
  name: Whiskers
```

**cat-duplicate.yaml**:

```yaml
apiVersion: "opa.example.com/v1"
kind: Cat
metadata:
  name: my-duplicate-cat-object
spec:
  name: Whiskers
```

Finally, try to create both `Cat` objects:

```bash
kubectl create -f cat.yaml
kubectl create -f cat-duplicate.yaml
```

The second object will be rejected since an object with the cat name `Whiskers` was created earlier.

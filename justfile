export COMMIT := `git rev-parse --short HEAD`
export VERSION := "0.0.0-" + COMMIT
export E2E_TEST := "default"

skaffoldTags := "tags.json"

defaul:
    @just --list

@_skaffold-ctx:
    skaffold config set default-repo localhost:5000

# build and publish image to release regisry, create chart archive
build-release:
    #!/usr/bin/env bash
    set -euxo pipefail
    skaffold build -b kube-mgmt -t {{VERSION}} --file-output={{skaffoldTags}}

    LATEST="$(jq -r .builds[0].imageName {{skaffoldTags}}):latest"
    CURRENT="$(jq -r .builds[0].tag {{skaffoldTags}})"

    docker tag $CURRENT $LATEST
    docker push $LATEST

    helm package charts/opa --version {{VERSION}} --app-version {{VERSION}}

test-go:
    go test ./...
    go vet ./...
    staticcheck ./...

test-helm-lint:
    ./test/linter/test.sh

# run unit tests
test: test-go test-helm-lint

# (re) create local k8s cluster using k3d
k3d: && _skaffold-ctx
    k3d cluster delete kube-mgmt || true
    k3d cluster create --config ./test/e2e/k3d.yaml

# build and publish docker to local registry
build: _skaffold-ctx
    skaffold build --file-output={{skaffoldTags}} --platform=linux/amd64

# install into local k8s
up: _skaffold-ctx down
    skaffold deploy --build-artifacts={{skaffoldTags}}

# remove from local k8s
down:
    skaffold delete || true

# run only e2e test script
test-e2e-sh:
    kubectl delete cm -l kube-mgmt/e2e=true || true
    ./test/e2e/{{E2E_TEST}}/test.sh

# run single e2e test
test-e2e: up test-e2e-sh

# run all e2e tests
test-e2e-all: build
    #!/usr/bin/env bash
    set -euxo pipefail
    for E in $(find test/e2e/ -mindepth 1 -maxdepth 1 -type d -printf '%f\n'); do
        just E2E_TEST=${E} test-e2e
    done

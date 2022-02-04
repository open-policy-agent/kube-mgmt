export COMMIT := `git rev-parse --short HEAD`
export VERSION := "0.0.0-" + COMMIT

default:
    @just --list

@skaffold-ctx:
    skaffold config set default-repo localhost:5000 -k k3d-kube-mgmt

# build docker image and pack helm chart
@build: skaffold-ctx
    skaffold build -t {{VERSION}} --file-output=skaffold.json
    helm package charts/opa --version {{VERSION}} --app-version {{VERSION}}

# same as build but also pushes `latest` tag
build-latest: build
    #!/usr/bin/env bash
    set -euxo pipefail
    LATEST="$(jq -r .builds[0].imageName skaffold.json):latest"
    CURRENT="$(jq -r .builds[0].tag skaffold.json)"
    docker tag $CURRENT $LATEST
    docker push $LATEST

@test-helm:
    ./test/linter/test.sh

@test-e2e:
    ./test/e2e/test.sh

# run all tests
test: test-helm test-e2e

# (re) create local k8s cluster using k3d
@k3d: && skaffold-ctx
    k3d cluster delete kube-mgmt || true
    k3d cluster create --config ./test/e2e/k3d.yaml

# render k8s manifests
@template:
    skaffold render -a skaffold.json

# deploy chart to local k8s
@up: skaffold-ctx
    skaffold run

# delete chart from local k8s
@down:
    skaffold delete || true


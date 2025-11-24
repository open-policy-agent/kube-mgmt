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
  set -euo pipefail

  skaffold build -b kube-mgmt -t {{VERSION}}
  helm package charts/opa-kube-mgmt --version {{VERSION}} --app-version {{VERSION}}

_latest:
  #!/usr/bin/env bash
  set -euo pipefail

  if [ -n "$(echo ${SKAFFOLD_IMAGE_REPO}|grep '^openpolicyagent$')" ]; then
    crane tag ${SKAFFOLD_IMAGE} latest
  fi

_helm-unittest:
  helm plugin ls | grep unittest || helm plugin install https://github.com/helm-unittest/helm-unittest --version v1.0.3

# golang linter
lint-go:
  go vet ./...
  staticcheck ./...

# helm linter
lint-helm filter="*": _helm-unittest
  helm unittest -f '../../test/lint/{{filter}}.yaml' charts/opa-kube-mgmt

# run all unit tests
lint: lint-go lint-helm

# run helm unit tests
test-helm filter="*": _helm-unittest
  helm unittest -f '../../test/unit/{{filter}}.yaml' charts/opa-kube-mgmt

# run golang unit tests
test-go:
  go test ./...

# run unit tests
test: lint test-go test-helm

# (re) create local k8s cluster using k3d
k3d: && _skaffold-ctx
  k3d cluster delete kube-mgmt || true
  k3d cluster create --config ./test/e2e/k3d.yaml

rebuild: && build
  rm -rf {{skaffoldTags}}

# build and publish docker to local registry
build: _skaffold-ctx
  skaffold build --file-output={{skaffoldTags}} --platform=linux/amd64

# install into local k8s
up: _skaffold-ctx down
  #!/usr/bin/env bash
  set -euo pipefail

  if [ ! -f "{{skaffoldTags}}" ]; then
    echo 'Run `just build` to build docker image'
    exit 1
  fi

  kubectl delete cm -l kube-mgmt/e2e=true || true
  skaffold deploy --build-artifacts={{skaffoldTags}}

# remove from local k8s
down:
  skaffold delete || true

# run only e2e test script
test-e2e-sh:
  #!/usr/bin/env bash
  set -euo pipefail

  kubectl delete cm -l kube-mgmt/e2e=true || true
  kubectl delete svc -l kube-mgmt/e2e=true || true
  ./test/e2e/{{E2E_TEST}}/test.sh

# run e2e test before script
test-e2e-before:
  ./test/e2e/{{E2E_TEST}}/before.sh || true

# run single e2e test
test-e2e: test-e2e-before up test-e2e-sh

# run all e2e tests
test-e2e-all: build
  #!/usr/bin/env bash
  set -euo pipefail

  for E in $(find test/e2e/ -mindepth 1 -maxdepth 1 -type d -printf '%f\n'|grep -E -v '^skip_'|sort); do
    echo "===================================================="
    echo "= Running e2e: \`${E}\` "
    echo "===================================================="
    just E2E_TEST=${E} test-e2e
  done

K3D := "kube-mgmt"
TEST_RESULTS := 'build/test-results'

defaul:
  @just --list

# golang linter
lint-go:
  go vet ./...
  staticcheck ./...

# helm linter
lint-helm filter="*":
  #!/usr/bin/env -S bash -euo pipefail

  mkdir -p {{TEST_RESULTS}}/helm-unittest

  helm unittest -f '../../test/lint/{{filter}}.yaml' \
    --output-file {{TEST_RESULTS}}/helm-unittest/lint.xml --output-type JUnit charts/opa-kube-mgmt

# run all unit tests
lint: lint-go lint-helm

# run helm unit tests
test-helm filter="*":
  #!/usr/bin/env -S bash -euo pipefail

  mkdir -p {{TEST_RESULTS}}/helm-unittest

  helm unittest -f '../../test/unit/{{filter}}.yaml' \
    --output-file {{TEST_RESULTS}}/helm-unittest/unit.xml --output-type JUnit charts/opa-kube-mgmt

# run golang unit tests
test-go:
  go test ./...

# run unit tests
test: lint test-go test-helm

# start kube-mgmt in local k8s cluster
@up: _ctx
  devspace deploy --var E2E_TEST=test/e2e/default

# stop kube-mgmt in local k8s cluster
@down: _ctx
  devspace purge --force-purge && rm -rf .devspace/

@_token:
  kubectl exec deploy/kube-mgmt-opa-kube-mgmt -n default -c mgmt -- cat /bootstrap/mgmt-token

# run e2e test using chainsaw and hurl
test-e2e E2E_TEST="": _ctx
  #!/usr/bin/env -S bash -euo pipefail

  SCENARIO="{{E2E_TEST}}"
  if [ -z "$SCENARIO" ]; then
    SCENARIO=$(find test/e2e/ -mindepth 1 -maxdepth 1 -type d | sort | fzf --header "Select e2e scenario")
  fi

  devspace purge
  devspace deploy --var E2E_TEST="$SCENARIO"

  mkdir -p {{TEST_RESULTS}}/chainsaw

  OPA_TOKEN=$(just _token 2>/dev/null || true) chainsaw test "$SCENARIO" --quiet --namespace default \
    --report-format JUNIT-TEST \
    --report-name "$(basename "$SCENARIO")" --report-path {{TEST_RESULTS}}/chainsaw

# run all e2e tests
test-e2e-all:
  #!/usr/bin/env -S bash -euo pipefail

  for E in $(find test/e2e/ -name 'chainsaw-test.yaml'|xargs -n1 dirname); do
    just test-e2e "${E}"
  done

@_ctx:
  kubectl config use-context k3d-{{K3D}}

_bundle:
  #!/usr/bin/env -S bash -euo pipefail

  opa build -b ./test/e2e/replicate_auto/bundle -o ./test/e2e/replicate_auto/bundle.tar.gz
  kubectl delete configmap -n default bundle --ignore-not-found
  kubectl create configmap -n default bundle --from-file ./test/e2e/replicate_auto/bundle.tar.gz

# (re) create local cluster
all: && _ctx _bundle down
  #!/usr/bin/env -S bash -euo pipefail

  k3d cluster delete {{K3D}} || true

  echo '
  apiVersion: k3d.io/v1alpha5
  kind: Simple
  metadata:
    name: {{K3D}}
  servers: 1
  agents: 0
  image: rancher/k3s:v1.33.9-k3s1
  registries:
    create:
      name: k3d-{{K3D}}-registry
      host: "0.0.0.0"
      hostPort: "5001"
    config: |
      mirrors:
        "localhost:5001":
          endpoint:
            - http://k3d-{{K3D}}-registry:5000
  ports:
    - port: 8080:80
      nodeFilters: ["loadbalancer"]
    - port: 8443:443
      nodeFilters: ["loadbalancer"]
  options:
    k3s:
      extraArgs:
        - arg: "--disable=local-storage,metrics-server"
          nodeFilters: ["server:*"]
  ' | k3d cluster create --config /dev/stdin

  kubectl config set-context k3d-{{K3D}} --namespace default

  docker login -u {{K3D}} -p {{K3D}} localhost:5001

  kubectl wait --for=create crd/ingressroutetcps.traefik.io --timeout=2m
  sleep 3
  kubectl wait --for=condition=Established crd/ingressroutetcps.traefik.io --timeout=1s


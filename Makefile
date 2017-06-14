BIN := kube-mgmt
PKG := github.com/open-policy-agent/kube-mgmt
REGISTRY ?= openpolicyagent
VERSION := 0.2
ARCH := amd64
COMMIT := $(shell ./build/get-build-commit.sh)

IMAGE := $(REGISTRY)/$(BIN)

BUILD_IMAGE ?= golang:1.8-alpine

.PHONY: all
all: image

.PHONY: build
build:
	docker run -it \
		-v $$(pwd)/.go:/go \
		-v $$(pwd):/go/src/$(PKG) \
		-v $$(pwd)/bin/linux_$(ARCH):/go/bin \
		-v $$(pwd)/.go/std/$(ARCH):/usr/local/go/pkg/linux_$(ARCH)_static \
		-w /go/src/$(PKG) \
		$(BUILD_IMAGE) \
		/bin/sh -c "ARCH=$(ARCH) VERSION=$(VERSION) COMMIT=$(COMMIT) PKG=$(PKG) ./build/build.sh"

.PHONY: image
image: build
	docker build -t $(IMAGE):$(VERSION) -f Dockerfile .
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest

.PHONY: clean
clean:
	rm -fr bin .go

.PHONY: undeploy
undeploy:
	kubectl delete -f manifests/deployment-opa.yml || true

.PHONY: deploy
deploy:
	kubectl create -f manifests/deployment-opa.yml

.PHONY: up
up: image undeploy deploy

.PHONY: push
push:
	docker push $(IMAGE):$(VERSION)

.PHONY: push-latest
push-latest:
	docker push $(IMAGE):latest

.PHONY: version
version:
	@echo $(VERSION)

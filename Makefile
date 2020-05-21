BIN := kube-mgmt
PKG := github.com/open-policy-agent/kube-mgmt
REGISTRY ?= pietervicloudcom
VERSION := 0.12-dev
ARCH := amd64
OS := linux
COMMIT := $(shell ./build/get-build-commit.sh)

IMAGE := $(REGISTRY)/$(BIN)

BUILD_IMAGE ?= golang:1.12-alpine

.PHONY: all
all: image

.PHONY: build
build:
	docker run -it \
		-v $$(pwd)/.go:/go \
		-v $$(pwd):/go/src/$(PKG) \
		-v $$(pwd)/bin/$(OS)_$(ARCH):/go/bin \
		-v $$(pwd)/.go/std/$(ARCH):/usr/local/go/pkg/linux_$(ARCH)_static \
		-w /go/src/$(PKG) \
		$(BUILD_IMAGE) \
		/bin/sh -c "OS=$(OS) ARCH=$(ARCH) VERSION=$(VERSION) COMMIT=$(COMMIT) PKG=$(PKG) ./build/build.sh"

.PHONY: build-linux-amd64
build-linux-amd64:
	make build OS=linux ARCH=amd64

.PHONY: build-linux-armv6
build-linux-armv6:
	make build OS=linux ARCH=arm
	mkdir -pv $$(pwd)/bin/linux_armv6
	cp $$(pwd)/bin/linux_arm/linux_arm/* $$(pwd)/bin/linux_armv6
	rm -rf $$(pwd)/bin/linux_arm

.PHONY: image
image: build-linux-amd64 build-linux-armv6 
	docker buildx build -t $(IMAGE):$(VERSION)-linux-amd64 -f Dockerfile --platform linux/amd64 --build-arg OS=linux --build-arg ARCH=amd64 .
	docker buildx build -t $(IMAGE):$(VERSION)-linux-armv6 -f Dockerfile --platform linux/arm/v6 --build-arg OS=linux --build-arg ARCH=armv6 .

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
	docker push $(IMAGE):$(VERSION)-linux-amd64
	docker push $(IMAGE):$(VERSION)-linux-armv6

	docker manifest create $(IMAGE):$(VERSION) $(IMAGE):$(VERSION)-linux-amd64 $(IMAGE):$(VERSION)-linux-armv6
	docker manifest push --purge $(IMAGE):$(VERSION)

.PHONY: push-latest
push-latest:
	docker manifest create $(IMAGE):latest $(IMAGE):$(VERSION)-linux-amd64 $(IMAGE):$(VERSION)-linux-armv6
	docker manifest push --purge $(IMAGE):latest

.PHONY: version
version:
	@echo $(VERSION)

#!/bin/sh

set -ex

go install -ldflags "-X ${PKG}/pkg/version.Version=${VERSION} -X ${PKG}/pkg/version.Git=${COMMIT}" ./cmd/kube-mgmt/.../

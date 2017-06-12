#!/bin/sh

set -ex

GOARCH=${ARCH} go install -ldflags "-X ${PKG}/version/version.Version=${VERSION} -X ${PKG}/version/version.Git=${COMMIT}" ./cmd/kube-mgmt/.../

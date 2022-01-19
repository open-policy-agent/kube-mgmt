FROM golang:1.17.6-alpine as build

ARG ARCH=amd64
ARG PKG=github.com/open-policy-agent/kube-mgmt
ARG VERSION=local
ARG COMMIT=local

WORKDIR /go/src/${PKG}

COPY . .

RUN set +x && \
    export GOARCH=${ARCH} && \
    go install -ldflags "-X ${PKG}/pkg/version.Version=${VERSION} -X ${PKG}/pkg/version.Git=${COMMIT}" ./cmd/kube-mgmt/.../

FROM alpine:3.12.3

MAINTAINER Torin Sandall torinsandall@gmail.com

COPY --from=build /go/bin/kube-mgmt /kube-mgmt

USER 1000

ENTRYPOINT ["/kube-mgmt"]

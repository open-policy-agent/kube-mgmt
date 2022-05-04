FROM golang:1.17.7-alpine as build

RUN if [ `uname -m` = "aarch64" ] ; then \
       ARCH="arm64"; \
    else \
       ARCH="amd64"; \
    fi
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

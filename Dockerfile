FROM golang:1.24 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w \
      -X github.com/open-policy-agent/kube-mgmt/pkg/version.Version=${VERSION} \
      -X github.com/open-policy-agent/kube-mgmt/pkg/version.Git=${COMMIT}" \
    -o /kube-mgmt \
    ./cmd/kube-mgmt

FROM alpine:3.20.8
COPY --from=builder /kube-mgmt /kube-mgmt
ENTRYPOINT ["/kube-mgmt"]

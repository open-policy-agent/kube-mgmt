FROM alpine

MAINTAINER Torin Sandall torinsandall@gmail.com

ARG OS=linux
ARG ARCH=amd64

ADD bin/${OS}_${ARCH}/kube-mgmt /kube-mgmt

USER 1000

ENTRYPOINT ["/kube-mgmt"]

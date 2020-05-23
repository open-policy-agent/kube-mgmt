FROM alpine

MAINTAINER Torin Sandall torinsandall@gmail.com

ARG SOURCE=bin/linux_amd64/

ADD ${SOURCE}/kube-mgmt /kube-mgmt

USER 1000

ENTRYPOINT ["/kube-mgmt"]

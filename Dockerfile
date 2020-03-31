FROM alpine:3.11.5

MAINTAINER Torin Sandall torinsandall@gmail.com

ADD bin/linux_amd64/kube-mgmt /kube-mgmt

USER 1000

ENTRYPOINT ["/kube-mgmt"]

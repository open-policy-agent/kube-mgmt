FROM alpine

MAINTAINER Torin Sandall torinsandall@gmail.com

ADD bin/linux_amd64/kube-mgmt /kube-mgmt

ENTRYPOINT ["/kube-mgmt"]
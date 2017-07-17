// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Sirupsen/logrus"
	"github.com/open-policy-agent/kube-mgmt/pkg/admission"
	"github.com/open-policy-agent/kube-mgmt/pkg/data"
	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/policies"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"
	versionpkg "github.com/open-policy-agent/kube-mgmt/pkg/version"
)

var (
	kubeconfig                = flag.String("kubeconfig", "", "set path to kubeconfig file manually")
	version                   = flag.Bool("version", false, "print version and exit")
	opaURL                    = flag.String("opa", "http://localhost:8181/v1", "set OPA API URL")
	dataRoot                  = flag.String("data-root", "kubernetes", "set root path for Kubernetes data")
	enableAdmissionControl    = flag.Bool("enable-admission-control", false, "enable admission control support")
	admissionWebhookName      = flag.String("admission-webhook-name", "admission.openpolicyagent.org", "set name of admission control webhook")
	admissionCACertFile       = flag.String("admission-ca-cert-file", "", "set path of admission control CA certificate file")
	admissionServiceName      = flag.String("admission-service-name", "", "set name of admission control service")
	admissionServiceNamespace = flag.String("admission-service-namespace", "", "service namespace of admission control service")
	cluster                   gvkFlag
	namespace                 gvkFlag
)

func init() {
	flag.Var(&cluster, "cluster", "cluster-level resources to replicate (group/version/kind)")
	flag.Var(&namespace, "namespace", "namespace-level resources to replicate (group/version/kind)")
}

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "kube-mgmt manages OPA on top of Kubernetes")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Usage: %v\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if *version {
		fmt.Println("Version:", versionpkg.Version)
		fmt.Println("Git:", versionpkg.Git)
		os.Exit(0)
	}

	logrus.Warn("First line of log stream.")

	kubeconfig, err := loadRESTConfig()
	if err != nil {
		logrus.Fatalf("Failed to load kubeconfig: %v", err)
	}

	sync := policies.New(kubeconfig, opa.New(*opaURL))
	_, err = sync.Run()
	if err != nil {
		logrus.Fatalf("Failed to start policy sync: %v", err)
	}

	for _, gvk := range cluster {
		sync := data.New(
			kubeconfig,
			opa.New(*opaURL).Prefix(*dataRoot),
			types.ResourceType{
				Group:      gvk.Group,
				Version:    gvk.Version,
				Resource:   gvk.Kind,
				Namespaced: false,
			})
		_, err := sync.Run()
		if err != nil {
			logrus.Fatalf("Failed to start data sync for %v: %v", gvk, err)
		}
	}

	for _, gvk := range namespace {
		sync := data.New(
			kubeconfig,
			opa.New(*opaURL).Prefix(*dataRoot),
			types.ResourceType{
				Group:      gvk.Group,
				Version:    gvk.Version,
				Resource:   gvk.Kind,
				Namespaced: true,
			})
		_, err := sync.Run()
		if err != nil {
			logrus.Fatalf("Failed to start data sync for %v: %v", gvk, err)
		}
	}

	if *enableAdmissionControl {
		err := admission.Register(kubeconfig, *admissionWebhookName, *admissionCACertFile, *admissionServiceName, *admissionServiceNamespace, nil)
		if err != nil {
			logrus.Fatalf("Failed to start admission registration: %v", err)
		}
	}

	quit := make(chan struct{})
	<-quit
}

func loadRESTConfig() (*rest.Config, error) {
	if *kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", *kubeconfig)
	}
	return rest.InClusterConfig()
}

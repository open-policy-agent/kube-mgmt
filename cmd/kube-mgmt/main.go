// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/open-policy-agent/kube-mgmt/pkg/admission"
	"github.com/open-policy-agent/kube-mgmt/pkg/data"
	"github.com/open-policy-agent/kube-mgmt/pkg/initialization"
	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/policies"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"
	"github.com/open-policy-agent/kube-mgmt/pkg/version"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type params struct {
	version                             bool
	kubeconfigFile                      string
	opaURL                              string
	replicateCluster                    gvkFlag
	replicateNamespace                  gvkFlag
	replicatePath                       string
	registerAdmissionControllers        bool
	admissionControllerName             string
	admissionControllerCACertFile       string
	admissionControllerServiceName      string
	admissionControllerServiceNamespace string
	initializeCluster                   gvkFlag
	initializeNamespace                 gvkFlag
	initializePath                      string
	initializerSuffix                   string
}

func main() {

	var params params
	commandName := path.Base(os.Args[0])

	rootCmd := &cobra.Command{
		Use:   commandName,
		Short: fmt.Sprintf("%v manages OPA on top of Kubernetes", commandName),
		Run: func(cmd *cobra.Command, args []string) {
			if params.version {
				fmt.Println("Version:", version.Version)
				fmt.Println("Git:", version.Git)
			} else {
				run(&params)
			}
		},
	}

	// Miscellaenous options.
	rootCmd.Flags().BoolVarP(&params.version, "version", "v", false, "print version and exit")
	rootCmd.Flags().StringVarP(&params.kubeconfigFile, "kubeconfig", "", "", "set path to kubeconfig manually")
	rootCmd.Flags().StringVarP(&params.opaURL, "opa-url", "", "http://localhost:8181/v1", "set URL of OPA API endpoint")

	// Replication options.
	rootCmd.Flags().VarP(&params.replicateNamespace, "replicate", "", "replicate namespace-level resources")
	rootCmd.Flags().VarP(&params.replicateCluster, "replicate-cluster", "", "replicate cluster-level resources")
	rootCmd.Flags().StringVarP(&params.replicatePath, "replicate-path", "", "kubernetes", "set path to replicate data into")

	// Admission control options.
	rootCmd.Flags().BoolVarP(&params.registerAdmissionControllers, "register-admission-controller", "", false, "register OPA as an admission controller")
	rootCmd.Flags().StringVarP(&params.admissionControllerName, "admission-controller-name", "", "admission.openpolicyagent.org", "set name of OPA admission controller")
	rootCmd.Flags().StringVarP(&params.admissionControllerCACertFile, "admission-controller-ca-cert-file", "", "", "set path of admission control CA certificate file")
	rootCmd.Flags().StringVarP(&params.admissionControllerServiceName, "admission-controller-service-name", "", "opa", "set name of admission control service")
	rootCmd.Flags().StringVarP(&params.admissionControllerServiceNamespace, "admission-controller-service-namespace", "", "default", "set namespace of admission control service")

	// Initializer options.
	rootCmd.Flags().VarP(&params.initializeNamespace, "initialize", "", "initialize namespace-level resources")
	rootCmd.Flags().VarP(&params.initializeCluster, "initialize-cluster", "", "initialize cluster-level resources")
	rootCmd.Flags().StringVarP(&params.initializePath, "initialize-path", "", "kubernetes/admission/initialize", "set path of initialization policy")
	rootCmd.Flags().StringVarP(&params.initializerSuffix, "initializer-suffix", "", "initializer.openpolicyagent.org", "set suffix of OPA initialization controllers")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(params *params) {

	logrus.Warn("First line of log stream.")

	kubeconfig, err := loadRESTConfig(params.kubeconfigFile)
	if err != nil {
		logrus.Fatalf("Failed to load kubeconfig: %v", err)
	}

	sync := policies.New(kubeconfig, opa.New(params.opaURL))
	_, err = sync.Run()
	if err != nil {
		logrus.Fatalf("Failed to start policy sync: %v", err)
	}

	for _, gvk := range params.replicateCluster {
		sync := data.New(kubeconfig, opa.New(params.opaURL).Prefix(params.replicatePath), getResourceType(gvk, false))
		_, err := sync.Run()
		if err != nil {
			logrus.Fatalf("Failed to start data sync for %v: %v", gvk, err)
		}
	}

	for _, gvk := range params.replicateNamespace {
		sync := data.New(kubeconfig, opa.New(params.opaURL).Prefix(params.replicatePath), getResourceType(gvk, true))
		_, err := sync.Run()
		if err != nil {
			logrus.Fatalf("Failed to start data sync for %v: %v", gvk, err)
		}
	}

	for _, gvk := range params.initializeCluster {
		name := getInitializerName(gvk, params.initializerSuffix)
		init := initialization.New(kubeconfig, opa.New(params.opaURL).Prefix(params.initializePath), getResourceType(gvk, false), name)
		_, err := init.Run()
		if err != nil {
			logrus.Fatalf("Failed to start initializer for %v: %v", gvk, err)
		}
	}

	for _, gvk := range params.initializeNamespace {
		name := getInitializerName(gvk, params.initializerSuffix)
		init := initialization.New(kubeconfig, opa.New(params.opaURL).Prefix(params.initializePath), getResourceType(gvk, true), name)
		_, err := init.Run()
		if err != nil {
			logrus.Fatalf("Failed to start initializer for %v: %v", gvk, err)
		}
	}

	if params.registerAdmissionControllers {
		if err := admission.InstallDefaultAdmissionPolicy("default-system-main", opa.New(params.opaURL)); err != nil {
			logrus.Fatalf("Failed to install default policy: %v", err)
		}
		err := admission.Register(kubeconfig, params.admissionControllerName, params.admissionControllerCACertFile, params.admissionControllerServiceName, params.admissionControllerServiceNamespace, nil)
		if err != nil {
			logrus.Fatalf("Failed to start admission registration: %v", err)
		}
	}

	quit := make(chan struct{})
	<-quit
}

func loadRESTConfig(path string) (*rest.Config, error) {
	if path != "" {
		return clientcmd.BuildConfigFromFlags("", path)
	}
	return rest.InClusterConfig()
}

func getInitializerName(gvk groupVersionKind, suffix string) string {
	return strings.Replace(gvk.String(), "/", ".", -1) + "." + suffix
}

func getResourceType(gvk groupVersionKind, namespaced bool) types.ResourceType {
	return types.ResourceType{
		Namespaced: namespaced,
		Group:      gvk.Group,
		Version:    gvk.Version,
		Resource:   gvk.Kind,
	}
}

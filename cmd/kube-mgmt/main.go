// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/open-policy-agent/kube-mgmt/pkg/configmap"
	"github.com/open-policy-agent/kube-mgmt/pkg/data"
	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"
	"github.com/open-policy-agent/kube-mgmt/pkg/version"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type params struct {
	version            bool
	kubeconfigFile     string
	opaURL             string
	opaAuth            string
	opaAuthFile        string
	podName            string
	podNamespace       string
	enablePolicies     bool
	enableData         bool
	policies           []string
	requirePolicyLabel bool
	replicateCluster   gvkFlag
	replicateNamespace gvkFlag
	replicatePath      string
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
	rootCmd.Flags().StringVarP(&params.opaAuth, "opa-auth-token", "", "", "set authentication token for OPA API endpoint")
	rootCmd.Flags().StringVarP(&params.opaAuthFile, "opa-auth-token-file", "", "", "set file containing authentication token for OPA API endpoint")
	rootCmd.Flags().StringVarP(&params.podName, "pod-name", "", "", "set pod name (required for admission registration ownership)")
	rootCmd.Flags().StringVarP(&params.podNamespace, "pod-namespace", "", "", "set pod namespace (required for admission registration ownership)")

	// Replication options.
	rootCmd.Flags().BoolVarP(&params.enablePolicies, "enable-policies", "", true, "whether to automatically discover policies from ConfigMaps")
	rootCmd.Flags().BoolVarP(&params.enableData, "enable-data", "", false, "whether to automatically discover data from correctly labelled ConfigMaps")
	rootCmd.Flags().StringSliceVarP(&params.policies, "policies", "", []string{"opa", "kube-federation-scheduling-policy"}, "automatically load policies from these namespaces")
	rootCmd.Flags().BoolVarP(&params.requirePolicyLabel, "require-policy-label", "", false, "only load policies out of labelled configmaps")
	rootCmd.Flags().VarP(&params.replicateNamespace, "replicate", "", "replicate namespace-level resources")
	rootCmd.Flags().VarP(&params.replicateCluster, "replicate-cluster", "", "replicate cluster-level resources")
	rootCmd.Flags().StringVarP(&params.replicatePath, "replicate-path", "", "kubernetes", "set path to replicate data into")
	var replicateResync time.Duration
	rootCmd.Flags().DurationVar(&replicateResync, "replicate-resync", 60*time.Second, "resend all PUT messages at this interval")
	rootCmd.Flags().MarkDeprecated("replicate-resync", "not applicable")

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

	if params.opaAuthFile != "" && params.opaAuth != "" {
		logrus.Fatalf("You can not use both --opa-auth-token and --opa-auth-token-file")
	}

	if params.opaAuthFile != "" {
		file, err := ioutil.ReadFile(params.opaAuthFile)
		if err != nil {
			logrus.Fatalf("Failed to read opa auth token file %s", params.opaAuthFile)
		}
		params.opaAuth = strings.Split(string(file), "\n")[0]
	}

	if params.enablePolicies || params.enableData {
		sync := configmap.New(
			kubeconfig,
			opa.New(params.opaURL, params.opaAuth),
			configmap.DefaultConfigMapMatcher(
				params.policies,
				params.requirePolicyLabel,
				params.enablePolicies,
				params.enableData,
			),
		)
		_, err = sync.Run(params.policies)
		if err != nil {
			logrus.Fatalf("Failed to start configmap sync: %v", err)
		}
	}

	for _, gvk := range params.replicateCluster {
		sync := data.New(kubeconfig, opa.New(params.opaURL, params.opaAuth).Prefix(params.replicatePath), getResourceType(gvk, false))
		_, err := sync.Run()
		if err != nil {
			logrus.Fatalf("Failed to start data sync for %v: %v", gvk, err)
		}
	}

	for _, gvk := range params.replicateNamespace {
		sync := data.New(kubeconfig, opa.New(params.opaURL, params.opaAuth).Prefix(params.replicatePath), getResourceType(gvk, true))
		_, err := sync.Run()
		if err != nil {
			logrus.Fatalf("Failed to start data sync for %v: %v", gvk, err)
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

func getResourceType(gvk groupVersionKind, namespaced bool) types.ResourceType {
	return types.ResourceType{
		Namespaced: namespaced,
		Group:      gvk.Group,
		Version:    gvk.Version,
		Resource:   gvk.Kind,
	}
}

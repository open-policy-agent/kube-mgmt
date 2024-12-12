// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/open-policy-agent/kube-mgmt/pkg/configmap"
	"github.com/open-policy-agent/kube-mgmt/pkg/data"
	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"
	"github.com/open-policy-agent/kube-mgmt/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type params struct {
	version            bool
	kubeconfigFile     string
	opaURL             string
	opaAuth            string
	opaAuthFile        string
	opaCAFile          string
	opaAllowInsecure   bool
	policyLabel        string
	policyValue        string
	dataLabel          string
	dataValue          string
	enablePolicies     bool
	enableData         bool
	namespaces         []string
	replicateCluster   gvkFlag
	replicateNamespace gvkFlag
	replicatePath      string
	logLevel           string
	replicateIgnoreNs  []string
}

func main() {

	var params params
	commandName := path.Base(os.Args[0])

	rootCmd := &cobra.Command{
		Use:   commandName,
		Short: fmt.Sprintf("%v manages OPA on top of Kubernetes", commandName),
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
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
	rootCmd.Flags().StringVarP(&params.opaCAFile, "opa-ca-file", "", "", "set file containing certificate authority for OPA certificate")
	rootCmd.Flags().BoolVarP(&params.opaAllowInsecure, "opa-allow-insecure", "", false, "allow insecure https connections to OPA")
	rootCmd.Flags().StringVar(&params.logLevel, "log-level", "info", "set log level {debug, info, warn}")

	// policy / data
	rootCmd.Flags().BoolVarP(&params.enablePolicies, "enable-policies", "", true, "whether to automatically discover policies from labelled ConfigMaps")
	rootCmd.Flags().StringVar(&params.policyLabel, "policy-label", "openpolicyagent.org/policy", "label name for filtering ConfigMaps with policies")
	rootCmd.Flags().StringVar(&params.policyValue, "policy-value", "rego", "label value for filtering ConfigMaps with policies")
	rootCmd.Flags().BoolVarP(&params.enableData, "enable-data", "", true, "whether to automatically discover data from labelled ConfigMaps")
	rootCmd.Flags().StringVar(&params.dataLabel, "data-label", "openpolicyagent.org/data", "label name for filtering ConfigMaps with data")
	rootCmd.Flags().StringVar(&params.dataValue, "data-value", "opa", "label value for filtering ConfigMaps with data")
	rootCmd.Flags().StringSliceVarP(&params.namespaces, "namespaces", "", []string{""}, "namespaces to load policies and data from")

	// replication
	rootCmd.Flags().VarP(&params.replicateNamespace, "replicate", "", "replicate namespace-level resources")
	rootCmd.Flags().VarP(&params.replicateCluster, "replicate-cluster", "", "replicate cluster-level resources")
	rootCmd.Flags().StringVarP(&params.replicatePath, "replicate-path", "", "kubernetes", "set path to replicate data into")
	rootCmd.Flags().StringSliceVarP(&params.replicateIgnoreNs, "replicate-ignore-namespaces", "", []string{""}, "namespaces that are ignored by replication")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if rootCmd.Flag("policy-label").Value.String() != "" || rootCmd.Flag("policy-value").Value.String() != "" {
			err := configmap.CustomLabel(params.policyLabel, params.policyValue)
			if err != nil {
				logrus.Fatalf("Invalid --policy-label:%v || --policy-value:%v, %v", params.policyLabel, params.policyValue, err)
			}
		}
		if rootCmd.Flag("data-label").Value.String() != "" || rootCmd.Flag("data-value").Value.String() != "" {
			err := configmap.CustomLabel(params.dataLabel, params.dataValue)
			if err != nil {
				logrus.Fatalf("Invalid --data-label:%v || --data-value:%v, %v", params.dataLabel, params.dataValue, err)
			}
		}
		return nil
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}

func run(params *params) {

	switch params.logLevel {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	default:
		logrus.Fatalf("Invalid log level %v", params.logLevel)
	}

	kubeconfig, err := loadRESTConfig(params.kubeconfigFile)
	if err != nil {
		logrus.Fatalf("Failed to load kubeconfig: %v", err)
	}

	if params.opaAuthFile != "" && params.opaAuth != "" {
		logrus.Fatalf("You can not use both --opa-auth-token and --opa-auth-token-file")
	}

	if params.opaAuthFile != "" {
		file, err := os.ReadFile(params.opaAuthFile)
		if err != nil {
			logrus.Fatalf("Failed to read opa auth token file %s", params.opaAuthFile)
		}
		params.opaAuth = strings.Split(string(file), "\n")[0]
	}

	if params.opaAllowInsecure && params.opaCAFile != "" {
		logrus.Fatalf("You can not use both --opa-allow-insecure and --opa-ca-file")
	}

	if params.opaAllowInsecure {
		config := &tls.Config{InsecureSkipVerify: params.opaAllowInsecure}
		http.DefaultTransport.(*http.Transport).TLSClientConfig = config
	}

	if params.opaCAFile != "" {
		rootCAs, _ := x509.SystemCertPool()
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		certs, err := os.ReadFile(params.opaCAFile)
		if err != nil {
			logrus.Fatalf("Failed to read opa certificate authority file %s", params.opaCAFile)
		}
		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			logrus.Println("No certs appended, using system certs only")
		}
		config := &tls.Config{RootCAs: rootCAs}
		http.DefaultTransport.(*http.Transport).TLSClientConfig = config
	}

	if params.enablePolicies || params.enableData {
		sync := configmap.New(
			kubeconfig,
			opa.New(params.opaURL, params.opaAuth),
			configmap.DefaultConfigMapMatcher(
				params.namespaces,
				params.enablePolicies,
				params.enableData,
				params.policyLabel,
				params.policyValue,
				params.dataLabel,
				params.dataValue,
			),
		)
		_, err = sync.Run(params.namespaces)
		if err != nil {
			logrus.Fatalf("Failed to start configmap sync: %v", err)
		}
	}

	var client dynamic.Interface
	if len(params.replicateCluster)+len(params.replicateNamespace) > 0 {
		client, err = dynamic.NewForConfig(kubeconfig)
		if err != nil {
			logrus.Fatalf("Failed to get dynamic client: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		opts := data.WithIgnoreNamespaces(params.replicateIgnoreNs)

		for _, gvk := range params.replicateCluster {
			sync := data.NewFromInterface(client, opa.New(params.opaURL, params.opaAuth).Prefix(params.replicatePath), getResourceType(gvk, false), opts)
			go sync.RunContext(ctx)
		}

		for _, gvk := range params.replicateNamespace {
			sync := data.NewFromInterface(client, opa.New(params.opaURL, params.opaAuth).Prefix(params.replicatePath), getResourceType(gvk, true), opts)
			go sync.RunContext(ctx)
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

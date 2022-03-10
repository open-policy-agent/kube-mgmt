// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

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
	rootCmd.Flags().StringVarP(&params.podName, "pod-name", "", "", "set pod name (required for admission registration ownership)")
	rootCmd.Flags().StringVarP(&params.podNamespace, "pod-namespace", "", "", "set pod namespace (required for admission registration ownership)")
	rootCmd.Flags().StringVarP(&params.policyLabel, "policy-label", "", "openpolicyagent.org/policy", "replace label openpolicyagent.org/policy")
	rootCmd.Flags().StringVarP(&params.policyValue, "policy-value", "", "rego", "replace value rego")

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

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if rootCmd.Flag("policy-label").Value.String() != "" || rootCmd.Flag("policy-value").Value.String() != "" {
			_, err := configmap.CustomPolicyLabel(params.policyLabel, params.policyValue)
			if err != nil {
				logrus.Fatalf("Invalid --policy-label:%v || --policy-value:%v, %v", params.policyLabel, params.policyValue, err)
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
		certs, err := ioutil.ReadFile(params.opaCAFile)
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
				params.policies,
				params.requirePolicyLabel,
				params.enablePolicies,
				params.enableData,
				params.policyLabel,
				params.policyValue,
			),
		)
		_, err = sync.Run(params.policies)
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
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, gvk := range params.replicateCluster {
		sync := data.NewFromInterface(client, opa.New(params.opaURL, params.opaAuth).Prefix(params.replicatePath), getResourceType(gvk, false))
		go sync.RunContext(ctx)
	}

	for _, gvk := range params.replicateNamespace {
		sync := data.NewFromInterface(client, opa.New(params.opaURL, params.opaAuth).Prefix(params.replicatePath), getResourceType(gvk, true))
		go sync.RunContext(ctx)
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

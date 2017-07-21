// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package admission

import (
	"time"

	"github.com/Sirupsen/logrus"

	"io/ioutil"

	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/apis/admissionregistration/v1alpha1"
	"k8s.io/client-go/rest"
)

var (
	defaultAdmissionPolicy = []byte(`
		package system

		default main = {
			"apiVersion": "admission.k8s.io/v1alpha1",
			"kind": "AdmissionReview",
			"status": {
				"allowed": true,
			},
		}
	`)
)

// InstallDefaultAdmissionPolicy will update OPA with a default policy under
// system.main to allow all resources. This function will block until the
// policy has been installed.
func InstallDefaultAdmissionPolicy(id string, opa opa_client.Policies) error {
	for {
		time.Sleep(time.Second * 1)
		if err := opa.InsertPolicy(id, defaultAdmissionPolicy); err != nil {
			logrus.Errorf("Failed to install default policy: %v", err)
		} else {
			return nil
		}
	}
}

// Register attempts to register an admission control webhook with the given CA
// certificate, service name/namespace, etc.
func Register(kubeconfig *rest.Config, webhookName, caCertFile, serviceName, serviceNamespace string, done <-chan struct{}) error {

	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}

	caCert, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return err
	}

	client := clientset.AdmissionregistrationV1alpha1().ExternalAdmissionHookConfigurations()
	_, err = client.Get(webhookName, metav1.GetOptions{})
	if err == nil {
		if err := client.Delete(webhookName, nil); err != nil {
			return err
		}
	}

	go func() {
		ticker := time.NewTicker(time.Second * 1)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				webhookConfig := &v1alpha1.ExternalAdmissionHookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: webhookName,
					},
					ExternalAdmissionHooks: []v1alpha1.ExternalAdmissionHook{
						{
							Name: webhookName,
							Rules: []v1alpha1.RuleWithOperations{{
								Operations: []v1alpha1.OperationType{v1alpha1.OperationAll},
								Rule: v1alpha1.Rule{
									APIGroups:   []string{"*"},
									APIVersions: []string{"*"},
									Resources:   []string{"*/*"},
								},
							}},
							ClientConfig: v1alpha1.AdmissionHookClientConfig{
								Service: v1alpha1.ServiceReference{
									Namespace: serviceNamespace,
									Name:      serviceName,
								},
								CABundle: caCert,
							},
						},
					},
				}
				if _, err := client.Create(webhookConfig); err != nil {
					logrus.Errorf("Failed to declare webhook configuration: %v", err)
				} else {
					logrus.Infof("Registered webhook configuration: %v", webhookName)
					return
				}
			}
		}
	}()

	return nil
}

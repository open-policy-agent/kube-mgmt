// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package policies

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	policyLabelKey       = "openpolicyagent.org/policy"
	errorLabelKey        = "openpolicyagent.org/policy-error"
	policyLabelValueRego = "rego"

	// Special namespace in Kubernetes federation that holds scheduling policies.
	kubeFederationSchedulingPolicy = "kube-federation-scheduling-policy"
)

// ConfigMapSync replicates policies stored in the API server as ConfigMaps into OPA.
type ConfigMapSync struct {
	kubeconfig *rest.Config
	opa        opa.Policies
	clientset  *kubernetes.Clientset
}

// New returns a new ConfigMapSync that can be started.
func New(kubeconfig *rest.Config, opa opa.Policies) *ConfigMapSync {
	cpy := *kubeconfig
	cpy.GroupVersion = &schema.GroupVersion{
		Version: "v1",
	}
	cpy.APIPath = "/api"
	cpy.ContentType = runtime.ContentTypeJSON
	cpy.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}
	builder := runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(
			*cpy.GroupVersion,
			&api.ListOptions{},
			&v1.ConfigMapList{},
			&v1.ConfigMap{})
		return nil
	})
	builder.AddToScheme(api.Scheme)
	return &ConfigMapSync{
		kubeconfig: &cpy,
		opa:        opa,
	}
}

// Run starts the synchronizer. To stop the synchronizer send a message to the
// channel.
func (s *ConfigMapSync) Run() (chan struct{}, error) {
	client, err := rest.RESTClientFor(s.kubeconfig)
	if err != nil {
		return nil, err
	}
	s.clientset, err = kubernetes.NewForConfig(s.kubeconfig)
	if err != nil {
		return nil, err
	}
	quit := make(chan struct{})
	source := cache.NewListWatchFromClient(
		client,
		"configmaps",
		v1.NamespaceAll,
		fields.Everything())
	store, controller := cache.NewInformer(
		source,
		&v1.ConfigMap{},
		time.Second*60,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.add,
			UpdateFunc: s.update,
			DeleteFunc: s.delete,
		})
	for _, obj := range store.List() {
		cm := obj.(*v1.ConfigMap)
		if s.match(cm) {
			s.syncAdd(cm)
		}
	}
	go controller.Run(quit)
	return quit, nil
}

func (s *ConfigMapSync) add(obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if s.match(cm) {
		s.syncAdd(cm)
	}
}

func (s *ConfigMapSync) update(_, obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if s.match(cm) {
		s.syncAdd(cm)
	}
}

func (s *ConfigMapSync) delete(obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if s.match(cm) {
		s.syncRemove(cm)
	}
}

func (s *ConfigMapSync) match(cm *v1.ConfigMap) bool {
	return s.matchLabel(cm) || s.matchNamespace(cm)
}

func (s *ConfigMapSync) matchLabel(cm *v1.ConfigMap) bool {
	return cm.Labels[policyLabelKey] == policyLabelValueRego
}

func (s *ConfigMapSync) matchNamespace(cm *v1.ConfigMap) bool {
	return cm.Namespace == kubeFederationSchedulingPolicy
}

func (s *ConfigMapSync) syncAdd(cm *v1.ConfigMap) {
	path := fmt.Sprintf("%v/%v", cm.Namespace, cm.Name)
	for key, value := range cm.Data {
		id := fmt.Sprintf("%v/%v", path, key)
		if err := s.opa.InsertPolicy(id, []byte(value)); err != nil {
			s.setErrorAnnotation(cm, err)
			continue
		}
	}
}

func (s *ConfigMapSync) syncRemove(cm *v1.ConfigMap) {
	path := fmt.Sprintf("%v/%v", cm.Namespace, cm.Name)
	for key := range cm.Data {
		id := fmt.Sprintf("%v/%v", path, key)
		if err := s.opa.DeletePolicy(id); err != nil {
			logrus.Errorf("Failed to delete policy %v: %v", id, err)
			continue
		}
	}
}

func (s *ConfigMapSync) setErrorAnnotation(cm *v1.ConfigMap, err error) {
	errBytes, err2 := json.Marshal(err)
	if err != nil {
		logrus.Errorf("Failed to serialize OPA error for %v/%v: %v", cm.Namespace, cm.Name, err2)
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				errorLabelKey: string(errBytes),
			},
		},
	}
	bs, err2 := json.Marshal(patch)
	if err2 != nil {
		logrus.Errorf("Failed to serialize error patch for %v/%v: %v", cm.Namespace, cm.Name, err2)
	}
	fmt.Println(string(bs))
	_, err2 = s.clientset.ConfigMaps(cm.Namespace).Patch(cm.Name, types.StrategicMergePatchType, bs)
	if err2 != nil {
		logrus.Errorf("Failed to update error for %v/%v: %v (err: %v)", cm.Namespace, cm.Name, err2, err)
	}
}

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
	statusAnnotationKey  = "openpolicyagent.org/policy-status"
	policyLabelValueRego = "rego"

	// Special namespace in Kubernetes federation that holds scheduling policies.
	kubeFederationSchedulingPolicy = "kube-federation-scheduling-policy"
)

// DefaultConfigMapMatcher returns a function that will match configmaps in
// specified namespaces and/or with the openpolicyagent.org/policy label.
func DefaultConfigMapMatcher(namespaces []string, requirePolicyLabel bool) func(*v1.ConfigMap) bool {
	return func(cm *v1.ConfigMap) bool {
		if requirePolicyLabel {
			return matchesNamespace(cm, namespaces) && matchesLabel(cm)
		}
		return matchesNamespace(cm, namespaces) || matchesLabel(cm)
	}
}

func matchesLabel(cm *v1.ConfigMap) bool {
	return cm.Labels[policyLabelKey] == policyLabelValueRego
}

func matchesNamespace(cm *v1.ConfigMap, policies []string) bool {
	for _, ns := range policies {
		if ns == cm.Namespace {
			return true
		}
	}
	return false
}

// ConfigMapSync replicates policies stored in the API server as ConfigMaps into OPA.
type ConfigMapSync struct {
	kubeconfig *rest.Config
	opa        opa.Policies
	clientset  *kubernetes.Clientset
	matcher    func(*v1.ConfigMap) bool
}

// New returns a new ConfigMapSync that can be started.
func New(kubeconfig *rest.Config, opa opa.Policies, matcher func(*v1.ConfigMap) bool) *ConfigMapSync {
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
		matcher:    matcher,
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
		if s.matcher(cm) {
			s.syncAdd(cm)
		}
	}
	go controller.Run(quit)
	return quit, nil
}

func (s *ConfigMapSync) add(obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if s.matcher(cm) {
		s.syncAdd(cm)
	}
}

func (s *ConfigMapSync) update(_, obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if s.matcher(cm) {
		s.syncAdd(cm)
	}
}

func (s *ConfigMapSync) delete(obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if s.matcher(cm) {
		s.syncRemove(cm)
	}
}

func (s *ConfigMapSync) syncAdd(cm *v1.ConfigMap) {
	path := fmt.Sprintf("%v/%v", cm.Namespace, cm.Name)
	for key, value := range cm.Data {
		id := fmt.Sprintf("%v/%v", path, key)
		if err := s.opa.InsertPolicy(id, []byte(value)); err != nil {
			s.setStatusAnnotation(cm, status{
				Status: "error",
				Error:  err,
			})
		} else {
			s.setStatusAnnotation(cm, status{
				Status: "ok",
			})
		}
	}
}

func (s *ConfigMapSync) syncRemove(cm *v1.ConfigMap) {
	path := fmt.Sprintf("%v/%v", cm.Namespace, cm.Name)
	for key := range cm.Data {
		id := fmt.Sprintf("%v/%v", path, key)
		if err := s.opa.DeletePolicy(id); err != nil {
			logrus.Errorf("Failed to delete policy %v: %v", id, err)
		}
	}
}

func (s *ConfigMapSync) setStatusAnnotation(cm *v1.ConfigMap, st status) {
	bs, err := json.Marshal(st)
	if err != nil {
		logrus.Errorf("Failed to serialize %v for %v/%v: %v", statusAnnotationKey, cm.Namespace, cm.Name, err)
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				statusAnnotationKey: string(bs),
			},
		},
	}
	bs, err = json.Marshal(patch)
	if err != nil {
		logrus.Errorf("Failed to serialize patch for %v/%v: %v", cm.Namespace, cm.Name, err)
	}
	_, err = s.clientset.ConfigMaps(cm.Namespace).Patch(cm.Name, types.StrategicMergePatchType, bs)
	if err != nil {
		logrus.Errorf("Failed to %v for %v/%v: %v", statusAnnotationKey, cm.Namespace, cm.Name, err)
	}
}

type status struct {
	Status string `json:"status"`
	Error  error  `json:"error,omitempty"`
}

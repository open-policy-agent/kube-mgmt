// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package configmap

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
	v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	policyLabelKey            = "openpolicyagent.org/policy"
	policyLabelValue          = "rego"
	policyStatusAnnotationKey = "openpolicyagent.org/policy-status"

	dataLabelKey            = "openpolicyagent.org/data"
	dataLabelValue          = "opa"
	dataStatusAnnotationKey = "openpolicyagent.org/data-status"

	// Special namespace in Kubernetes federation that holds scheduling policies.
	kubeFederationSchedulingPolicy = "kube-federation-scheduling-policy"

	resyncPeriod        = time.Second * 60
	syncResetBackoffMin = time.Second
	syncResetBackoffMax = time.Second * 30
)

// DefaultConfigMapMatcher returns a function that will match configmaps in
// specified namespaces and/or with a policy or data label. The first bool return
// value specifies a policy/data match and the second bool indicates if the configmap
// contains a policy.
func DefaultConfigMapMatcher(namespaces []string, requirePolicyLabel bool) func(*v1.ConfigMap) (bool, bool) {
	return func(cm *v1.ConfigMap) (bool, bool) {
		if requirePolicyLabel {
			return matchesNamespace(cm, namespaces) && matchesLabel(cm, policyLabelKey, policyLabelValue), true
		}

		// Check for data label. This label needs to be set
		// on any configmap that contains JSON data to be loaded into OPA.
		if matchesNamespace(cm, namespaces) && matchesLabel(cm, dataLabelKey, dataLabelValue) {
			return true, false
		}

		// No data type label, so treat all other configmaps as potential policy type
		return matchesNamespace(cm, namespaces) || matchesLabel(cm, policyLabelKey, policyLabelValue), true
	}
}

func matchesLabel(cm *v1.ConfigMap, labelKey, labelValue string) bool {
	return cm.Labels[labelKey] == labelValue
}

func matchesNamespace(cm *v1.ConfigMap, namespaces []string) bool {
	for _, ns := range namespaces {
		if ns == cm.Namespace || ns == "*" {
			return true
		}
	}
	return false
}

// Sync replicates policies or data stored in the API server as ConfigMaps into OPA.
type Sync struct {
	kubeconfig *rest.Config
	opa        opa.Client
	clientset  *kubernetes.Clientset
	matcher    func(*v1.ConfigMap) (bool, bool)
}

// New returns a new Sync that can be started.
func New(kubeconfig *rest.Config, opa opa.Client, matcher func(*v1.ConfigMap) (bool, bool)) *Sync {
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
	return &Sync{
		kubeconfig: &cpy,
		opa:        opa,
		matcher:    matcher,
	}
}

// Run starts the synchronizer. To stop the synchronizer send a message to the
// channel.
func (s *Sync) Run() (chan struct{}, error) {
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
		if match, isPolicy := s.matcher(cm); match {
			s.syncAdd(cm, isPolicy)
		}
	}
	go controller.Run(quit)
	return quit, nil
}

func (s *Sync) add(obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if match, isPolicy := s.matcher(cm); match {
		s.syncAdd(cm, isPolicy)
	}
}

func (s *Sync) update(_, obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if match, isPolicy := s.matcher(cm); match {
		s.syncAdd(cm, isPolicy)
	}
}

func (s *Sync) delete(obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if match, isPolicy := s.matcher(cm); match {
		s.syncRemove(cm, isPolicy)
	}
}

func (s *Sync) syncAdd(cm *v1.ConfigMap, isPolicy bool) {
	path := fmt.Sprintf("%v/%v", cm.Namespace, cm.Name)
	for key, value := range cm.Data {
		id := fmt.Sprintf("%v/%v", path, key)

		var err error
		if isPolicy {
			err = s.opa.InsertPolicy(id, []byte(value))
		} else {
			// We don't need to know the JSON structure, just pass it
			// directly to the OPA data store.
			var data map[string]interface{}
			if err = json.Unmarshal([]byte(value), &data); err != nil {
				logrus.Errorf("Failed to parse JSON data in configmap with id: %s", id)
			} else {
				err = s.opa.PutData(id, data)
			}
		}

		if err != nil {
			s.setStatusAnnotation(cm, status{
				Status: "error",
				Error:  err,
			}, isPolicy)
		} else {
			s.setStatusAnnotation(cm, status{
				Status: "ok",
			}, isPolicy)
		}
	}
}

func (s *Sync) syncRemove(cm *v1.ConfigMap, isPolicy bool) {
	path := fmt.Sprintf("%v/%v", cm.Namespace, cm.Name)
	for key := range cm.Data {
		id := fmt.Sprintf("%v/%v", path, key)

		if isPolicy {
			if err := s.opa.DeletePolicy(id); err != nil {
				logrus.Errorf("Failed to delete policy %v: %v", id, err)
			}
		} else {
			if err := s.opa.PatchData(path, "remove", nil); err != nil {
				logrus.Errorf("Failed to remove %v (will reset OPA data and resync in %v): %v", id, resyncPeriod, err)
				s.syncReset(id)
			}
		}
	}
}

func (s *Sync) setStatusAnnotation(cm *v1.ConfigMap, st status, isPolicy bool) {
	bs, err := json.Marshal(st)

	statusAnnotationKey := policyStatusAnnotationKey
	if !isPolicy {
		statusAnnotationKey = dataStatusAnnotationKey
	}
	if err != nil {
		logrus.Errorf("Failed to serialize %v for %v/%v: %v", statusAnnotationKey, cm.Namespace, cm.Name, err)
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				policyStatusAnnotationKey: string(bs),
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

func (s *Sync) syncReset(id string) {
	d := syncResetBackoffMin
	for {
		if err := s.opa.PutData("/", map[string]interface{}{}); err != nil {
			logrus.Errorf("Failed to reset OPA data for %v (will retry after %v): %v", id, d, err)
		} else {
			return
		}
		time.Sleep(d)
		d = d * 2
		if d > syncResetBackoffMax {
			d = syncResetBackoffMax
		}
	}
}

type status struct {
	Status string `json:"status"`
	Error  error  `json:"error,omitempty"`
}

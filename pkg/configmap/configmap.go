// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package configmap

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	policyStatusAnnotationKey = "openpolicyagent.org/policy-status"

	dataLabelKey            = "openpolicyagent.org/data"
	dataLabelValue          = "opa"
	dataStatusAnnotationKey = "openpolicyagent.org/data-status"

	// Special namespace in Kubernetes federation that holds scheduling policies.
	// commented because staticcheck: 'const kubeFederationSchedulingPolicy is unused (U1000)'
	// kubeFederationSchedulingPolicy = "kube-federation-scheduling-policy"

	resyncPeriod        = time.Second * 60
	syncResetBackoffMin = time.Second
	syncResetBackoffMax = time.Second * 30
)

// CustomPolicyLabel allows the default key "openpolicyagent.org/policy"
// to be replaced by another value. This would allow two instances of kube-mgmt
// to share a single namepace with config maps for different servers. (ie. validating & mutating)
func CustomPolicyLabel(key, value string) (string, error) {
	_, err := labels.NewRequirement(key, selection.Equals, []string{value})
	if err != nil {
		return "", err
	}

	policyLabelKey := key
	policyLabelValue := value
	fullLabel := strings.Join([]string{policyLabelKey, policyLabelValue}, "=")
	return fullLabel, nil
}

// DefaultConfigMapMatcher returns a function that will match configmaps in
// specified namespaces and/or with a policy or data label. The first bool return
// value specifies a policy/data match and the second bool indicates if the configmap
// contains a policy.
func DefaultConfigMapMatcher(namespaces []string, requirePolicyLabel, enablePolicies, enableData bool, policyLabelKey, policyLabelValue string) func(*v1.ConfigMap) (bool, bool) {
	return func(cm *v1.ConfigMap) (bool, bool) {
		var match, isPolicy bool

		// Check for data label. This label needs to be set on any
		// configmap that contains JSON data to be loaded into OPA.
		if enableData {
			match = matchesNamespace(cm, namespaces) && matchesLabel(cm, dataLabelKey, dataLabelValue)
		}

		// Check for explicit policy label or match on any policy namespace.
		if !match && enablePolicies {
			if requirePolicyLabel {
				match = matchesNamespace(cm, namespaces) && matchesLabel(cm, policyLabelKey, policyLabelValue)
			} else {
				match = matchesNamespace(cm, namespaces) || matchesLabel(cm, policyLabelKey, policyLabelValue)
			}

			if match {
				isPolicy = true
			}
		}
		return match, isPolicy
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
	scheme := runtime.NewScheme()
	cpy.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(scheme)}
	builder := runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		scheme.AddKnownTypes(
			*cpy.GroupVersion,
			&metav1.ListOptions{},
			&metav1.Status{},
			&v1.ConfigMapList{},
			&v1.ConfigMap{})
		return nil
	})
	builder.AddToScheme(scheme)
	return &Sync{
		kubeconfig: &cpy,
		opa:        opa,
		matcher:    matcher,
	}
}

// Run starts the synchronizer. To stop the synchronizer send a message to the
// channel.
func (s *Sync) Run(namespaces []string) (chan struct{}, error) {
	client, err := rest.RESTClientFor(s.kubeconfig)
	if err != nil {
		return nil, err
	}
	s.clientset, err = kubernetes.NewForConfig(s.kubeconfig)
	if err != nil {
		return nil, err
	}
	quit := make(chan struct{})

	for _, namespace := range namespaces {
		if namespace == "*" {
			namespace = v1.NamespaceAll
		}
		source := cache.NewListWatchFromClient(
			client,
			"configmaps",
			namespace,
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
	}
	return quit, nil
}

func (s *Sync) add(obj interface{}) {
	cm := obj.(*v1.ConfigMap)
	if match, isPolicy := s.matcher(cm); match {
		s.syncAdd(cm, isPolicy)
	}
}

func (s *Sync) update(oldObj, obj interface{}) {
	oldCm, cm := oldObj.(*v1.ConfigMap), obj.(*v1.ConfigMap)
	if match, isPolicy := s.matcher(cm); match {
		// avoid processing new versions of the ConfigMap that don't actually
		// change policy, data or labels
		// (issue https://github.com/open-policy-agent/kube-mgmt/issues/131)
		if cm.GetResourceVersion() != oldCm.GetResourceVersion() {
			fp, oldFp := fingerprint(cm), fingerprint(oldCm)
			if fp == oldFp {
				return
			}
		}
		s.syncAdd(cm, isPolicy)
	} else {
		// check if the label was removed
		if match, isPolicy := s.matcher(oldCm); match {
			s.syncRemove(oldCm, isPolicy)
		}
	}
}

func (s *Sync) delete(obj interface{}) {
	if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = d.Obj
	}
	cm := obj.(*v1.ConfigMap)
	if match, isPolicy := s.matcher(cm); match {
		s.syncRemove(cm, isPolicy)
	}
}

func (s *Sync) syncAdd(cm *v1.ConfigMap, isPolicy bool) {
	path := fmt.Sprintf("%v/%v", cm.Namespace, cm.Name)
	// sort keys so that errors, if any, are always in the same order
	sortedKeys := make([]string, 0, len(cm.Data))
	for key := range cm.Data {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	var syncErr errList
	for _, key := range sortedKeys {
		value := cm.Data[key]
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
			syncErr = append(syncErr, err)
		}
	}
	if syncErr != nil {
		s.setStatusAnnotation(cm, status{
			Status: "error",
			Error:  syncErr,
		}, isPolicy)
	} else {
		s.setStatusAnnotation(cm, status{
			Status: "ok",
		}, isPolicy)
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
	annotation := string(bs)
	if cm.Annotations != nil {
		if existing, ok := cm.Annotations[policyStatusAnnotationKey]; ok {
			if existing == annotation {
				// If the annotation did not change, do not write it.
				// (issue https://github.com/open-policy-agent/kube-mgmt/issues/90)
				return
			}
		}
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				policyStatusAnnotationKey: annotation,
			},
		},
	}
	bs, err = json.Marshal(patch)
	if err != nil {
		logrus.Errorf("Failed to serialize patch for %v/%v: %v", cm.Namespace, cm.Name, err)
	}
	_, err = s.clientset.CoreV1().ConfigMaps(cm.Namespace).Patch(context.TODO(), cm.Name, types.StrategicMergePatchType, bs, metav1.PatchOptions{})
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

// fingerprint for the labels and data of a configmap.
func fingerprint(cm *v1.ConfigMap) uint64 {
	hash := fnv.New64a()
	data := json.NewEncoder(hash)
	data.Encode(cm.Labels)
	data.Encode(cm.Data)
	return hash.Sum64()
}

// errList is an error type that can marshal a list of errors to json
type errList []error

var (
	// Make sure we implement the proper interfaces
	_ error          = errList{}
	_ json.Marshaler = errList{}
)

type status struct {
	Status string  `json:"status"`
	Error  errList `json:"error,omitempty"`
}

// MarshalJSON implements json.Marshaler
func (m errList) MarshalJSON() ([]byte, error) {
	if m == nil || len(m) <= 0 {
		return []byte(`""`), nil
	}
	list := make([]json.RawMessage, 0, len(m))
	for _, err := range m {
		if b, marshalErr := json.Marshal(err); marshalErr == nil {
			list = append(list, b)
		} else {
			// fallback to quoted .Error() string if marshalling fails
			list = append(list, []byte(fmt.Sprintf("%q", err.Error())))
		}
	}
	if len(list) == 1 {
		return list[0], nil // for backward compatibility
	}
	return json.Marshal(list)
}

// Error implements error
func (m errList) Error() string {
	if m == nil || len(m) <= 0 {
		return ""
	}
	text := make([]string, 0, len(m))
	for _, err := range m {
		text = append(text, err.Error())
	}
	return strings.Join(text, "\n")
}

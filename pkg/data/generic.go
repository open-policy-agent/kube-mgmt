// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package data

import (
	"time"

	"github.com/Sirupsen/logrus"
	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// GenericSync replicates Kubernetes resources into OPA as raw JSON.
type GenericSync struct {
	kubeconfig *rest.Config
	opa        opa_client.Data
	ns         types.ResourceType
}

const (
	resyncPeriod        = time.Second * 60
	syncResetBackoffMin = time.Second
	syncResetBackoffMax = time.Second * 30
)

// New returns a new GenericSync that cna be started.
func New(kubeconfig *rest.Config, opa opa_client.Data, ns types.ResourceType) *GenericSync {
	cpy := *kubeconfig
	if ns.Group == "" {
		cpy.APIPath = "/api"
	} else {
		cpy.APIPath = "/apis"
	}
	cpy.GroupVersion = &schema.GroupVersion{
		Group:   ns.Group,
		Version: ns.Version,
	}
	cpy.NegotiatedSerializer = dynamic.ContentConfig().NegotiatedSerializer
	return &GenericSync{
		kubeconfig: &cpy,
		ns:         ns,
		opa:        opa.Prefix(ns.Resource),
	}
}

// Run starts the synchronizer. To stop the synchronizer send a message to the
// channel.
func (s *GenericSync) Run() (chan struct{}, error) {
	client, err := rest.RESTClientFor(s.kubeconfig)
	if err != nil {
		return nil, err
	}
	quit := make(chan struct{})
	source := cache.NewListWatchFromClient(
		client,
		s.ns.Resource,
		api.NamespaceAll,
		fields.Everything())
	store, controller := cache.NewInformer(
		source,
		&unstructured.Unstructured{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.syncAdd,
			UpdateFunc: s.update,
			DeleteFunc: s.syncRemove,
		})
	for _, obj := range store.List() {
		s.syncAdd(obj)
	}
	go controller.Run(quit)
	return quit, nil
}

func (s *GenericSync) update(_, obj interface{}) {
	s.syncAdd(obj)
}

func (s *GenericSync) syncAdd(obj interface{}) {
	u := obj.(*unstructured.Unstructured)
	name := u.GetName()
	var path = u.GetName()
	if s.ns.Namespaced {
		path = u.GetNamespace() + "/" + name
	}
	if err := s.opa.PutData(path, u); err != nil {
		logrus.Errorf("Failed to add or update %v/%v (will reset OPA data and resync in %v): %v", s.ns, path, resyncPeriod, err)
		s.syncReset()
	}
}

func (s *GenericSync) syncRemove(obj interface{}) {
	u := obj.(*unstructured.Unstructured)
	name := u.GetName()
	var path = u.GetName()
	if s.ns.Namespaced {
		path = u.GetNamespace() + "/" + name
	}
	if err := s.opa.PatchData(path, "remove", nil); err != nil {
		logrus.Errorf("Failed to remove %v/%v (will reset OPA data and resync in %v): %v", s.ns, path, resyncPeriod, err)
		s.syncReset()
	}
}

func (s *GenericSync) syncReset() {
	d := syncResetBackoffMin
	for {
		if err := s.opa.PutData("/", map[string]interface{}{}); err != nil {
			logrus.Errorf("Failed to reset OPA data for %v (will retry after %v): %v", s.ns, d, err)
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

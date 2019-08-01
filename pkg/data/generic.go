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
	kubeconfig   *rest.Config
	opa          opa_client.Data
	ns           types.ResourceType
	resyncPeriod time.Duration
	controller   cache.Controller
	store        cache.Store
}

const (
	syncResetBackoffMin = time.Second
	syncResetBackoffMax = time.Second * 30
)

// New returns a new GenericSync that cna be started.
func New(kubeconfig *rest.Config, opa opa_client.Data, ns types.ResourceType, resyncPeriod time.Duration) *GenericSync {
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
		kubeconfig:   &cpy,
		ns:           ns,
		opa:          opa.Prefix(ns.Resource),
		resyncPeriod: resyncPeriod,
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
	s.store, s.controller = cache.NewInformer(
		source,
		&unstructured.Unstructured{},
		s.resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    s.syncAdd,
			UpdateFunc: s.update,
			DeleteFunc: s.syncRemove,
		})
	go s.controller.Run(quit)
	return quit, nil
}

// Returns the controller for this syncer
func (s *GenericSync) Controller() cache.Controller {
	return s.controller
}

func (s *GenericSync) update(_, obj interface{}) {
	s.syncAdd(obj)
}

func (s *GenericSync) syncAdd(obj interface{}) {
	if path, err := s.put(obj); err != nil {
		logrus.Errorf("Failed to add or update %v/%v (will reset OPA data and resync in %s): %v", s.ns, path, s.resyncPeriod.String(), err)
		s.syncReset()
	}
}

func (s *GenericSync) addAll() error {
	for _, obj := range s.store.List() {
		if _, err := s.put(obj); err != nil {
			return err
		}
	}
	return nil
}

func (s *GenericSync) put(obj interface{}) (string, error) {
	u := obj.(*unstructured.Unstructured)
	name := u.GetName()
	var path = u.GetName()
	if s.ns.Namespaced {
		path = u.GetNamespace() + "/" + name
	}
	return path, s.opa.PutData(path, u)
}

func (s *GenericSync) syncRemove(obj interface{}) {
	// OnDelete can return an object of type DeletedFinalStateUnknown if the watch event was missed
	staleObj, stale := obj.(cache.DeletedFinalStateUnknown)
	if stale {
		obj = staleObj.Obj
	}
	u := obj.(*unstructured.Unstructured)
	name := u.GetName()
	var path = u.GetName()
	if s.ns.Namespaced {
		path = u.GetNamespace() + "/" + name
	}
	if err := s.opa.PatchData(path, "remove", nil); err != nil {
		logrus.Errorf("Failed to remove %v/%v (will reset OPA data and resync in %s): %v", s.ns, path, s.resyncPeriod.String(), err)
		s.syncReset()
	}
}

func (s *GenericSync) syncReset() {
	d := syncResetBackoffMin
	for {
		if err := s.opa.PutData("/", map[string]interface{}{}); err != nil {
			logrus.Errorf("Failed to reset OPA data for %v (will retry after %s): %v", s.ns, d.String(), err)
		} else {
			if err := s.addAll(); err != nil {
				logrus.Errorf("Failed to reload OPA data for %v (will retry after %s): %v", s.ns, d.String(), err)
			} else {
				return
			}
		}
		time.Sleep(d)
		d = d * 2
		if d > syncResetBackoffMax {
			d = syncResetBackoffMax
		}
	}
}

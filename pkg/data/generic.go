// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// GenericSync replicates Kubernetes resources into OPA as raw JSON.
type GenericSync struct {
	client      dynamic.Interface
	opa         opa_client.Data
	ns          types.ResourceType
	createError error // to support deprecated calls to New / Run
}

// The min/max amount of time to wait when resetting the synchronizer.
const (
	backoffMax = time.Second * 30
	backoffMin = time.Second
)

// New returns a new GenericSync that can be started.
// Deprecated: Please Use NewFromInterface instead.
func New(kubeconfig *rest.Config, opa opa_client.Data, ns types.ResourceType) *GenericSync {
	client, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return &GenericSync{createError: err}
	}
	return NewFromInterface(client, opa, ns)
}

// NewFromInterface returns a new GenericSync that can be started.
func NewFromInterface(client dynamic.Interface, opa opa_client.Data, ns types.ResourceType) *GenericSync {
	return &GenericSync{
		client: client,
		ns:     ns,
		opa:    opa.Prefix(ns.Resource),
	}
}

// Run starts the synchronizer. To stop the synchronizer send a message to the
// channel.
// Deprecated: Please use RunContext instead.
func (s *GenericSync) Run() (chan struct{}, error) {

	// To support legacy way of creating GenericSync from *rest.Config
	if s.createError != nil {
		return nil, s.createError
	}

	quit := make(chan struct{})
	go s.loop(quit)
	return quit, nil
}

// RunContext starts the synchronizer in the foreground.
// To stop the synchronizer, cancel the context.
func (s *GenericSync) RunContext(ctx context.Context) error {
	if s.createError != nil {
		return s.createError
	}
	s.loop(ctx.Done())
	return nil
}

func (s *GenericSync) loop(quit <-chan struct{}) {

	defer func() {
		logrus.Infof("Sync for %v finished. Exiting.", s.ns)
	}()

	resource := s.client.Resource(schema.GroupVersionResource{
		Group:    s.ns.Group,
		Version:  s.ns.Version,
		Resource: s.ns.Resource,
	})

	delay := backoffMin

	for {

		err := s.sync(resource, quit)
		if err == nil {
			return
		}

		switch err.(type) {

		case errChannelClosed:
			logrus.Infof("Sync channel for %v closed. Restarting immediately.", s.ns)
			delay = backoffMin

		case errOPA:
			logrus.Errorf("Sync for %v failed due to OPA error. Trying again in %v. Reason: %v", s.ns, delay, err)
			delay = backoffMin
			t := time.NewTimer(delay)
			select {
			case <-t.C:
				break
			case <-quit:
				return
			}

		case errKubernetes:
			logrus.Errorf("Sync for %v failed due to Kubernetes error. Trying again in %v. Reason: %v", s.ns, delay, err)
			delay *= 2
			if delay > backoffMax {
				delay = backoffMax
			}
			t := time.NewTimer(delay)
			select {
			case <-t.C:
				break
			case <-quit:
				return
			}
		}
	}
}

type errKubernetes struct{ error }

type errOPA struct{ error }

type errChannelClosed struct{}

func (err errKubernetes) Unwrap() error {
	return err.error
}

func (err errOPA) Unwrap() error {
	return err.error
}

func (errChannelClosed) Error() string {
	return "channel closed"
}

// sync starts replicating Kubernetes resources into OPA. If an error occurs
// during the replication process this function returns and indicates whether
// the synchronizer should backoff. The synchronizer will backoff whenever the
// Kubernetes API returns an error.
func (s *GenericSync) sync(resource dynamic.NamespaceableResourceInterface, quit <-chan struct{}) error {

	logrus.Infof("Syncing %v.", s.ns)
	tList := time.Now()
	result, err := resource.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return errKubernetes{fmt.Errorf("list: %w", err)}
	}

	dList := time.Since(tList)
	resourceVersion := result.GetResourceVersion()
	logrus.Infof("Listed %v and got %v resources with resourceVersion %v. Took %v.", s.ns, len(result.Items), resourceVersion, dList)

	tLoad := time.Now()

	if err := s.syncAll(result.Items); err != nil {
		return errOPA{fmt.Errorf("reset: %w", err)}
	}

	dLoad := time.Since(tLoad)
	logrus.Infof("Loaded %v resources into OPA. Took %v. Starting watch at resourceVersion %v.", s.ns, dLoad, resourceVersion)

	w, err := resource.Watch(context.TODO(), metav1.ListOptions{
		ResourceVersion: resourceVersion,
	})
	if err != nil {
		return errKubernetes{fmt.Errorf("watch: %w", err)}
	}

	defer w.Stop()

	ch := w.ResultChan()

	for {
		select {
		case evt := <-ch:
			switch evt.Type {
			case watch.Added:
				err := s.syncAdd(evt.Object)
				if err != nil {
					return errOPA{fmt.Errorf("add event: %w", err)}
				}
			case watch.Modified:
				err := s.syncAdd(evt.Object)
				if err != nil {
					return errOPA{fmt.Errorf("modify event: %w", err)}
				}
			case watch.Deleted:
				err := s.syncRemove(evt.Object)
				if err != nil {
					return errOPA{fmt.Errorf("delete event: %w", err)}
				}
			case watch.Error:
				return errKubernetes{fmt.Errorf("error event: %v", evt.Object)}
			default:
				return errChannelClosed{}
			}
		case <-quit:
			return nil
		}
	}
}

func (s *GenericSync) syncAdd(obj runtime.Object) error {
	path, err := objPath(obj, s.ns.Namespaced)
	if err != nil {
		return err
	}
	return s.opa.PutData(path, obj)
}

func (s *GenericSync) syncRemove(obj runtime.Object) error {
	path, err := objPath(obj, s.ns.Namespaced)
	if err != nil {
		return err
	}
	return s.opa.PatchData(path, "remove", nil)
}

func (s *GenericSync) syncAll(objs []unstructured.Unstructured) error {

	// Build a list of patches to apply.
	payload, err := generateSyncPayload(objs, s.ns.Namespaced)
	if err != nil {
		return err
	}

	return s.opa.PutData("/", payload)
}

func generateSyncPayload(objs []unstructured.Unstructured, namespaced bool) (map[string]interface{}, error) {
	combined := make(map[string]interface{}, len(objs))
	for _, obj := range objs {
		path, err := objPath(&obj, namespaced)
		if err != nil {
			return nil, err
		}

		// Ensure the path in thee map up to our value exists
		// We make some assumptions about the paths that do exist
		// being the correct types due to the expected uniform
		// objPath's for each of the similar object types being
		// sync'd with the GenericSync instance.
		segments := strings.Split(path, "/")
		dir := combined
		for i := 0; i < len(segments)-1; i++ {
			next, ok := combined[segments[i]]
			if !ok {
				next = map[string]interface{}{}
				dir[segments[i]] = next
			}
			dir = next.(map[string]interface{})
		}
		dir[segments[len(segments)-1]] = obj.Object
	}

	return combined, nil
}

func objPath(obj runtime.Object, namespaced bool) (string, error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}
	name := m.GetName()
	var path string
	if namespaced {
		path = m.GetNamespace() + "/" + name
	} else {
		path = name
	}
	return path, nil
}

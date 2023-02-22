// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// The min/max amount of time to wait when resetting the synchronizer.
const (
	backoffMax   = time.Second * 30
	backoffMin   = time.Second
	jitterFactor = 1.2
	FieldMeta    = "metadata.namespace!="
)

// GenericSync replicates Kubernetes resources into OPA as raw JSON.
type GenericSync struct {
	createError  error // to support deprecated calls to New / Run
	client       dynamicClient
	opa          opa_client.Data
	ns           types.ResourceType
	limiter      workqueue.RateLimiter
	jitterFactor float64
}

// New returns a new GenericSync that can be started.
// Deprecated: Please Use NewFromInterface instead.
func New(kubeconfig *rest.Config, opa opa_client.Data, ns types.ResourceType) *GenericSync {
	client, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return &GenericSync{createError: err}
	}
	return NewFromInterface(client, opa, ns)
}

type Option func(s *GenericSync)

// NewFromInterface returns a new GenericSync that can be started.
func NewFromInterface(client dynamic.Interface, opa opa_client.Data, ns types.ResourceType, opts ...Option) *GenericSync {
	s := &GenericSync{
		client:       dynamicClient{client},
		ns:           ns,
		opa:          opa.Prefix(ns.Resource),
		jitterFactor: jitterFactor,
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.limiter == nil { // Use default rateLimiter if not configured
		s.limiter = workqueue.NewItemExponentialFailureRateLimiter(backoffMin, backoffMax)
	}
	return s
}

// WithBackoff tunes the values of exponential backoff and jitter factor
func WithBackoff(min, max time.Duration, jitterFactor float64) Option {
	return func(s *GenericSync) {
		s.limiter = workqueue.NewItemExponentialFailureRateLimiter(min, max)
		s.jitterFactor = jitterFactor
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
	ctx, cancel := context.WithCancel(context.Background())
	go func() { // propagate cancel signal from channel to context
		<-quit
		cancel()
	}()
	var ignoreNamespaces []string
	go s.RunContext(ctx, ignoreNamespaces)
	return quit, nil
}

// RunContext starts the synchronizer in the foreground.
// To stop the synchronizer, cancel the context.
func (s *GenericSync) RunContext(ctx context.Context, ignoreNamespaces []string) error {
	if s.createError != nil {
		return s.createError
	}

	store, queue := s.setup(ctx, ignoreNamespaces)
	go func() {
		<-ctx.Done()
		queue.ShutDown()
	}()

	s.loop(store, queue)
	return nil
}

// setup the store and queue for this GenericSync instance
func (s *GenericSync) setup(ctx context.Context, ignoreNamespaces []string) (cache.SharedInformer, workqueue.DelayingInterface) {

	resource := s.client.ResourceFor(s.ns, metav1.NamespaceAll)
	queue := workqueue.NewNamedDelayingQueue(s.ns.String())
	var ignoreNs string
	if len(ignoreNamespaces) >= 1 {
		for _, ns := range ignoreNamespaces {
			ignoreNs = FieldMeta + ns + "," + ignoreNs
		}
	}

	ignoreNs = strings.TrimSuffix(ignoreNs, ",")

	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = ignoreNs
				return resource.List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = ignoreNs
				return resource.Watch(ctx, options)
			},
		},
		&unstructured.Unstructured{},
		0,
		cache.Indexers{},
	)

	informer.AddEventHandler(resourceEventQueue{queue})
	start, quit := time.Now(), ctx.Done()
	go informer.Run(quit)
	for !cache.WaitForCacheSync(quit, informer.HasSynced) {
		logrus.Warnf("Failed to sync cache for %v, retrying...", s.ns)
	}
	if informer.HasSynced() {
		logrus.Infof("Initial informer sync for %v completed, took %v", s.ns, time.Since(start))
	}

	//Add the list after initial startup
	for _, item := range informer.GetStore().ListKeys() {
		queue.AddAfter(item, 1*time.Second)
	}

	return informer, queue
}

// resourceEventQueue is a cache.ResourceEventHandler that queues all events
type resourceEventQueue struct {
	workqueue.Interface
}

// OnAdd implements ResourceHandler
func (q resourceEventQueue) OnAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logrus.Warnf("failed to retrieve key: %v", err)
		return
	}
	whatisthis, _ := meta.Accessor(obj)
	logrus.Infof("added que %s", whatisthis.GetName())
	q.Add(key)
}

func (q resourceEventQueue) resourceVersionMatch(oldObj, newObj interface{}) bool {
	var (
		oldMeta metav1.Object
		newMeta metav1.Object
		err     error
	)
	oldMeta, err = meta.Accessor(oldObj)
	if err == nil {
		newMeta, err = meta.Accessor(newObj)
	}
	if err != nil {
		logrus.Warnf("failed to retrieve meta: %v", err)
		return false
	}
	return newMeta.GetResourceVersion() == oldMeta.GetResourceVersion()
}

func (q resourceEventQueue) resourceIsElection(newObj interface{}) bool {
	var (
		newMeta metav1.Object
		err     error
	)
	newMeta, err = meta.Accessor(newObj)
	if err != nil {
		logrus.Warnf("failed to retrieve meta: %v", err)
		return false
	}
	annotations := newMeta.GetAnnotations()
	_, ok := annotations["control-plane.alpha.kubernetes.io/leader"]

	return ok
}

// OnUpdate implements ResourceHandler
func (q resourceEventQueue) OnUpdate(oldObj, newObj interface{}) {
	if !q.resourceVersionMatch(oldObj, newObj) { // Avoid sync flood on relist. We don't use resync.
		if !q.resourceIsElection(newObj) {
			q.OnAdd(newObj)
		}
	}
}

// OnDelete implements ResourceHandler
func (q resourceEventQueue) OnDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logrus.Warnf("failed to retrieve key: %v", err)
		return
	}
	q.Add(key)
}

const initPath = ""

// loop starts replicating Kubernetes resources into OPA. If an error occurs
// during the replication process, this function will backoff and reload
// all resources into OPA from scratch.
func (s *GenericSync) loop(store cache.SharedInformer, queue workqueue.DelayingInterface) {

	logrus.Infof("Syncing %v.", s.ns)
	defer func() {
		logrus.Infof("Sync for %v finished. Exiting.", s.ns)
	}()

	var delay time.Duration
	for !queue.ShuttingDown() {

		queue.AddAfter(initPath, delay) // this special path will trigger a full load
		syncDone := false               // discard everything until initPath

		var err error
		for err == nil {
			key, shuttingDown := queue.Get()
			if shuttingDown {
				return
			}
			err = s.processNext(store, key.(string), &syncDone)
			if key == initPath && syncDone {
				s.limiter.Forget(initPath)
			}
			logrus.Debugf("queue length: %d", queue.Len())
			queue.Done(key)
		}

		delay := wait.Jitter(s.limiter.When(initPath), s.jitterFactor)
		logrus.Errorf("Sync for %v failed, trying again in %v. Reason: %v", s.ns, delay, err)
	}
}

func (s *GenericSync) processNext(store cache.SharedInformer, path string, syncDone *bool) error {

	// On receiving the initPath, load a full dump of the data store
	if path == initPath {
		if *syncDone {
			return nil
		}
		start, list := time.Now(), store.GetStore().List()
		if err := s.syncAll(list); err != nil {
			return err
		}
		logrus.Infof("Loaded %d resources of kind %v into OPA. Took %v", len(list), s.ns, time.Since(start))
		*syncDone = true // sync is now Done
		return nil
	}

	// Ignore updates queued before the initial load
	if !*syncDone {
		return nil
	}

	obj, exists, err := store.GetStore().GetByKey(path)
	if err != nil {
		return fmt.Errorf("store error: %w", err)
	}
	if exists {
		if err := s.opa.PutData(path, obj); err != nil {
			return fmt.Errorf("add event: %w", err)
		}
	} else {
		if err := s.opa.PatchData(path, "remove", nil); err != nil {
			return fmt.Errorf("delete event: %w", err)
		}
	}
	return nil
}

func (s *GenericSync) syncAll(objs []interface{}) error {

	// Build a list of patches to apply.
	payload, err := generateSyncPayload(objs, s.ns.Namespaced)
	if err != nil {
		return err
	}

	return s.opa.PutData("/", payload)
}

func generateSyncPayload(objs []interface{}, namespaced bool) (map[string]interface{}, error) {
	combined := make(map[string]interface{}, len(objs))
	for _, obj := range objs {
		path, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			return nil, err
		}

		// Ensure the path in the map up to our value exists
		// We make some assumptions about the paths that do exist
		// being the correct types due to the expected uniform
		// paths for each of the similar object types being
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
		dir[segments[len(segments)-1]] = obj
	}

	return combined, nil
}

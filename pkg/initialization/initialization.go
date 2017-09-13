// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package initialization

import (
	"encoding/json"
	"time"

	"github.com/Sirupsen/logrus"
	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8s_types "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/apis/admissionregistration/v1alpha1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// Initializer implements an initializer controller that queries OPA for values
// to apply to new resources. The values are applied to the resource using the
// strategic-merge-patch method.
type Initializer struct {
	kubeconfig *rest.Config
	ns         types.ResourceType
	opa        opa_client.Data
	client     *rest.RESTClient
	name       string
	owner      metav1.OwnerReference
}

// New returns a new Initializer that can be started.
func New(kubeconfig *rest.Config, opa opa_client.Data, ns types.ResourceType, name string, owner metav1.OwnerReference) *Initializer {
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
	return &Initializer{
		kubeconfig: &cpy,
		name:       name,
		ns:         ns,
		opa:        opa,
		owner:      owner,
	}
}

// Run starts the initializer and returns a channel that can be used to stop
// it.
func (i *Initializer) Run() (chan struct{}, error) {

	if err := i.declare(); err != nil {
		return nil, err
	}

	var err error
	i.client, err = rest.RESTClientFor(i.kubeconfig)
	if err != nil {
		return nil, err
	}

	quit := make(chan struct{})

	source := cache.NewListWatchFromClient(
		i.client,
		i.ns.Resource,
		api.NamespaceAll,
		fields.Everything())

	sourcePreInitialization := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.IncludeUninitialized = true
			return source.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.IncludeUninitialized = true
			return source.Watch(options)
		},
	}

	_, controller := cache.NewInformer(
		sourcePreInitialization,
		&unstructured.Unstructured{},
		time.Second*60,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				u := obj.(*unstructured.Unstructured)
				if err := i.initializeResource(u); err != nil {
					logrus.Errorf("Failed to initialize %v object %v/%v: %v", i.ns.Resource, u.GetNamespace(), u.GetName(), err)
				}
			},
		})

	go controller.Run(quit)

	return quit, nil
}

func (i *Initializer) initializeResource(u *unstructured.Unstructured) error {

	initializers, ok := getUpdatedInitializers(u, i.name)
	if !ok {
		return nil
	}

	merge, err := queryOPA(i.opa, u)
	if err != nil {
		return err
	}

	metadata, ok := merge["metadata"].(map[string]interface{})
	if !ok {
		metadata = map[string]interface{}{}
		merge["metadata"] = metadata
	}

	metadata["initializers"] = initializers

	bs, err := json.Marshal(merge)
	if err != nil {
		return err
	}

	result := i.client.Patch(k8s_types.StrategicMergePatchType).
		Resource(i.ns.Resource).
		NamespaceIfScoped(u.GetNamespace(), i.ns.Namespaced).
		Name(u.GetName()).
		Body(bs).
		Do()

	if err := result.Error(); err != nil {
		return err
	}

	return nil
}

func (i *Initializer) declare() error {
	clientset, err := kubernetes.NewForConfig(i.kubeconfig)
	if err != nil {
		return err
	}

	client := clientset.InitializerConfigurations()
	_, err = client.Get(i.name, metav1.GetOptions{})
	if err == nil {
		if err := client.Delete(i.name, nil); err != nil {
			return err
		}
	}

	for {
		time.Sleep(time.Second * 1)
		initConfig := &v1alpha1.InitializerConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: i.name,
				OwnerReferences: []metav1.OwnerReference{
					i.owner,
				},
			},
			Initializers: []v1alpha1.Initializer{
				{
					Name: i.name,
					Rules: []v1alpha1.Rule{
						{
							APIGroups:   []string{"*"},
							APIVersions: []string{"*"},
							Resources:   []string{i.ns.Resource},
						},
					},
				},
			},
		}
		if _, err := client.Create(initConfig); err != nil {
			logrus.Errorf("Failed to declare initializer configuration: %v", err)
		} else {
			logrus.Infof("Registered initializer configuration: %v", i.name)
			return nil
		}
	}
}

func queryOPA(opa opa_client.Data, obj interface{}) (map[string]interface{}, error) {
	result, err := opa.PostData("", obj)
	if opa_client.IsUndefinedErr(err) {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	var merge map[string]interface{}
	return merge, json.Unmarshal(result, &merge)
}

func getUpdatedInitializers(u *unstructured.Unstructured, name string) (result *metav1.Initializers, ok bool) {
	is := u.GetInitializers()
	if is == nil {
		return nil, false
	}
	if len(is.Pending) == 0 {
		return nil, false
	}
	if is.Pending[0].Name != name {
		return nil, false
	}
	if len(is.Pending) == 1 {
		return nil, true
	}
	is.Pending = is.Pending[1:]
	return is, true
}

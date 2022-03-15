package data

import (
	"github.com/open-policy-agent/kube-mgmt/pkg/types"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// resourceInterface knows how to use the method `Resource` of dynamic.Interface
type resourceInterface interface {
	Resource(schema.GroupVersionResource) dynamic.NamespaceableResourceInterface
}

// dynamicClient wraps a resourceInterface with some utilities
type dynamicClient struct {
	resourceInterface
}

// ResourceFor builds a dynamic.ResourceInterface for a ResourceType
func (f dynamicClient) ResourceFor(resourceType types.ResourceType, namespace string) dynamic.ResourceInterface {
	resource := f.Resource(schema.GroupVersionResource{
		Group:    resourceType.Group,
		Version:  resourceType.Version,
		Resource: resourceType.Resource,
	})
	if resourceType.Namespaced {
		return resource.Namespace(namespace)
	}
	return resource
}

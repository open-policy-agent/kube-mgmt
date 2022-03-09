package data

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/open-policy-agent/kube-mgmt/pkg/types"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestGenericSync(t *testing.T) {

	testCases := []struct {
		label        string
		resourceType types.ResourceType
		prefix       string
		objs         []runtime.Object
		expected     string
	}{
		{
			label: "Single Cluster Resource",
			resourceType: types.ResourceType{
				Namespaced: false,
				Resource:   "nodes",
				Version:    "v1",
			},
			prefix: "",
			objs: []runtime.Object{
				&apiv1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Spec:   apiv1.NodeSpec{},
					Status: apiv1.NodeStatus{},
				},
			},
			expected: `{
				"node1":{
					"apiVersion": "v1",
					"kind": "Node",
					"metadata":{
						"creationTimestamp":null,
						"name":"node1"
					},
					"spec":{
					},
					"status":{
						"daemonEndpoints":{
							"kubeletEndpoint":{
								"Port":0
							}
						},
						"nodeInfo":{
							"architecture":"",
							"bootID":"",
							"containerRuntimeVersion":"",
							"kernelVersion":"",
							"kubeProxyVersion":"",
							"kubeletVersion":"",
							"machineID":"",
							"operatingSystem":"",
							"osImage":"",
							"systemUUID":""
						}
					}
				}
			}`,
		},
		{
			label: "Single Cluster Resource With Prefix",
			resourceType: types.ResourceType{
				Namespaced: false,
				Resource:   "nodes",
				Version:    "v1",
			},
			prefix: "kube",
			objs: []runtime.Object{
				&apiv1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Spec:   apiv1.NodeSpec{},
					Status: apiv1.NodeStatus{},
				},
			},
			expected: `{
				"node1":{
					"apiVersion": "v1",
					"kind": "Node",
					"metadata":{
						"creationTimestamp":null,
						"name":"node1"
					},
					"spec":{
					},
					"status":{
						"daemonEndpoints":{
							"kubeletEndpoint":{
								"Port":0
							}
						},
						"nodeInfo":{
							"architecture":"",
							"bootID":"",
							"containerRuntimeVersion":"",
							"kernelVersion":"",
							"kubeProxyVersion":"",
							"kubeletVersion":"",
							"machineID":"",
							"operatingSystem":"",
							"osImage":"",
							"systemUUID":""
						}
					}
				}
			}`,
		},
		{
			label: "Multiple Cluster Resources With Prefix",
			resourceType: types.ResourceType{
				Namespaced: false,
				Resource:   "nodes",
				Version:    "v1",
			},
			prefix: "kube",
			objs: []runtime.Object{
				&apiv1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Spec:   apiv1.NodeSpec{},
					Status: apiv1.NodeStatus{},
				},
				&apiv1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Spec:   apiv1.NodeSpec{},
					Status: apiv1.NodeStatus{},
				},
				&apiv1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node3",
					},
					Spec:   apiv1.NodeSpec{},
					Status: apiv1.NodeStatus{},
				},
			},
			expected: `{
				"node1":{
					"apiVersion": "v1",
					"kind": "Node",
					"metadata":{
						"creationTimestamp":null,
						"name":"node1"
					},
					"spec":{
					},
					"status":{
						"daemonEndpoints":{
							"kubeletEndpoint":{
								"Port":0
							}
						},
						"nodeInfo":{
							"architecture":"",
							"bootID":"",
							"containerRuntimeVersion":"",
							"kernelVersion":"",
							"kubeProxyVersion":"",
							"kubeletVersion":"",
							"machineID":"",
							"operatingSystem":"",
							"osImage":"",
							"systemUUID":""
						}
					}
				},
				"node2":{
					"apiVersion": "v1",
					"kind": "Node",
					"metadata":{
						"creationTimestamp":null,
						"name":"node2"
					},
					"spec":{
					},
					"status":{
						"daemonEndpoints":{
							"kubeletEndpoint":{
								"Port":0
							}
						},
						"nodeInfo":{
							"architecture":"",
							"bootID":"",
							"containerRuntimeVersion":"",
							"kernelVersion":"",
							"kubeProxyVersion":"",
							"kubeletVersion":"",
							"machineID":"",
							"operatingSystem":"",
							"osImage":"",
							"systemUUID":""
						}
					}
				},
				"node3":{
					"apiVersion": "v1",
					"kind": "Node",
					"metadata":{
						"creationTimestamp":null,
						"name":"node3"
					},
					"spec":{
					},
					"status":{
						"daemonEndpoints":{
							"kubeletEndpoint":{
								"Port":0
							}
						},
						"nodeInfo":{
							"architecture":"",
							"bootID":"",
							"containerRuntimeVersion":"",
							"kernelVersion":"",
							"kubeProxyVersion":"",
							"kubeletVersion":"",
							"machineID":"",
							"operatingSystem":"",
							"osImage":"",
							"systemUUID":""
						}
					}
				}
			}`,
		},
		{
			label: "Single Namespaced Resource",
			resourceType: types.ResourceType{
				Namespaced: true,
				Resource:   "pods",
				Version:    "v1",
			},
			prefix: "",
			objs: []runtime.Object{
				&apiv1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns1",
					},
					Spec:   apiv1.PodSpec{},
					Status: apiv1.PodStatus{},
				},
			},
			expected: `{
				"ns1":{
					"pod1":{
						"apiVersion": "v1",
						"kind": "Pod",
						"metadata":{
							"creationTimestamp":null,
							"name":"pod1",
							"namespace":"ns1"
						},
						"spec":{
							"containers":null
						},
						"status":{
						}
					}
				}
			}`,
		},
		{
			label: "Single Namespaced Resource With Prefix",
			resourceType: types.ResourceType{
				Namespaced: true,
				Resource:   "pods",
				Version:    "v1",
			},
			prefix: "kube",
			objs: []runtime.Object{
				&apiv1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns1",
					},
					Spec:   apiv1.PodSpec{},
					Status: apiv1.PodStatus{},
				},
			},
			expected: `{
				"ns1":{
					"pod1":{
						"apiVersion": "v1",
						"kind": "Pod",
						"metadata":{
							"creationTimestamp":null,
							"name":"pod1",
							"namespace":"ns1"
						},
						"spec":{
							"containers":null
						},
						"status":{
						}
					}
				}
			}`,
		},
		{
			label: "Multiple Namespaced Resources With Prefix",
			resourceType: types.ResourceType{
				Namespaced: true,
				Resource:   "pods",
				Version:    "v1",
			},
			prefix: "kube",
			objs: []runtime.Object{
				&apiv1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns1",
					},
					Spec:   apiv1.PodSpec{},
					Status: apiv1.PodStatus{},
				},
				&apiv1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod2",
						Namespace: "ns1",
					},
					Spec:   apiv1.PodSpec{},
					Status: apiv1.PodStatus{},
				},
				&apiv1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod1",
						Namespace: "ns2",
					},
					Spec:   apiv1.PodSpec{},
					Status: apiv1.PodStatus{},
				},
			},
			expected: `{
				"ns1":{
					"pod1":{
						"apiVersion": "v1",
						"kind": "Pod",
						"metadata":{
							"creationTimestamp":null,
							"name":"pod1",
							"namespace":"ns1"
						},
						"spec":{
							"containers":null
						},
						"status":{
						}
					},
					"pod2":{
						"apiVersion": "v1",
						"kind": "Pod",
						"metadata":{
							"creationTimestamp":null,
							"name":"pod2",
							"namespace":"ns1"
						},
						"spec":{
							"containers":null
						},
						"status":{
						}
					}
				},
				"ns2":{
					"pod1": {
						"apiVersion": "v1",
						"kind": "Pod",
						"metadata":{
							"creationTimestamp":null,
							"name":"pod1",
							"namespace":"ns2"
						},
						"spec":{
							"containers":null
						},
						"status":{
						}
					}
				}
			}`,
		},
	}

	sc := runtime.NewScheme()
	if err := scheme.AddToScheme(sc); err != nil {
		t.Fatalf("Failed to build initial scheme: %v", err)
	}

	for _, tc := range testCases {

		var expectedJson map[string]interface{}
		mustUnmarshalJSON(t, []byte(tc.expected), &expectedJson)

		t.Run(fmt.Sprintf("%s - GenerateSyncPayload", tc.label), func(t *testing.T) {
			result := mustGenerateSyncPayload(t, tc.resourceType, tc.objs)
			if !reflect.DeepEqual(result, expectedJson) {
				t.Errorf("Sync payload expected:\n\n%v\n\nActual:\n\n%v\n", expectedJson, result)
				t.Fail()
			}
		})

		t.Run(fmt.Sprintf("%s - Run", tc.label), func(t *testing.T) {
			result := mustRun(t, sc, tc.resourceType, tc.prefix, tc.objs)
			if !reflect.DeepEqual(result, expectedJson) {
				t.Errorf("Fake data expected:\n\n%v\n\nActual:\n\n%v\n", expectedJson, result)
				t.Fail()
			}
		})
	}
}

func mustGenerateSyncPayload(t *testing.T, resourceType types.ResourceType, objs []runtime.Object) map[string]interface{} {
	t.Helper()
	data := make([]unstructured.Unstructured, 0, len(objs))
	for _, obj := range objs {
		data = append(data, mustUnstructure(t, obj))
	}
	patches, err := generateSyncPayload(data, resourceType.Namespaced)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	return mustJSONRoundTrip(t, patches)
}

func mustRun(t *testing.T, scheme *runtime.Scheme, resourceType types.ResourceType, prefix string, objs []runtime.Object) map[string]interface{} {

	t.Helper()
	client := fake.NewSimpleDynamicClient(scheme, objs...)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))

	fake := &mockData{
		onUpdate: cancel,
	}
	sync := NewFromInterface(client, fake.Prefix(prefix), resourceType)
	sync.RunContext(ctx)

	// Check prefixes are stacked as expected, before returning
	expectedPrefix := []string{prefix, resourceType.Resource}
	if !reflect.DeepEqual(fake.prefix, expectedPrefix) {
		t.Errorf("Expected prefix to be %s, got %s", expectedPrefix, fake.prefix)
		t.Fail()
	}
	return mustJSONRoundTrip(t, fake.data)
}

func mustMarshalJSON(t *testing.T, obj interface{}) []byte {
	t.Helper()
	bs, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("error marshalling JSON: %s", err)
	}
	return bs
}

func mustUnmarshalJSON(t *testing.T, bs []byte, v interface{}) {
	t.Helper()
	if len(bs) > 0 {
		err := json.Unmarshal(bs, v)
		if err != nil {
			t.Fatalf("error unmarshalling JSON: %s", err)
		}
	}
}

func mustJSONRoundTrip(t *testing.T, obj interface{}) map[string]interface{} {
	t.Helper()

	bs := mustMarshalJSON(t, obj)
	var out map[string]interface{}
	mustUnmarshalJSON(t, bs, &out)

	return out
}

func mustUnstructure(t *testing.T, obj runtime.Object) unstructured.Unstructured {
	t.Helper()
	current, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatalf("Failed to convert runtime Object to unstructured: %v", err)
	}
	return unstructured.Unstructured{Object: current}
}

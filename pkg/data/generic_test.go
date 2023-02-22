package data

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/open-policy-agent/kube-mgmt/internal/expect"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

type testCase struct {
	Label        string
	ResourceType types.ResourceType
	Prefix       string
	Objs         []runtime.Object
	Expected     string
}

// NewFakeDynamicClient builds a new FakeDynamicClient
func newFakeDynamicClient(t *testing.T, objs ...runtime.Object) dynamicClient {
	sc := runtime.NewScheme()
	if err := scheme.AddToScheme(sc); err != nil {
		t.Fatalf("Failed to build initial scheme: %v", err)
	}
	return dynamicClient{resourceInterface: fake.NewSimpleDynamicClient(sc, objs...)}
}

func TestGenericSync(t *testing.T) {
	t.Parallel()

	testCases := []testCase{
		{
			Label: "Single Cluster Resource",
			ResourceType: types.ResourceType{
				Namespaced: false,
				Resource:   "nodes",
				Version:    "v1",
			},
			Prefix: "",
			Objs: []runtime.Object{
				&apiv1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "node1",
						ResourceVersion: "0",
					},
					Spec:   apiv1.NodeSpec{},
					Status: apiv1.NodeStatus{},
				},
			},
			Expected: `{
				"node1":{
					"apiVersion": "v1",
					"kind": "Node",
					"metadata":{
						"creationTimestamp":null,
						"name":"node1",
						"resourceVersion":"0"
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
			Label: "Single Cluster Resource With Prefix",
			ResourceType: types.ResourceType{
				Namespaced: false,
				Resource:   "nodes",
				Version:    "v1",
			},
			Prefix: "kube",
			Objs: []runtime.Object{
				&apiv1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "node1",
						ResourceVersion: "0",
					},
					Spec:   apiv1.NodeSpec{},
					Status: apiv1.NodeStatus{},
				},
			},
			Expected: `{
				"node1":{
					"apiVersion": "v1",
					"kind": "Node",
					"metadata":{
						"creationTimestamp":null,
						"name":"node1",
						"resourceVersion":"0"
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
			Label: "Multiple Cluster Resources With Prefix",
			ResourceType: types.ResourceType{
				Namespaced: false,
				Resource:   "nodes",
				Version:    "v1",
			},
			Prefix: "kube",
			Objs: []runtime.Object{
				&apiv1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "node1",
						ResourceVersion: "0",
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
						Name:            "node2",
						ResourceVersion: "0",
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
						Name:            "node3",
						ResourceVersion: "0",
					},
					Spec:   apiv1.NodeSpec{},
					Status: apiv1.NodeStatus{},
				},
			},
			Expected: `{
				"node1":{
					"apiVersion": "v1",
					"kind": "Node",
					"metadata":{
						"creationTimestamp":null,
						"name":"node1",
						"resourceVersion":"0"
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
						"name":"node2",
						"resourceVersion":"0"
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
						"name":"node3",
						"resourceVersion":"0"
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
			Label: "Single Namespaced Resource",
			ResourceType: types.ResourceType{
				Namespaced: true,
				Resource:   "pods",
				Version:    "v1",
			},
			Prefix: "",
			Objs: []runtime.Object{
				&apiv1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						Namespace:       "ns1",
						ResourceVersion: "0",
					},
					Spec:   apiv1.PodSpec{},
					Status: apiv1.PodStatus{},
				},
			},
			Expected: `{
				"ns1":{
					"pod1":{
						"apiVersion": "v1",
						"kind": "Pod",
						"metadata":{
							"creationTimestamp":null,
							"name":"pod1",
							"namespace":"ns1",
							"resourceVersion":"0"
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
			Label: "Single Namespaced Resource With Prefix",
			ResourceType: types.ResourceType{
				Namespaced: true,
				Resource:   "pods",
				Version:    "v1",
			},
			Prefix: "kube",
			Objs: []runtime.Object{
				&apiv1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						Namespace:       "ns1",
						ResourceVersion: "0",
					},
					Spec:   apiv1.PodSpec{},
					Status: apiv1.PodStatus{},
				},
			},
			Expected: `{
				"ns1":{
					"pod1":{
						"apiVersion": "v1",
						"kind": "Pod",
						"metadata":{
							"creationTimestamp":null,
							"name":"pod1",
							"namespace":"ns1",
							"resourceVersion":"0"
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
			Label: "Multiple Namespaced Resources With Prefix",
			ResourceType: types.ResourceType{
				Namespaced: true,
				Resource:   "pods",
				Version:    "v1",
			},
			Prefix: "kube",
			Objs: []runtime.Object{
				&apiv1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            "pod1",
						Namespace:       "ns1",
						ResourceVersion: "0",
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
						Name:            "pod2",
						Namespace:       "ns1",
						ResourceVersion: "0",
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
						Name:            "pod1",
						Namespace:       "ns2",
						ResourceVersion: "0",
					},
					Spec:   apiv1.PodSpec{},
					Status: apiv1.PodStatus{},
				},
			},
			Expected: `{
				"ns1":{
					"pod1":{
						"apiVersion": "v1",
						"kind": "Pod",
						"metadata":{
							"creationTimestamp":null,
							"name":"pod1",
							"namespace":"ns1",
							"resourceVersion":"0"
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
							"namespace":"ns1",
							"resourceVersion":"0"
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
							"namespace":"ns2",
							"resourceVersion":"0"
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

	for _, tc := range testCases {

		tc := tc // We will be running the tests in parallel, so avoid issues with loop var
		expected := expect.MustMarshal(t, expect.MustUnmarshal(t, []byte(tc.Expected)))

		t.Run(fmt.Sprintf("%s - Must Generate Sync Payload", tc.Label), func(t *testing.T) {
			t.Parallel()
			tc.testGenerateSyncPayload(t, expected)
		})

		t.Run(fmt.Sprintf("%s - Must Load Existing Resources", tc.Label), func(t *testing.T) {
			t.Parallel()
			tc.testLoad(t, expected)
		})

		t.Run(fmt.Sprintf("%s - Must Add New Resources", tc.Label), func(t *testing.T) {
			t.Parallel()
			tc.testAdd(t)
		})

		t.Run(fmt.Sprintf("%s - Must Remove Resources", tc.Label), func(t *testing.T) {
			t.Parallel()
			tc.testDelete(t)
		})

		t.Run(fmt.Sprintf("%s - Must Update Resources", tc.Label), func(t *testing.T) {
			t.Parallel()
			tc.testUpdate(t)
		})

		t.Run(fmt.Sprintf("%s - Must Retry Load On Error", tc.Label), func(t *testing.T) {
			t.Parallel()
			tc.testRetryLoad(t, expected)
		})

		t.Run(fmt.Sprintf("%s - Must Retry Add On Error", tc.Label), func(t *testing.T) {
			t.Parallel()
			tc.testRetryAdd(t)
		})

		t.Run(fmt.Sprintf("%s - Must Retry Update On Error", tc.Label), func(t *testing.T) {
			t.Parallel()
			tc.testRetryUpdate(t)
		})

		t.Run(fmt.Sprintf("%s - Must Retry Delete On Error", tc.Label), func(t *testing.T) {
			t.Parallel()
			tc.testRetryDelete(t)
		})
	}
}

func TestEventQueue(t *testing.T) {
	t.Parallel()

	tc := testCase{
		Label: "Single Cluster Resource",
		ResourceType: types.ResourceType{
			Namespaced: false,
			Resource:   "nodes",
			Version:    "v1",
		},
		Prefix: "",
		Objs: []runtime.Object{
			&apiv1.Node{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Node",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            "node1",
					ResourceVersion: "0",
				},
				Spec:   apiv1.NodeSpec{},
				Status: apiv1.NodeStatus{},
			},
		},
		Expected: `{
				"node1":{
					"apiVersion": "v1",
					"kind": "Node",
					"metadata":{
						"creationTimestamp":null,
						"name":"node1",
						"resourceVersion":"0"
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
	}

	t.Run(fmt.Sprintf("%s - Must Update On Different ResourceVersion", tc.Label), func(t *testing.T) {
		t.Parallel()
		tc.testUpdateDifferentVersion(t)
	})

	t.Run(fmt.Sprintf("%s - Must Skip On Same ResourceVersion", tc.Label), func(t *testing.T) {
		t.Parallel()
		tc.testUpdateSameVersion(t)
	})
}

func (tc *testCase) testGenerateSyncPayload(t *testing.T, expected []byte) {

	data := make([]interface{}, 0, len(tc.Objs))
	for _, obj := range tc.Objs {
		data = append(data, obj)
	}

	patches, err := generateSyncPayload(data, tc.ResourceType.Namespaced)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	result := expect.MustRoundTrip(t, patches)
	expect.MustEqual(t, result, expected)
}

func (tc *testCase) Play(t *testing.T, client dynamicClient, play expect.Script) *expect.Client {
	t.Helper()

	return expect.Play(t, play, func(ctx context.Context, mockClient *expect.Client) {
		data := mockClient.Prefix(tc.Prefix)
		sync := NewFromInterface(
			client,
			data,
			tc.ResourceType,
			WithBackoff(0, 5*time.Second, 0),
		)
		ignoreNamespace := []string{"opa"}
		sync.RunContext(ctx, ignoreNamespace)
	})
}

func (tc *testCase) testLoad(t *testing.T, expected []byte) {

	client := newFakeDynamicClient(t, tc.Objs...)
	play := expect.Script{
		expect.PutData("/", expected).End(),
	}

	data := tc.Play(t, client, play)
	expect.MustEqual(t, data.PrefixList, []string{tc.Prefix, tc.ResourceType.Resource})
}

func (tc *testCase) testAdd(t *testing.T) {

	client, obj := newFakeDynamicClient(t), tc.Objs[0]
	play := expect.Script{
		expect.PutData("/", []byte("{}")).Do(client.MustCreate(t, tc.ResourceType, obj)),
		expect.PutData(expect.MustKey(t, obj), expect.MustRoundTrip(t, obj)).End(),
	}

	tc.Play(t, client, play)
}

func (tc *testCase) testDelete(t *testing.T) {

	client, obj := newFakeDynamicClient(t, tc.Objs...), tc.Objs[0]
	play := expect.Script{
		expect.PutData("/").Do(client.MustRemove(t, tc.ResourceType, obj)),
		expect.PatchData(expect.MustKey(t, obj), "remove").End(),
	}

	tc.Play(t, client, play)
}

func (tc *testCase) testUpdate(t *testing.T) {

	change := mustUnstructure(t, tc.Objs[0])
	change.SetLabels(map[string]string{"test": "update"})
	change.SetResourceVersion("1")

	client := newFakeDynamicClient(t, tc.Objs...)
	play := expect.Script{
		expect.PutData("/").Do(client.MustUpdate(t, tc.ResourceType, change)),
		expect.PutData(expect.MustKey(t, change), expect.MustRoundTrip(t, change.Object)).End(),
	}

	tc.Play(t, client, play)
}

func (tc *testCase) testRetryLoad(t *testing.T, expected []byte) {

	client := newFakeDynamicClient(t, tc.Objs...)
	play := expect.Script{
		expect.PutData("/").DoError(errors.New("test fail update")),
		expect.PutData("/", expected).End(),
	}

	tc.Play(t, client, play)
}

func (tc *testCase) testRetryAdd(t *testing.T) {

	client, obj := newFakeDynamicClient(t), tc.Objs[0]
	play := expect.Script{
		expect.PutData("/").Do(client.MustCreate(t, tc.ResourceType, obj)),
		expect.PutData(expect.MustKey(t, obj)).DoError(errors.New("test fail update")),
		expect.PutData("/").End(),
	}

	tc.Play(t, client, play)
}

func (tc *testCase) testRetryUpdate(t *testing.T) {

	change := mustUnstructure(t, tc.Objs[0])
	change.SetLabels(map[string]string{"test": "update"})
	change.SetResourceVersion("1")

	client := newFakeDynamicClient(t, tc.Objs...)
	play := expect.Script{
		expect.PutData("/").Do(client.MustUpdate(t, tc.ResourceType, change)),
		expect.PutData(expect.MustKey(t, change)).DoError(errors.New("Failed to update")),
		expect.PutData("/").End(),
		// don't check the payload on this last put, because we
		// have removed an item so it no longer matches the tc.expected
	}

	tc.Play(t, client, play)
}

func (tc *testCase) testRetryDelete(t *testing.T) {

	client, obj := newFakeDynamicClient(t, tc.Objs...), tc.Objs[0]
	play := expect.Script{
		expect.PutData("/").Do(client.MustRemove(t, tc.ResourceType, obj)),
		expect.PatchData(expect.MustKey(t, obj), "remove").DoError(errors.New("test Patch failed")),
		expect.PutData("/").End(),
		// don't check the payload on this last put, because we
		// have removed an item so it no longer matches the tc.expected
	}

	tc.Play(t, client, play)
}

func (tc *testCase) testUpdateSameVersion(t *testing.T) {

	change := mustUnstructure(t, tc.Objs[0])
	change.SetAnnotations(map[string]string{"test": "update"})

	client := newFakeDynamicClient(t, tc.Objs...)
	play := expect.Script{
		expect.PutData("/").Do(client.MustUpdate(t, tc.ResourceType, change)),
		expect.Nothing(100 * time.Millisecond).End(),
	}

	tc.Play(t, client, play)
}

func (tc *testCase) testUpdateDifferentVersion(t *testing.T) {

	change := mustUnstructure(t, tc.Objs[0])
	change.SetLabels(map[string]string{"test": "update"})
	change.SetResourceVersion("1")

	client := newFakeDynamicClient(t, tc.Objs...)
	play := expect.Script{
		expect.PutData("/").Do(client.MustUpdate(t, tc.ResourceType, change)),
		expect.PutData(expect.MustKey(t, change), expect.MustRoundTrip(t, change)).End(),
	}

	tc.Play(t, client, play)
}

// MustCreate returns an action that creates an instance of the resource
func (f dynamicClient) MustCreate(t *testing.T, resourceType types.ResourceType, obj runtime.Object) expect.Action {
	namespace := mustAccess(t, obj).GetNamespace()
	return func() error {
		r := f.ResourceFor(resourceType, namespace)
		if _, err := r.Create(context.Background(), mustUnstructure(t, obj), metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create object %v: %v", obj, err)
		}
		return nil
	}
}

// MustRemove returns an Action that removes an instance of the resource
func (f dynamicClient) MustRemove(t *testing.T, resourceType types.ResourceType, obj runtime.Object) expect.Action {
	m := mustAccess(t, obj)
	return func() error {
		r := f.ResourceFor(resourceType, m.GetNamespace())
		if err := r.Delete(context.Background(), m.GetName(), metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Failed to remove object %v: %v", obj, err)
		}
		return nil
	}
}

// MustUpdate returns an Action that updates an instance of the resource
func (f dynamicClient) MustUpdate(t *testing.T, resourceType types.ResourceType, obj runtime.Object) expect.Action {
	namespace := mustAccess(t, obj).GetNamespace()
	return func() error {
		r := f.ResourceFor(resourceType, namespace)
		if _, err := r.Update(context.Background(), obj.(*unstructured.Unstructured), metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Failed to create object %v: %v", obj, err)
		}
		return nil
	}
}

// mustUnstructure clones the object provided into an Unstructured object
func mustUnstructure(t *testing.T, obj runtime.Object) *unstructured.Unstructured {
	copiedObj := expect.MustUnmarshal(t, expect.MustMarshal(t, obj))
	if asMap, ok := copiedObj.(map[string]interface{}); ok {
		return &unstructured.Unstructured{Object: asMap}
	}
	t.Fatalf("Failed to copy %#v as a map[string]interface{}", obj)
	return nil // to make staticcheck happy
}

// mustAccess returns an accessor for the given object
func mustAccess(t *testing.T, obj runtime.Object) metav1.Object {
	m, err := meta.Accessor(obj)
	if err != nil {
		t.Fatalf("Failed to build accessor for %v: %v", obj, err)
	}
	return m
}

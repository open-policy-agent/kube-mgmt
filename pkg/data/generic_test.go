package data

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/open-policy-agent/kube-mgmt/internal/mock"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"

	apiv1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
)

type testCase struct {
	label        string
	resourceType types.ResourceType
	prefix       string
	objs         []runtime.Object
	expected     string
}

func TestGenericSync(t *testing.T) {

	testCases := []testCase{
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
						Name:            "node1",
						ResourceVersion: "0",
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
						Name:            "node1",
						ResourceVersion: "0",
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
			expected: `{
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
						Name:            "pod1",
						Namespace:       "ns1",
						ResourceVersion: "0",
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
						Name:            "pod1",
						Namespace:       "ns1",
						ResourceVersion: "0",
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
			expected: `{
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

	sc := runtime.NewScheme()
	if err := scheme.AddToScheme(sc); err != nil {
		t.Fatalf("Failed to build initial scheme: %v", err)
	}

	for _, tc := range testCases {

		tc := tc // We will be running the tests in parallel, so avoid issues with loop var
		tc.expected = string(mock.MustMarshal(t, mock.MustUnmarshal(t, []byte(tc.expected))))

		t.Run(fmt.Sprintf("%s - GenerateSyncPayload", tc.label), func(t *testing.T) {
			t.Parallel()
			tc.testGenerateSyncPayload(t)
		})

		t.Run(fmt.Sprintf("%s - Run", tc.label), func(t *testing.T) {
			t.Parallel()
			tc.testRun(t, sc)
		})

		t.Run(fmt.Sprintf("%s - Add", tc.label), func(t *testing.T) {
			t.Parallel()
			tc.testAdd(t, sc)
		})

		t.Run(fmt.Sprintf("%s - Delete", tc.label), func(t *testing.T) {
			t.Parallel()
			tc.testDelete(t, sc)
		})

		t.Run(fmt.Sprintf("%s - Update", tc.label), func(t *testing.T) {
			t.Parallel()
			tc.testUpdate(t, sc)
		})

		t.Run(fmt.Sprintf("%s - Retry Run", tc.label), func(t *testing.T) {
			t.Parallel()
			tc.testRetryRun(t, sc)
		})

		t.Run(fmt.Sprintf("%s - Retry Add", tc.label), func(t *testing.T) {
			t.Parallel()
			tc.testRetryAdd(t, sc)
		})

		t.Run(fmt.Sprintf("%s - Retry Update", tc.label), func(t *testing.T) {
			t.Parallel()
			tc.testRetryUpdate(t, sc)
		})

		t.Run(fmt.Sprintf("%s - Retry Delete", tc.label), func(t *testing.T) {
			t.Parallel()
			tc.testRetryDelete(t, sc)
		})
	}
}

func (tc testCase) testGenerateSyncPayload(t *testing.T) {
	data := make([]interface{}, 0, len(tc.objs))
	for _, obj := range tc.objs {
		data = append(data, obj)
	}

	patches, err := generateSyncPayload(data, tc.resourceType.Namespaced)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	result := mock.MustString(t, patches)
	mock.MustEqual(t, result, tc.expected)
}

func (tc *testCase) play(t *testing.T, scheme *runtime.Scheme, client *fake.FakeDynamicClient, lastEvent mock.Event) *mock.Data {
	t.Helper()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	data := mock.Stage(t, lastEvent.Do(func() error {
		cancel()
		return nil
	}))
	sync := NewFromInterface(client, data.Prefix(tc.prefix), tc.resourceType, WithBackoff(0, 5*time.Second))

	sync.RunContext(ctx)
	if d, ok := ctx.Deadline(); ok && d.Before(time.Now()) {
		t.Fatal("Test canceled because of timeout")
	}
	return data
}

func (tc *testCase) testRun(t *testing.T, scheme *runtime.Scheme) {

	client := fake.NewSimpleDynamicClient(scheme, tc.objs...)
	play := mock.Script{}.
		Expect(mock.PutData("/", tc.expected))

	data := tc.play(t, scheme, client, play)
	mock.MustEqual(t, data.PrefixList, []string{tc.prefix, tc.resourceType.Resource})
}

func (tc *testCase) testAdd(t *testing.T, scheme *runtime.Scheme) {
	obj := tc.objs[0]

	client := fake.NewSimpleDynamicClient(scheme)
	play := mock.Script{}.
		Expect(mock.PutData("/", "{}")).
		Do(tc.mustCreate(t, client, obj)).
		Expect(mock.PutData(mustKey(t, obj), mock.MustString(t, obj)))

	tc.play(t, scheme, client, play)
}

func (tc *testCase) testDelete(t *testing.T, scheme *runtime.Scheme) {
	obj := tc.objs[0]

	client := fake.NewSimpleDynamicClient(scheme, tc.objs...)
	play := mock.Script{}.
		Expect(mock.PutData("/")).
		Do(tc.mustRemove(t, client, obj)).
		Expect(mock.PatchData(mustKey(t, obj), "remove"))

	tc.play(t, scheme, client, play)
}

func (tc *testCase) testUpdate(t *testing.T, scheme *runtime.Scheme) {
	change := mustUnstructure(t, tc.objs[0])
	change.SetLabels(map[string]string{"test": "update"})
	change.SetResourceVersion("1")

	client := fake.NewSimpleDynamicClient(scheme, tc.objs...)
	play := mock.Script{}.
		Expect(mock.PutData("/")).
		Do(tc.mustUpdate(t, client, change)).
		Expect(mock.PutData(mustKey(t, change), mock.MustString(t, change.Object)))

	tc.play(t, scheme, client, play)
}

func (tc *testCase) testRetryRun(t *testing.T, scheme *runtime.Scheme) {

	client := fake.NewSimpleDynamicClient(scheme, tc.objs...)
	play := mock.Script{}.
		Expect(mock.PutData("/")).
		Do(mock.MustError(errors.New("test fail update"))).
		Expect(mock.PutData("/", tc.expected))

	tc.play(t, scheme, client, play)
}

func (tc *testCase) testRetryAdd(t *testing.T, scheme *runtime.Scheme) {
	obj := tc.objs[0]

	client := fake.NewSimpleDynamicClient(scheme)
	play := mock.Script{}.
		Expect(mock.PutData("/")).
		Do(tc.mustCreate(t, client, obj)).
		Expect(mock.PutData(mustKey(t, obj))).
		Do(mock.MustError(errors.New("test fail update"))).
		Expect(mock.PutData("/"))

	tc.play(t, scheme, client, play)
}

func (tc *testCase) testRetryUpdate(t *testing.T, scheme *runtime.Scheme) {
	change := mustUnstructure(t, tc.objs[0])
	change.SetLabels(map[string]string{"test": "update"})
	change.SetResourceVersion("1")

	client := fake.NewSimpleDynamicClient(scheme, tc.objs...)
	play := mock.Script{}.
		Expect(mock.PutData("/")).
		Do(tc.mustUpdate(t, client, change)).
		Expect(mock.PutData(mustKey(t, change))).
		Do(mock.MustError(errors.New("Failed to update"))).
		Expect(mock.PutData("/"))
		// don't check the payload on this last put, because we
		// have removed an item so it no longer matches the tc.expected

	tc.play(t, scheme, client, play)
}

func (tc *testCase) testRetryDelete(t *testing.T, scheme *runtime.Scheme) {
	obj := tc.objs[0]

	client := fake.NewSimpleDynamicClient(scheme, tc.objs...)
	play := mock.Script{}.
		Expect(mock.PutData("/")).
		Do(tc.mustRemove(t, client, obj)).
		Expect(mock.PatchData(mustKey(t, obj), "remove")).
		Do(mock.MustError(errors.New("test Patch failed"))).
		Expect(mock.PutData("/"))
		// don't check the payload on this last put, because we
		// have removed an item so it no longer matches the tc.expected

	tc.play(t, scheme, client, play)
}

func (tc *testCase) mustGetResource(t *testing.T, client *fake.FakeDynamicClient, useNamespaceFrom runtime.Object) dynamic.ResourceInterface {
	t.Helper()

	nsr := client.Resource(mustGvr(tc.resourceType))
	if !tc.resourceType.Namespaced {
		return nsr
	}
	if useNamespaceFrom == nil {
		return nsr.Namespace(metav1.NamespaceAll)
	}
	accessor := meta.NewAccessor()
	ns, err := accessor.Namespace(useNamespaceFrom)
	if err != nil {
		t.Fatalf("Failed to get namespace from namespaced obj %v: %v", useNamespaceFrom, err)
	}
	return nsr.Namespace(ns)
}

func (tc *testCase) mustCreate(t *testing.T, client *fake.FakeDynamicClient, obj runtime.Object) mock.Action {
	return func() error {
		t.Helper()

		r := tc.mustGetResource(t, client, obj)
		if _, err := r.Create(context.Background(), mustUnstructure(t, tc.objs[0]), metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create object %v: %v", obj, err)
		}
		return nil
	}
}

func (tc *testCase) mustRemove(t *testing.T, client *fake.FakeDynamicClient, obj runtime.Object) mock.Action {
	return func() (zero error) {
		t.Helper()

		m, err := meta.Accessor(obj)
		if err != nil {
			t.Fatalf("Failed to build accessor for %v: %v", obj, err)
		}
		r := tc.mustGetResource(t, client, obj)
		if err := r.Delete(context.Background(), m.GetName(), metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Failed to remove object %v: %v", obj, err)
		}
		return nil
	}
}

func (tc *testCase) mustUpdate(t *testing.T, client *fake.FakeDynamicClient, obj runtime.Object) mock.Action {
	return func() error {
		t.Helper()

		r := tc.mustGetResource(t, client, obj)
		if _, err := r.Update(context.Background(), obj.(*unstructured.Unstructured), metav1.UpdateOptions{}); err != nil {
			t.Fatalf("Failed to create object %v: %v", obj, err)
		}
		return nil
	}
}

func mustKey(t *testing.T, obj runtime.Object) string {
	t.Helper()

	path, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		t.Fatalf("Failed to get path from object %v: %v", obj, err)
	}
	return path
}

func mustGvr(resourceType types.ResourceType) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    resourceType.Group,
		Version:  resourceType.Version,
		Resource: resourceType.Resource,
	}
}

func mustUnstructure(t *testing.T, obj runtime.Object) *unstructured.Unstructured {
	copiedObj := mock.MustUnmarshal(t, mock.MustMarshal(t, obj))
	if asMap, ok := copiedObj.(map[string]interface{}); ok {
		return &unstructured.Unstructured{Object: asMap}
	}
	t.Fatalf("Failed to copy %#v as a map[string]interface{}", obj)
	return nil // to make staticcheck happy
}

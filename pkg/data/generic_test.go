package data

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"github.com/open-policy-agent/kube-mgmt/pkg/types"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

func TestGeneratePatchesSingleClusterResource(t *testing.T) {
	resourceType := types.ResourceType{
		Namespaced: false,
		Resource:   "nodes",
		Version:    "v1",
	}
	prefix := ""
	objs := []unstructured.Unstructured{
		{
			mustJSONRoundTripObj(t, apiv1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
				Spec:       apiv1.NodeSpec{},
				Status:     apiv1.NodeStatus{},
			}),
		},
	}

	expected := `{
	    "node1":{
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
	}`

	verifyGenerateSyncPayload(t, resourceType, prefix, objs, expected)
}

func TestGeneratePatchesSingleClusterResourceWithPrefix(t *testing.T) {
	resourceType := types.ResourceType{
		Namespaced: false,
		Resource:   "nodes",
		Version:    "v1",
	}
	prefix := "kube"
	objs := []unstructured.Unstructured{
		{
			mustJSONRoundTripObj(t, apiv1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
				Spec:       apiv1.NodeSpec{},
				Status:     apiv1.NodeStatus{},
			}),
		},
	}

	expected := `{
	    "node1":{
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
	}`

	verifyGenerateSyncPayload(t, resourceType, prefix, objs, expected)
}

func TestGeneratePatchesMultipleClusterResourceWithPrefix(t *testing.T) {
	resourceType := types.ResourceType{
		Namespaced: false,
		Resource:   "nodes",
		Version:    "v1",
	}
	prefix := "kube"
	objs := []unstructured.Unstructured{
		{
			mustJSONRoundTripObj(t, apiv1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node1",
				},
				Spec:       apiv1.NodeSpec{},
				Status:     apiv1.NodeStatus{},
			}),
		},
		{
			mustJSONRoundTripObj(t, apiv1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node2",
				},
				Spec:       apiv1.NodeSpec{},
				Status:     apiv1.NodeStatus{},
			}),
		},
		{
			mustJSONRoundTripObj(t, apiv1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node3",
				},
				Spec:       apiv1.NodeSpec{},
				Status:     apiv1.NodeStatus{},
			}),
		},
	}

	expected := `{
	    "node1":{
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
	}`

	verifyGenerateSyncPayload(t, resourceType, prefix, objs, expected)
}

func TestGeneratePatchesSingleNamespacedResource(t *testing.T) {
	resourceType := types.ResourceType{
		Namespaced: true,
		Resource:   "pods",
		Version:    "v1",
	}
	prefix := ""
	objs := []unstructured.Unstructured{
		{
			mustJSONRoundTripObj(t, apiv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod1",
					Namespace: "ns1",
				},
				Spec:       apiv1.PodSpec{},
				Status:     apiv1.PodStatus{},
			}),
		},
	}

	expected := `{
	    "ns1":{
	        "pod1":{
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
	}`

	verifyGenerateSyncPayload(t, resourceType, prefix, objs, expected)
}

func TestGeneratePatchesSingleNamespacedResourceWithPrefix(t *testing.T) {
	resourceType := types.ResourceType{
		Namespaced: true,
		Resource:   "pods",
		Version:    "v1",
	}
	prefix := "kube"
	objs := []unstructured.Unstructured{
		{
			mustJSONRoundTripObj(t, apiv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod1",
					Namespace: "ns1",
				},
				Spec:       apiv1.PodSpec{},
				Status:     apiv1.PodStatus{},
			}),
		},
	}

	expected := `{
	    "ns1":{
	        "pod1":{
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
	}`

	verifyGenerateSyncPayload(t, resourceType, prefix, objs, expected)
}

func TestGeneratePatchesMultipleNamespacedResourceWithPrefix(t *testing.T) {
	resourceType := types.ResourceType{
		Namespaced: true,
		Resource:   "pods",
		Version:    "v1",
	}
	prefix := "kube"
	objs := []unstructured.Unstructured{
		{
			mustJSONRoundTripObj(t, apiv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod1",
					Namespace: "ns1",
				},
				Spec:       apiv1.PodSpec{},
				Status:     apiv1.PodStatus{},
			}),
		},
		{
			mustJSONRoundTripObj(t, apiv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod2",
					Namespace: "ns1",
				},
				Spec:       apiv1.PodSpec{},
				Status:     apiv1.PodStatus{},
			}),
		},
		{
			mustJSONRoundTripObj(t, apiv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pod1",
					Namespace: "ns2",
				},
				Spec:       apiv1.PodSpec{},
				Status:     apiv1.PodStatus{},
			}),
		},
	}

	expected := `{
	    "ns1":{
	        "pod1":{
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
	        "pod1":{
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
	}`

	verifyGenerateSyncPayload(t, resourceType, prefix, objs, expected)
}

func verifyGenerateSyncPayload(t *testing.T, resourceType types.ResourceType, prefix string, objs []unstructured.Unstructured, expected string) {
	t.Helper()
	s := New(&rest.Config{}, opa.New("http://localhost:8181/", "").Prefix(prefix), resourceType)
	patches, err := s.generateSyncPayload(objs)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	actual := mustMarshalJSON(t, patches)
	formattedExpected := mustJSONRoundTripString(t, []byte(expected))
	if string(actual) != string(formattedExpected) {
		t.Errorf("Expected:\n\n%s\n\nActual:\n\n%s\n", formattedExpected, actual)
	}
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

func mustJSONRoundTripString(t *testing.T, str []byte) []byte {
	t.Helper()

	var obj map[string]interface{}
	mustUnmarshalJSON(t, str, &obj)
	return mustMarshalJSON(t, obj)
}

func mustJSONRoundTripObj(t *testing.T, obj interface{}) map[string]interface{} {
	t.Helper()

	bs := mustMarshalJSON(t, obj)

	out := map[string]interface{}{}
	mustUnmarshalJSON(t, bs, &out)
	
	return out
}

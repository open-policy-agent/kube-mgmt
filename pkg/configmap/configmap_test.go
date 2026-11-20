package configmap

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

const (
	policyLabel = "openpolicyagent.org/policy"
	dataLabel   = "openpolicyagent.org/data"
)

// recordingOPA is an opa.Client that records the policies and data pushed to it.
type recordingOPA struct {
	mu       sync.Mutex
	policies []string
	data     []string
}

func (r *recordingOPA) InsertPolicy(id string, _ []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policies = append(r.policies, id)
	return nil
}

func (r *recordingOPA) PutData(path string, _ interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = append(r.data, path)
	return nil
}

func (r *recordingOPA) pushed() (policies, data []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Sorted(slices.Values(r.policies)), slices.Sorted(slices.Values(r.data))
}

func (r *recordingOPA) DeletePolicy(string) error                             { return nil }
func (r *recordingOPA) Prefix(string) opa.Data                                { return r }
func (r *recordingOPA) PatchData(string, string, *interface{}) error          { return nil }
func (r *recordingOPA) PostData(string, interface{}) (json.RawMessage, error) { return nil, nil }

func configMap(namespace, name string, labels, data map[string]string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, Labels: labels},
		Data:       data,
	}
}

// newTestSync returns a Sync for the "default" namespace whose informer cache
// holds the given ConfigMaps, as if Run had listed them.
func newTestSync(t *testing.T, cms ...*v1.ConfigMap) (*Sync, *recordingOPA, cache.Store) {
	t.Helper()
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	objs := make([]runtime.Object, 0, len(cms))
	for _, cm := range cms {
		if err := store.Add(cm); err != nil {
			t.Fatalf("Failed to add %v to store: %v", cm.Name, err)
		}
		objs = append(objs, cm)
	}
	client := &recordingOPA{}
	s := &Sync{
		opa:       client,
		clientset: fake.NewClientset(objs...),
		matcher:   DefaultConfigMapMatcher([]string{"default"}, true, true, policyLabel, "rego", dataLabel, "opa"),
		stores:    []cache.Store{store},
	}
	return s, client, store
}

func TestResync(t *testing.T) {
	s, client, _ := newTestSync(t,
		configMap("default", "policies", map[string]string{policyLabel: "rego"},
			map[string]string{"a.rego": "package a", "b.rego": "package b"}),
		loaded(configMap("default", "data", map[string]string{dataLabel: "opa"},
			map[string]string{"d.json": `{"k": "v"}`})),
		configMap("default", "unlabeled", nil,
			map[string]string{"x.rego": "package x"}),
		configMap("other", "policies", map[string]string{policyLabel: "rego"},
			map[string]string{"o.rego": "package o"}),
	)

	s.Resync()

	policies, data := client.pushed()
	if want := []string{"default/policies/a.rego", "default/policies/b.rego"}; !slices.Equal(policies, want) {
		t.Errorf("Pushed policies %v, want %v", policies, want)
	}
	if want := []string{"default/data/d.json"}; !slices.Equal(data, want) {
		t.Errorf("Pushed data %v, want %v", data, want)
	}
	cm, err := s.clientset.CoreV1().ConfigMaps("default").Get(context.Background(), "policies", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap: %v", err)
	}
	if got, want := cm.Annotations[statusAnnotationKey], `{"status":"ok"}`; got != want {
		t.Errorf("Status annotation is %q, want %q", got, want)
	}

	// "data" already had the annotations, so it must not be patched again.
	var patched []string
	for _, action := range s.clientset.(*fake.Clientset).Actions() {
		if patch, ok := action.(k8stesting.PatchAction); ok {
			patched = append(patched, patch.GetNamespace()+"/"+patch.GetName())
		}
	}
	if want := []string{"default/policies"}; !slices.Equal(patched, want) {
		t.Errorf("Patched ConfigMaps %v, want %v", patched, want)
	}
}

// loaded returns cm with the annotations of a successful load.
func loaded(cm *v1.ConfigMap) *v1.ConfigMap {
	cm.Annotations = map[string]string{
		statusAnnotationKey:  `{"status":"ok"}`,
		retriesAnnotationKey: "0",
	}
	return cm
}

// deletingStore simulates ConfigMaps that are deleted while Resync runs: the
// informer removes them from its cache right after Resync has listed it.
type deletingStore struct {
	cache.Store
	deleted []*v1.ConfigMap
}

func (d deletingStore) ListKeys() []string {
	defer d.deleteAll()
	return d.Store.ListKeys()
}

func (d deletingStore) List() []interface{} {
	defer d.deleteAll()
	return d.Store.List()
}

func (d deletingStore) deleteAll() {
	for _, cm := range d.deleted {
		_ = d.Store.Delete(cm)
	}
}

func TestResyncSkipsConfigMapsDeletedDuringResync(t *testing.T) {
	kept := configMap("default", "kept", map[string]string{policyLabel: "rego"}, map[string]string{"k.rego": "package k"})
	gone := configMap("default", "gone", map[string]string{policyLabel: "rego"}, map[string]string{"g.rego": "package g"})
	s, client, store := newTestSync(t, kept, gone)
	s.stores = []cache.Store{deletingStore{Store: store, deleted: []*v1.ConfigMap{gone}}}

	s.Resync()

	// Pushing "gone" would leave a deleted policy enforced in OPA, because its
	// delete event has already been handled.
	policies, _ := client.pushed()
	if want := []string{"default/kept/k.rego"}; !slices.Equal(policies, want) {
		t.Errorf("Pushed policies %v, want %v", policies, want)
	}
}

func TestResyncWaitsForEventHandlers(t *testing.T) {
	s, client, _ := newTestSync(t,
		configMap("default", "policies", map[string]string{policyLabel: "rego"}, map[string]string{"a.rego": "package a"}))

	// Hold the lock like an event handler that is still running.
	s.mu.RLock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Resync()
	}()

	time.Sleep(50 * time.Millisecond)
	if policies, _ := client.pushed(); len(policies) != 0 {
		t.Fatalf("Resync pushed %v while an event handler was running", policies)
	}
	s.mu.RUnlock()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Resync did not finish after the event handler did")
	}
	if policies, _ := client.pushed(); len(policies) != 1 {
		t.Fatalf("Pushed policies %v, want exactly one", policies)
	}
}

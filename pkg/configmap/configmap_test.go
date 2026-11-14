package configmap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/kube-mgmt/pkg/opa"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeOPA struct {
	mu       sync.Mutex
	fail     bool
	inserted int
	// deps maps a policy id to a policy id it needs to compile.
	deps   map[string]string
	loaded map[string]bool
}

func (f *fakeOPA) InsertPolicy(id string, _ []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inserted++
	if f.fail {
		return errors.New("compile error")
	}
	if dep, ok := f.deps[id]; ok && !f.loaded[dep] {
		return fmt.Errorf("compile error: %v needs %v", id, dep)
	}
	if f.loaded == nil {
		f.loaded = map[string]bool{}
	}
	f.loaded[id] = true
	return nil
}

func (f *fakeOPA) DeletePolicy(string) error                             { return nil }
func (f *fakeOPA) Prefix(string) opa.Data                                { return f }
func (f *fakeOPA) PatchData(string, string, *interface{}) error          { return nil }
func (f *fakeOPA) PutData(string, interface{}) error                     { return nil }
func (f *fakeOPA) PostData(string, interface{}) (json.RawMessage, error) { return nil, nil }

func (f *fakeOPA) setFail(fail bool) {
	f.mu.Lock()
	f.fail = fail
	f.mu.Unlock()
}

func (f *fakeOPA) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inserted
}

// newTestSync returns a Sync whose retries are queued instead of run on a timer.
func newTestSync(t *testing.T, cm *v1.ConfigMap, o *fakeOPA) (*Sync, *[]func()) {
	t.Helper()
	queued := &[]func(){}
	s := &Sync{
		opa:       o,
		clientset: fake.NewClientset(cm),
		matcher:   DefaultConfigMapMatcher([]string{"*"}, true, true, "openpolicyagent.org/policy", "rego", "openpolicyagent.org/data", "opa"),
		retries:   map[string]retryState{},
		retryAfter: func(_ time.Duration, f func()) {
			*queued = append(*queued, f)
		},
	}
	return s, queued
}

func runQueued(q *[]func()) {
	fns := *q
	*q = nil
	for _, f := range fns {
		f()
	}
}

func policyCM() *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "opa",
			Name:            "rule",
			ResourceVersion: "1",
			Labels:          map[string]string{"openpolicyagent.org/policy": "rego"},
		},
		Data: map[string]string{"rule.rego": "package rule"},
	}
}

func annotations(t *testing.T, s *Sync) map[string]string {
	t.Helper()
	cm, err := s.clientset.CoreV1().ConfigMaps("opa").Get(context.TODO(), "rule", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return cm.Annotations
}

func TestRetriesAreLimited(t *testing.T) {
	o := &fakeOPA{fail: true}
	s, q := newTestSync(t, policyCM(), o)

	s.add(policyCM())
	for i := 0; i < 10 && len(*q) > 0; i++ {
		runQueued(q)
	}

	if got, want := o.count(), defaultRetries+1; got != want {
		t.Fatalf("expected %d load attempts, got %d", want, got)
	}
	if got := annotations(t, s)[retriesAnnotationKey]; got != "0" {
		t.Fatalf("expected retries annotation 0, got %q", got)
	}
}

// Another instance resetting the shared retries annotation to 0 must not stop
// this instance from retrying (issue #194).
func TestRetryIgnoresOtherInstanceAnnotations(t *testing.T) {
	o := &fakeOPA{fail: true}
	s, q := newTestSync(t, policyCM(), o)

	s.add(policyCM())

	// Another instance succeeds and writes status ok / retries 0.
	oldCm := policyCM()
	newCm := policyCM()
	newCm.ResourceVersion = "2"
	newCm.Annotations = map[string]string{retriesAnnotationKey: "0", statusAnnotationKey: `{"status":"ok"}`}
	s.update(oldCm, newCm)
	if got := o.count(); got != 1 {
		t.Fatalf("annotation-only update should not trigger a sync, got %d attempts", got)
	}

	o.setFail(false)
	runQueued(q)

	if got := o.count(); got != 2 {
		t.Fatalf("expected the pending retry to run, got %d attempts", got)
	}
	if len(*q) != 0 {
		t.Fatalf("expected no further retries after success")
	}
	if got := annotations(t, s)[statusAnnotationKey]; got != `{"status":"ok"}` {
		t.Fatalf("expected ok status, got %q", got)
	}
}

func TestContentChangeResetsRetries(t *testing.T) {
	o := &fakeOPA{fail: true}
	s, q := newTestSync(t, policyCM(), o)

	s.add(policyCM())
	for i := 0; i < 10 && len(*q) > 0; i++ {
		runQueued(q)
	}
	attempts := o.count()

	newCm := policyCM()
	newCm.ResourceVersion = "5"
	newCm.Data["rule.rego"] = "package rule2"
	if _, err := s.clientset.CoreV1().ConfigMaps("opa").Update(context.TODO(), newCm, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	s.update(policyCM(), newCm)
	if len(*q) != 1 {
		t.Fatalf("expected a retry to be scheduled for new content")
	}
	if got := o.count(); got != attempts+1 {
		t.Fatalf("expected one new attempt, got %d", got-attempts)
	}
}

// Policies that depend on each other in a chain longer than the retry budget
// must all load once their dependencies do.
func TestDependencyChainLoads(t *testing.T) {
	const n = 6
	o := &fakeOPA{deps: map[string]string{}}
	cms := make([]*v1.ConfigMap, n)
	for i := range cms {
		cm := policyCM()
		cm.Name = fmt.Sprintf("chain-%d", i)
		cms[i] = cm
		if i < n-1 {
			o.deps[fmt.Sprintf("opa/chain-%d/rule.rego", i)] = fmt.Sprintf("opa/chain-%d/rule.rego", i+1)
		}
	}
	s, q := newTestSync(t, cms[0], o)
	for _, cm := range cms[1:] {
		if _, err := s.clientset.CoreV1().ConfigMaps("opa").Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	// Informers list ConfigMaps by name, so dependents are added first.
	for _, cm := range cms {
		s.add(cm)
	}
	for i := 0; i < 100 && len(*q) > 0; i++ {
		runQueued(q)
	}

	for i := range cms {
		if id := fmt.Sprintf("opa/chain-%d/rule.rego", i); !o.loaded[id] {
			t.Errorf("policy %v was not loaded", id)
		}
	}
	if len(s.retries) != 0 {
		t.Errorf("expected no pending retries, got %v", len(s.retries))
	}
}

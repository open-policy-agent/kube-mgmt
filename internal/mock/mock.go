package mock

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
)

type request string

const (
	patchRequest request = "PatchData"
	putRequest   request = "PutData"
)

// Request represents an operation against the mock client.
type Request struct {
	req      request
	path     string
	op       string
	expected []string
}

// PutData describes a PutData request.
func PutData(path string, expected ...string) Request {
	return Request{
		req:      putRequest,
		path:     path,
		expected: expected,
	}
}

// PatchData describes a PatchData request
func PatchData(path string, op string, expected ...string) Request {
	return Request{
		req:      patchRequest,
		path:     path,
		op:       op,
		expected: expected,
	}
}

// Data emulates OPA Data Client API
type Data struct {
	PrefixList []string
	// This function will be called by PutData and PatchData
	actor func(req Request, value interface{}) error
}

var _ opa_client.Data = &Data{}

type step struct {
	Request
	action Action
}

// Script is a sequence of steps to perform on requests to mock.Data.
type Script []step

// Event is an accessor for a step in a script
type Event struct {
	s Script
	r Request
}

// Expect to receive a particular request next
func (s Script) Expect(req Request) Event {
	return Event{
		s: s,
		r: req,
	}
}

// Action performed when an expected Event arrives.
// The result of the action is passed through to the
//the caller of mock.Data.
type Action func() error

// Do arranges for the action to be called when the
// event hapens.
func (e Event) Do(action Action) Script {
	return append(e.s, step{Request: e.r, action: action})
}

// Prefix implements Data
func (f *Data) Prefix(path string) opa_client.Data {
	f.PrefixList = append(f.PrefixList, path)
	return f
}

// PatchData implements Data.
func (f *Data) PatchData(path string, op string, value *interface{}) (err error) {
	req := Request{
		req:  patchRequest,
		path: path,
		op:   op,
	}
	var actualValue interface{}
	if value != nil {
		actualValue = *value
	}
	return f.actor(req, actualValue)
}

// PutData implements Data
func (f *Data) PutData(path string, value interface{}) (err error) {
	req := Request{
		req:  putRequest,
		path: path,
	}
	return f.actor(req, value)
}

var errNotSupported = errors.New("PostData not supported")

// PostData implements Data. Currently not supported.
func (*Data) PostData(string, interface{}) (json.RawMessage, error) {
	return nil, errNotSupported
}

// Stage returns a Data mock instructed to play the provided Script
func Stage(t *testing.T, play Script) *Data {
	cursor := 0
	return &Data{
		actor: func(req Request, value interface{}) error {
			if cursor > len(play) {
				t.Fatalf("Expected at most %d steps, got one more request", len(play))
			}
			cue := play[cursor]
			if cue.req != req.req {
				t.Fatalf("Expected request %s at step %d, got %s", cue.req, cursor, req.req)
			}
			if req.path != cue.path {
				t.Logf("Expected path %s at step %d, got %s", cue.path, cursor, req.path)
				t.Fail()
			}
			if req.op == string(patchRequest) && req.op != cue.op {
				t.Logf("Expected operation %s at step %d, got %s", cue.op, cursor, req.op)
				t.Fail()
			}
			if cue.expected != nil && len(cue.expected) > 0 {
				MustEqual(t, MustString(t, value), strings.Join(cue.expected, ""))
			}
			cursor++
			if cue.action != nil {
				return cue.action()
			}
			return nil
		},
	}
}

// MustMarshal marshals the objet to JSON, calls t.Fatal on error
func MustMarshal(t *testing.T, obj interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("error marshalling JSON: %s", err)
	}
	return data
}

// MustUmnarshal unmarshals the objet from JSON, calls t.Fatal on error
func MustUnmarshal(t *testing.T, data []byte) interface{} {
	t.Helper()

	var result interface{}
	if len(data) > 0 {
		err := json.Unmarshal(data, &result)
		if err != nil {
			t.Fatalf("error unmarshalling JSON: %s", err)
		}
	}
	return result
}

// mustString marshals the object to JSON consistently.
//
// Kubernetes objects have custom marshallers that output the
// json in a custom order. So comparing the marshalled representation of
// a kubernetes object with that of a map or an *unstructured.Unstructured
// built from that same object will fail.
//
// MustString will make sure the generated string is comparable.
func MustString(t *testing.T, obj interface{}) string {
	return string(MustMarshal(t, MustUnmarshal(t, MustMarshal(t, obj))))
}

// MustEqual compares the values and calls t.Fatal on error
func MustEqual(t *testing.T, result, expected interface{}) {
	t.Helper()

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected:\n\n%q\n\nActual:\n\n%q\n", expected, result)
	}
}

// MustError builds an action that returns the given error
func MustError(err error) Action {
	return func() error {
		return err
	}
}

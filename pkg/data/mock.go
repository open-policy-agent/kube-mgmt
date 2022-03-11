package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
	"k8s.io/client-go/dynamic/fake"
)

// mockData emulates OPA Data Client API
type mockData struct {
	PrefixList []string
	// This function will be called by PutData and PatchData
	actor func(op operation, path string, value interface{}) error
}

type operation string

const (
	operationPut   operation = "PutData"
	operationPatch operation = "PatchData"
)

type step struct {
	operation    operation
	expectedPath string
	expectedData map[string]interface{}
	action       func(client *fake.FakeDynamicClient)
	err          error
}

// script is a sequence of steps to perform when data in mockData changes.
type script []step

// OnPatch defines the action that will be triggered or the error that
// will be returned when the client gets a PatchData request next.
func (s script) OnPatch(expectedPath string, do func(client *fake.FakeDynamicClient), err error) script {
	return append(s, step{
		operation:    operationPatch,
		expectedPath: expectedPath,
		expectedData: nil,
		action:       do,
		err:          err,
	})
}

// OnPut defines the action that will be triggered or the error that
// will be returned when the client gets a PutData request next.
func (s script) OnPut(expectedPath string, expectedData map[string]interface{}, do func(client *fake.FakeDynamicClient), err error) script {
	return append(s, step{
		operation:    operationPut,
		expectedPath: expectedPath,
		expectedData: expectedData,
		action:       do,
		err:          err,
	})
}

// Prefix implements Data
func (f *mockData) Prefix(path string) opa_client.Data {
	f.PrefixList = append(f.PrefixList, path)
	return f
}

// PatchData implements Data. Currently not supported.
func (f *mockData) PatchData(path string, op string, value *interface{}) (err error) {
	if op != "remove" {
		return fmt.Errorf("unsupported operation %s", op)
	}
	return f.actor(operationPatch, path, nil)
}

// PutData implements Data
func (f *mockData) PutData(path string, value interface{}) (err error) {
	return f.actor(operationPut, path, value)
}

var errNotSupported = errors.New("PostData not supported")

// PostData implements Data. Currently not supported.
func (*mockData) PostData(string, interface{}) (json.RawMessage, error) {
	return nil, errNotSupported
}

// Run the script against a given
func (m *mockData) Play(t *testing.T, client *fake.FakeDynamicClient, sync *GenericSync, play script) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))

	cursor := 0
	m.actor = func(op operation, path string, value interface{}) error {
		cue := play[cursor]
		cursor++
		if cue.operation != op {
			t.Fatalf("Expected operation %s at step %d, got %s", cue.operation, cursor, op)
		}
		if path != cue.expectedPath {
			t.Logf("Expected path %s, got %s", cue.expectedPath, path)
			t.Fail()
		}
		if cue.expectedData != nil {
			data := mustJSONRoundTrip(t, value)
			mustEqual(t, data, cue.expectedData)
		}
		if cue.action != nil {
			cue.action(client)
		}
		if cursor >= len(play) {
			cancel()
		}
		return cue.err
	}
	sync.RunContext(ctx)

	if cursor < len(play) {
		t.Fatalf("Expected %d operations, got %d", len(play), cursor)
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

func mustJSONRoundTrip(t *testing.T, obj interface{}) map[string]interface{} {
	t.Helper()

	bs := mustMarshalJSON(t, obj)
	var out map[string]interface{}
	mustUnmarshalJSON(t, bs, &out)

	return out
}

func mustEqual(t *testing.T, result, expected interface{}) {
	t.Helper()

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Sync payload expected:\n\n%q\n\nActual:\n\n%q\n", expected, result)
		t.Fail()
	}
}

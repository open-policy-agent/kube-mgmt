package expect

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// Script is a sequence of expected Requests for a Client,
// and the Actions to perform on each Request.
type Script []Step

// Play creates a client with the script provided, and runs the show.
// When the script ends, the show is cancelled and the final state
// of the client returned.
func Play(t *testing.T, script Script, show func(ctx context.Context, client *Client)) *Client {
	steps := len(script)
	if steps <= 0 || show == nil {
		return nil
	}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	// Arrange to cancel the context on the last step
	last := script[steps-1]
	script[steps-1].Action = func() error {
		defer cancel()
		if last.Action == nil {
			return nil
		}
		return last.Action()
	}

	var (
		actor     func(req Request, value interface{}) error
		improvise func(cursor int)
		cursor    int = 0
	)

	actor = func(req Request, value interface{}) error {
		if cursor >= len(script) {
			t.Fatalf("Expected at most %d steps, got one more request %v", len(script), req)
		}
		// Save the actual value received to req. We do it
		// here because we have the *testing.T instance, and
		// can call t.Fatal if conversions fail.
		if req.req == insertPolicyRequest {
			req.value = value.([]byte)
		} else {
			req.value = MustRoundTrip(t, value)
		}
		// Check that the request matches the cue
		cue := script[cursor]
		if !cue.Equals(req) {
			seq := script[:cursor+1].strings("\n\t")
			t.Fatalf("Expected sequence:\n\t%v\nError at step %d, got:\n\t%v", seq, cursor, req)
		}
		cursor++
		if cursor < len(script) && script[cursor].req == noRequest {
			// If the next update is timed, schedule it.
			go improvise(cursor)
		}
		if cue.Action == nil {
			return nil
		}
		return cue.Action()
	}

	// improvise triggers the step without any external input
	improvise = func(cursor int) {
		<-time.After(script[cursor].interval)
		actor(script[cursor].Request, nil)
	}

	client := &Client{actor: actor}
	if script[0].req == noRequest {
		// boot the script if the first step is a wait.
		go improvise(0)
	}

	show(ctx, client)
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(time.Now()) {
		t.Fatalf("Test failed because of timeout")
	}
	return client
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

// mustRoundtrip marshals the object to JSON consistently.
//
// Kubernetes objects have custom marshallers that output the
// json in a custom order. So comparing the marshalled representation of
// a kubernetes object with that of a map or an *unstructured.Unstructured
// built from that same object will fail.
//
// MustRoundTrip will make sure the generated string is comparable.
func MustRoundTrip(t *testing.T, obj interface{}) []byte {
	return MustMarshal(t, MustUnmarshal(t, MustMarshal(t, obj)))
}

// MustEqual compares the values and calls t.Fatal on error
func MustEqual(t *testing.T, result, expected interface{}) {
	t.Helper()

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected:\n\n%q\n\nActual:\n\n%q\n", expected, result)
	}
}

// MustKey gets the mentaNamespaceKey of an object
func MustKey(t *testing.T, obj runtime.Object) string {
	t.Helper()

	path, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		t.Fatalf("Failed to get path from object %v: %v", obj, err)
	}
	return path
}

// strings formats the Script as a list of strings for printing
func (s Script) strings(sep string) string {
	steps := make([]string, 0, len(s))
	for cursor, step := range s {
		steps = append(steps, fmt.Sprintf("%d: %s", cursor, step.String()))
	}
	return strings.Join(steps, sep)
}

package expect

import (
	"fmt"
	"reflect"
	"time"
)

type request string

const (
	patchRequest        request = "PatchData"
	putRequest          request = "PutData"
	insertPolicyRequest request = "InsertPolicy"
	deletePolicyRequest request = "DeletePolicy"
	noRequest           request = "Nothing"
)

// Request represents an operation against the mock client.
type Request struct {
	req      request
	path     string
	op       string
	value    []byte
	interval time.Duration // Only applies to script.Expect(Nothing())
}

// Equals compares two requests
func (expected Request) Equals(actual Request) bool {
	return expected.req == actual.req &&
		expected.path == actual.path &&
		(expected.req != patchRequest || expected.op == actual.op) &&
		(expected.value == nil || reflect.DeepEqual(expected.value, actual.value))
}

// String implements fmt.Stringer
func (r Request) String() string {
	if r.value != nil {
		return fmt.Sprintf("{req: %q, path: %q, op: %q, value: %q}", r.req, r.path, r.op, string(r.value))
	}
	return fmt.Sprintf("{req: %q, path: %q, op: %q}", r.req, r.path, r.op)
}

func optional(expected ...[]byte) []byte {
	if len(expected) > 0 {
		return expected[0]
	}
	return nil
}

// PutData describes a PutData request with an optional expected value
// (expected value can be omitted)
func PutData(path string, expected ...[]byte) Request {
	return Request{
		req:   putRequest,
		path:  path,
		value: optional(expected...),
	}
}

// PatchData describes a PatchData request with an optional expected value
// (expected value can be omitted)
func PatchData(path string, op string, expected ...[]byte) Request {
	return Request{
		req:   patchRequest,
		path:  path,
		op:    op,
		value: optional(expected...),
	}
}

// InsertPolicy describes a InsertPolicy request with an optional expected value
// (expected value can be omitted)
func InsertPolicy(path string, expected ...[]byte) Request {
	return Request{
		req:   insertPolicyRequest,
		path:  path,
		value: optional(expected...),
	}
}

// DeletePolicy describes a DeletePolicy request with an optional expected value
// (expected value can be omitted)
func DeletePolicy(path string) Request {
	return Request{
		req:  deletePolicyRequest,
		path: path,
	}
}

// Nothing describes an empty action. The client must not get any request for the given time
func Nothing(duration time.Duration) Request {
	return Request{
		req:      noRequest,
		interval: duration,
	}
}

// Action performed when an expected Request arrives.
// The request will return the result of invoking the Action.
type Action func() error

// Step combines a Request and an Action
type Step struct {
	Request
	Action
}

// Do turns a Request into a Step
func (req Request) Do(action Action) Step {
	return Step{
		Request: req,
		Action:  action,
	}
}

// DoError is a shortcut for Do(func() error { return err })
func (req Request) DoError(err error) Step {
	return Step{
		Request: req,
		Action: func() error {
			return err
		},
	}
}

// End is a shortcut for Do(nil)
func (req Request) End() Step {
	return Step{
		Request: req,
	}
}

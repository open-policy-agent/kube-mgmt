// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHTTPClientMakePatch(t *testing.T) {

	tests := []struct {
		prefix string
		path   string
		op     string
		value  string
		want   string
	}{
		{
			prefix: "",
			path:   "foo",
			op:     "add",
			value:  "true",
			want: `[{
				"path": "/foo",
				"op": "add",
				"value": true
			}]`,
		},
		{
			prefix: "",
			path:   "default/foo",
			op:     "remove",
			value:  "",
			want: `[{
				"path": "/default/foo",
				"op": "remove"
			}]`,
		},
		{
			prefix: "type",
			path:   "default/foo",
			op:     "remove",
			value:  "",
			want: `[{
				"path": "/type/default/foo",
				"op": "remove"
			}]`,
		},
		{
			prefix: "/type1/subtypeA/",
			path:   "default/foo",
			op:     "remove",
			value:  "",
			want: `[{
				"path": "/type1/subtypeA/default/foo",
				"op": "remove"
			}]`,
		},
	}

	for _, tc := range tests {

		client := &httpClient{"URL", tc.prefix, ""}
		var value *interface{}

		if tc.value != "" {
			var x interface{}
			if err := json.Unmarshal([]byte(tc.value), &x); err != nil {
				panic(err)
			}
			value = &x
		}

		patch := mustMakePatch(client, tc.path, tc.op, value)

		var expected interface{}
		if err := json.Unmarshal([]byte(tc.want), &expected); err != nil {
			panic(err)
		}

		if !reflect.DeepEqual(patch, expected) {
			t.Errorf("Expected %v but got: %v", expected, patch)
		}
	}

}

func mustMakePatch(client *httpClient, path, op string, value *interface{}) interface{} {

	buf, err := client.makePatch(path, op, value)
	if err != nil {
		panic(err)
	}

	return mustUnmarshalJSON(buf)
}

func mustUnmarshalJSON(r io.Reader) interface{} {
	var x interface{}
	err := json.NewDecoder(r).Decode(&x)
	if err != nil {
		panic(err)
	}
	return x
}

func TestInstanceID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path != "/v1/config":
			http.NotFound(w, r)
		case r.Header.Get("Authorization") != "Bearer secret":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			fmt.Fprint(w, `{"result":{"labels":{"id":"abc","version":"1.19.0"}}}`)
		}
	}))
	defer srv.Close()

	// A trailing slash in --opa-url must not change the request path.
	for _, url := range []string{srv.URL + "/v1", srv.URL + "/v1/"} {
		id, err := InstanceID(context.Background(), url, "secret")
		if err != nil || id != "abc" {
			t.Fatalf("InstanceID(%q) = %q, %v; want \"abc\", nil", url, id, err)
		}
	}
}

func TestInstanceIDErrors(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name:    "rejected by OPA",
			handler: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) },
			wantErr: "401",
		},
		{
			name:    "no instance ID",
			handler: func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"result":{}}`) },
			wantErr: "no labels.id",
		},
		{
			name:    "invalid JSON",
			handler: func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `not json`) },
			wantErr: "decode",
		},
		{
			name:    "OPA hangs",
			handler: func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() },
			wantErr: "deadline exceeded",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err := InstanceID(ctx, srv.URL+"/v1", "")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got error %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestWatchRestarts(t *testing.T) {
	type check struct {
		id  string
		err error
	}
	down := check{err: errors.New("connection refused")}

	cases := []struct {
		name    string
		checks  []check
		resyncs int
	}{
		{"same instance", []check{{id: "a"}, {id: "a"}, {id: "a"}}, 0},
		{"restart between checks", []check{{id: "a"}, {id: "b"}, {id: "b"}}, 1},
		{"restart seen while down", []check{{id: "a"}, down, down, {id: "b"}}, 1},
		{"outage without restart", []check{{id: "a"}, down, {id: "a"}}, 0},
		{"unreachable at startup", []check{down, {id: "a"}, {id: "a"}}, 1},
		{"never reachable", []check{down, down}, 0},
		{"repeated restarts", []check{{id: "a"}, {id: "b"}, {id: "c"}}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			calls := 0
			instanceID := func(context.Context) (string, error) {
				if calls == len(tc.checks) {
					cancel()
					return "", ctx.Err()
				}
				c := tc.checks[calls]
				calls++
				return c.id, c.err
			}
			resyncs := 0
			watchRestarts(ctx, time.Millisecond, instanceID, func() { resyncs++ })

			if calls != len(tc.checks) {
				t.Fatalf("got %d checks, want %d", calls, len(tc.checks))
			}
			if resyncs != tc.resyncs {
				t.Fatalf("got %d resyncs, want %d", resyncs, tc.resyncs)
			}
		})
	}
}

func TestWatchRestartsFirstCheckIsImmediate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var deadline time.Time
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchRestarts(ctx, time.Hour, func(checkCtx context.Context) (string, error) {
			deadline, _ = checkCtx.Deadline()
			cancel()
			return "a", nil
		}, func() {})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("first check did not run before the first interval elapsed")
	}
	if d := time.Until(deadline); deadline.IsZero() || d > maxRestartCheckTimeout {
		t.Fatalf("check deadline is %v away, want at most %v", d, maxRestartCheckTimeout)
	}
}

func TestWatchRestartsDisabled(t *testing.T) {
	watchRestarts(context.Background(), 0, func(context.Context) (string, error) {
		t.Fatal("unexpected check with interval 0")
		return "", nil
	}, func() {})
}

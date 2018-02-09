// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Error contains the standard error fields returned by OPA.
type Error struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Errors  json.RawMessage `json:"errors,omitempty"`
}

func (err *Error) Error() string {
	return fmt.Sprintf("code %v: %v", err.Code, err.Message)
}

// Undefined represents an undefined response from OPA.
type Undefined struct{}

func (Undefined) Error() string {
	return fmt.Sprintf("undefined")
}

// IsUndefinedErr returns true if the err represents an undefined result from
// OPA.
func IsUndefinedErr(err error) bool {
	_, ok := err.(Undefined)
	return ok
}

// Client defines the OPA client interface.
type Client interface {
	Policies
	Data
}

// Policies defines the policy management interface in OPA.
type Policies interface {
	InsertPolicy(id string, bs []byte) error
	DeletePolicy(id string) error
}

// Data defines the interface for pushing and querying data in OPA.
type Data interface {
	Prefix(path string) Data
	PatchData(path string, op string, value *interface{}) error
	PutData(path string, value interface{}) error
	PostData(path string, value interface{}) (json.RawMessage, error)
}

// New returns a new Client object.
func New(url string) Client {
	return &httpClient{strings.TrimRight(url, "/"), ""}
}

type httpClient struct {
	url    string
	prefix string
}

func (c *httpClient) Prefix(path string) Data {
	cpy := *c
	cpy.prefix = joinPaths("/", c.prefix, path)
	return &cpy
}

func (c *httpClient) PatchData(path string, op string, value *interface{}) error {
	buf, err := c.makePatch(path, op, value)
	if err != nil {
		return err
	}
	resp, err := c.do("PATCH", slashPath("data"), buf)
	if err != nil {
		return err
	}
	return c.handleErrors(resp)
}

func (c *httpClient) PutData(path string, value interface{}) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(value); err != nil {
		return err
	}
	absPath := slashPath("data", c.prefix, path)
	resp, err := c.do("PUT", absPath, &buf)
	if err != nil {
		return err
	}
	return c.handleErrors(resp)
}

func (c *httpClient) PostData(path string, value interface{}) (json.RawMessage, error) {
	var buf bytes.Buffer
	var input struct {
		Input interface{} `json:"input"`
	}
	input.Input = value
	if err := json.NewEncoder(&buf).Encode(input); err != nil {
		return nil, err
	}
	absPath := slashPath("data", c.prefix, path)
	resp, err := c.do("POST", absPath, &buf)
	if err != nil {
		return nil, err
	}
	var result struct {
		Result json.RawMessage        `json:"result"`
		Error  map[string]interface{} `json:"error"`
	}
	if resp.StatusCode != 200 {
		return nil, c.handleErrors(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Result == nil {
		return nil, Undefined{}
	}
	return result.Result, nil
}

func (c *httpClient) InsertPolicy(id string, bs []byte) error {
	buf := bytes.NewBuffer(bs)
	path := slashPath("policies", id)
	resp, err := c.do("PUT", path, buf)
	if err != nil {
		return err
	}
	return c.handleErrors(resp)
}

func (c *httpClient) DeletePolicy(id string) error {
	path := slashPath("policies", id)
	resp, err := c.do("DELETE", path, nil)
	if err != nil {
		return err
	}
	return c.handleErrors(resp)
}

func (c *httpClient) makePatch(path, op string, value *interface{}) (io.Reader, error) {
	patch := []struct {
		Path  string       `json:"path"`
		Op    string       `json:"op"`
		Value *interface{} `json:"value,omitempty"`
	}{
		{
			Path:  slashPath(c.prefix, path),
			Op:    op,
			Value: value,
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(patch); err != nil {
		return nil, err
	}
	return &buf, nil
}

func (c *httpClient) handleErrors(resp *http.Response) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var err Error
	if err := json.NewDecoder(resp.Body).Decode(&err); err != nil {
		return err
	}
	return &err
}

func (c *httpClient) do(verb, path string, body io.Reader) (*http.Response, error) {
	url := c.url + path
	req, err := http.NewRequest(verb, url, body)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func slashPath(paths ...string) string {
	return makePath("/", paths...)
}

func makePath(join string, paths ...string) string {
	return join + joinPaths(join, paths...)
}

func joinPaths(join string, paths ...string) string {
	parts := []string{}
	for _, path := range paths {
		path = strings.Trim(path, join)
		if path != "" {
			parts = append(parts, path)
		}
	}
	return strings.Join(parts, join)
}

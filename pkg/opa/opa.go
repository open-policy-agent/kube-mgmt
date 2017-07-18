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
	PatchData(path string, op string, value interface{}) error
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
	prefix := strings.Trim(path, "/")
	if cpy.prefix != "" {
		prefix = cpy.prefix + "/" + prefix
	}
	cpy.prefix = prefix
	return &cpy
}

func (c *httpClient) PatchData(path string, op string, value interface{}) error {
	var prefix = "/"
	if c.prefix != "" {
		prefix = "/" + c.prefix
	}
	patch := []struct {
		Path  string      `json:"path"`
		Op    string      `json:"op"`
		Value interface{} `json:"value"`
	}{
		{
			Path:  prefix + strings.Trim(path, "/"),
			Op:    op,
			Value: value,
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(patch); err != nil {
		return err
	}
	resp, err := c.do("PUT", "/data", &buf)
	if err != nil {
		return err
	}
	return c.handleErrors(resp)
}

func (c *httpClient) PutData(path string, value interface{}) error {
	var prefix = "/"
	if c.prefix != "" {
		prefix = "/" + c.prefix
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(value); err != nil {
		return err
	}
	resp, err := c.do("PUT", "/data"+prefix+"/"+strings.Trim(path, "/"), &buf)
	if err != nil {
		return err
	}
	return c.handleErrors(resp)
}

func (c *httpClient) PostData(path string, value interface{}) (json.RawMessage, error) {
	var prefix = "/"
	if c.prefix != "" {
		prefix = "/" + c.prefix
	}
	var buf bytes.Buffer
	var input struct {
		Input interface{} `json:"input"`
	}
	input.Input = value
	if err := json.NewEncoder(&buf).Encode(input); err != nil {
		return nil, err
	}
	resp, err := c.do("POST", "/data"+prefix+"/"+strings.Trim(path, "/"), &buf)
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
	id = strings.Trim(id, "/")
	resp, err := c.do("PUT", "/policies/"+id, buf)
	if err != nil {
		return err
	}
	return c.handleErrors(resp)
}

func (c *httpClient) DeletePolicy(id string) error {
	id = strings.Trim(id, "/")
	resp, err := c.do("DELETE", "/policies/"+id, nil)
	if err != nil {
		return err
	}
	return c.handleErrors(resp)
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

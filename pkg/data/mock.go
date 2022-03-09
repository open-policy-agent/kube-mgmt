package data

import (
	"encoding/json"

	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
)

// mockData emulates OPA Data Client API
type mockData struct {
	prefix   []string
	data     interface{}
	onUpdate func() // called after each PutData
}

// Prefix implements Data
func (f *mockData) Prefix(path string) opa_client.Data {
	f.prefix = append(f.prefix, path)
	return f
}

// PatchData implements Data. Currently not supported.
func (*mockData) PatchData(string, string, *interface{}) error {
	return nil
}

// PutData implements Data
func (f *mockData) PutData(path string, value interface{}) error {
	if f.onUpdate != nil {
		defer f.onUpdate()
	}
	f.data = value
	return nil
}

// PostData implements Data. Currently not supported.
func (*mockData) PostData(string, interface{}) (json.RawMessage, error) {
	return nil, nil
}

package expect

import (
	"encoding/json"
	"errors"

	opa_client "github.com/open-policy-agent/kube-mgmt/pkg/opa"
)

// Client emulates OPA Client API
type Client struct {
	PrefixList []string
	// This function will be called on every request
	actor func(req Request, value interface{}) error
}

// Prefix implements Data
func (f *Client) Prefix(path string) opa_client.Data {
	f.PrefixList = append(f.PrefixList, path)
	return f
}

// PatchData implements Data.
func (f *Client) PatchData(path string, op string, value *interface{}) (err error) {
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
func (f *Client) PutData(path string, value interface{}) (err error) {
	req := Request{
		req:  putRequest,
		path: path,
	}
	return f.actor(req, value)
}

var errNotSupported = errors.New("PostData not supported")

// PostData implements Data. Currently not supported.
func (*Client) PostData(string, interface{}) (json.RawMessage, error) {
	return nil, errNotSupported
}

// InsertPolicy implements Policies
func (f *Client) InsertPolicy(path string, value []byte) (err error) {
	req := Request{
		req:  insertPolicyRequest,
		path: path,
	}
	return f.actor(req, value)
}

// DeletePolicy implements Policies
func (f *Client) DeletePolicy(path string) (err error) {
	req := Request{
		req:  deletePolicyRequest,
		path: path,
	}
	return f.actor(req, nil)
}

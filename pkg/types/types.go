// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package types contains type information used by controllers.
package types

import "strings"

// ResourceType describes a resource type in Kubernetes.
type ResourceType struct {
	// Namespaced indicates if this kind is namespaced.
	Namespaced bool
	Resource   string
	Group      string
	Version    string
}

func (t ResourceType) String() string {
	parts := []string{}
	if t.Group != "" {
		parts = append(parts, t.Group)
	}
	if t.Version != "" {
		parts = append(parts, t.Version)
	}
	if t.Resource != "" {
		parts = append(parts, t.Resource)
	}
	return strings.Join(parts, "/")
}

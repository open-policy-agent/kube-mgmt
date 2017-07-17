// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package types contains type information used by controllers.
package types

// ResourceType describes a resource type in Kubernetes.
type ResourceType struct {
	// Namespaced indicates if this kind is namespaced.
	Namespaced bool
	Resource   string
	Group      string
	Version    string
}

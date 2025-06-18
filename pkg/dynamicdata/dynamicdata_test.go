package dynamicdata

import (
	"context"
	"fmt"
	"testing"

	sdktest "github.com/open-policy-agent/opa/sdk/test"
	"github.com/open-policy-agent/opa/v1/logging"
)

func TestAnalyzer(t *testing.T) {

	ctx := context.Background()

	s := sdktest.MustNewServer(sdktest.RawBundles(true), sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
		".manifest": `{"roots": ["main"]}`,
		"main/main.rego": `package main
		import rego.v1
		main if { data.kubernetes.resources.pods[ns][_].metadata.labels.badlabel == "badbadbad"; r2 }
		r2 if { data.kubernetes.resources.namespaces["default"].metadata.labels.foo == "bar" }`,
	}))

	defer s.Stop()

	config := fmt.Appendf(nil, `{
		services: {
			test: {
				url: "%v/bundles"
			}
		},
		bundles: {
			test: {
				service: test,
				resource: bundle.tar.gz
			}
		}
	}`, s.URL())

	a, err := newAnalyzer(ctx, config, "kubernetes/resources", "main/main", logging.New())
	if err != nil {
		t.Fatal(err)
	}

	result := <-a.C
	if len(result.Refs) != 2 || result.Refs[0].Resource != "namespaces" || result.Refs[1].Resource != "pods" {
		t.Fatalf("expected to identify pods reference but got: %v", result)
	}

	if err := a.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestAnalyzerNoDeps(t *testing.T) {

	ctx := context.Background()

	s := sdktest.MustNewServer(sdktest.RawBundles(true), sdktest.MockBundle("/bundles/bundle.tar.gz", map[string]string{
		".manifest": `{"roots": ["main"]}`,
		"main/main.rego": `package main
		import rego.v1
		main if { true }`,
	}))

	defer s.Stop()

	config := fmt.Appendf(nil, `{
		services: {
			test: {
				url: "%v/bundles"
			}
		},
		bundles: {
			test: {
				service: test,
				resource: bundle.tar.gz
			}
		}
	}`, s.URL())

	a, err := newAnalyzer(ctx, config, "kubernetes/resources", "main/main", logging.New())
	if err != nil {
		t.Fatal(err)
	}

	result := <-a.C
	if len(result.Refs) != 0 {
		t.Fatalf("expected not to identify any resources but got: %v", result)
	}

	if err := a.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

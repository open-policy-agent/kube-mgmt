package main

import (
	"reflect"
	"testing"
)

func TestFlagParsing(t *testing.T) {
	var f gvkFlag

	badPaths := []string{
		"foo/bar/",
		"foo",
	}

	for _, tc := range badPaths {
		if err := f.Set(tc); err == nil {
			t.Fatalf("Expected error from %v", tc)
		}
	}

	expected := gvkFlag{
		{"example.org", "foo", "bar"},
	}

	if err := f.Set("example.org/Foo/bar"); err != nil || !reflect.DeepEqual(expected, f) {
		t.Fatalf("Expected %v but got: %v (err: %v)", expected, f, err)
	}

	expected = append(expected, groupVersionKind{"example.org", "bar", "baz"})

	if err := f.Set("example.org/Bar/baz"); err != nil || !reflect.DeepEqual(expected, f) {
		t.Fatalf("Expected %v but got: %v (err: %v)", expected, f, err)
	}

	expected = append(expected, groupVersionKind{"", "v2", "corge"})

	if err := f.Set("v2/corge"); err != nil || !reflect.DeepEqual(expected, f) {
		t.Fatalf("Expected %v but got: %v (err: %v)", expected, f, err)
	}

}

func TestFlagString(t *testing.T) {

	var f gvkFlag
	expected := "[example.org/foo/bar]"

	if err := f.Set("example.org/foo/bar"); err != nil || f.String() != expected {
		t.Fatalf("Exepcted %v but got: %v (err: %v)", expected, f.String(), err)
	}
}

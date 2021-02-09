package main

import (
	"testing"
)

func BenchmarkParentDir(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parentDir("/a/b/c/d//e")
	}
}

func TestParentDir(t *testing.T) {
	initLoggers()
	input := []string{
		"/a/b",
		"/a/b/../c",
		"a",
		"/",
	}

	expected := []string{
		"/a",
		"/a",
		".",
		"/",
	}

	for i := 0 ; i < len(input) ; i++ {
		if parentDir(input[i]) != expected[i] {
			t.Error(input[i], expected[i])
		}
	}
}

func TestChrooted(t *testing.T) {
	if chrooted("/ok", "/") != false {
		t.Error()
	}
	if chrooted("/ok", "/ok/foo/bar") != true {
		t.Error()
	}
	if chrooted("/", "/whatever") != true {
		t.Error()
	}
	if chrooted("/ok", "/ok") != true {
		t.Error()
	}
	if chrooted("/foo", "/bar") != false {
		t.Error()
	}
}


func TestWalk(t *testing.T) {
	testCases := []struct {
		description string
		hasFlags []string
		count int
	}{
		{
			description: "No JMES filtering",
			hasFlags: []string{},
			count: 12,
		},
		{
			description: "key foodict is present",
			hasFlags: []string{"foodict"},
			count: 1,
		},
		{
			description: "env_type is clientvm",
			hasFlags: []string{"env_type == 'ocp-clientvm'"},
			count: 2,
		},
		{
			description: "Is a Babylon catalog item",
			hasFlags: []string{"__meta__.catalog"},
			count: 5,
		},
		{
			description: "env_type is clientvm and purpose is development",
			hasFlags: []string{
				"env_type == 'ocp-clientvm'",
				"purpose == 'development'",
			},
			count: 1,
		},
	}

	for _, tc := range testCases {
		result, err := findCatalogItems(".", tc.hasFlags)
		if err != nil {
			t.Error()
		}
		if len(result) != tc.count {
			t.Error(tc.description, len(result), tc.count)
		}
	}
}

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
	testCases := []struct {
		root string
		path string
		result bool
	}{
		{
			root: "/ok",
			path: "/",
			result: false,
		},
		{
			root: "ok",
			path: "/",
			result: false,
		},
		{
			root: "foo/bar",
			path: "foo/bar",
			result: true,
		},
		{
			root: "foo/bar",
			path: "foo/bar/a",
			result: true,
		},
		{
			root: "/ok",
			path: "/ok/foo/bar",
			result: true,
		},
		{
			root: "/",
			path: "/whatever",
			result: true,
		},
		{
			root: "/ok",
			path: "/ok",
			result: true,
		},
		{
			root: "/foo",
			path: "/bar",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/c/a.yaml",
			result: true,
		},
		{
			root: "/a/b/c",
			path: "/a/b/a.yaml",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/cc/a.yaml",
			result: false,
		},
	}

	for _, tc := range testCases {
		if tc.result != chrooted(tc.root, tc.path) {
			t.Error(tc.root, tc.path, tc.result)
		}
	}
}
func TestResolvePath(t *testing.T) {
	testCases := []struct {
		root string
		path string
		contextFile string
		result string
		description string
		expectedErr error
	}{
		{
			root: "/a/b/c",
			path: "/d.yaml",
			contextFile: "whatever",
			result: "/a/b/c/d.yaml",
			description: "include absolute path in AgnosticV repo",
			expectedErr: nil,
		},
		{
			root: "/a/b/c",
			path: "/d/e/f.yaml",
			contextFile: "whatever",
			result: "/a/b/c/d/e/f.yaml",
			description: "include absolute path in AgnosticV repo",
			expectedErr: nil,
		},
		{
			root: "/a/b/c",
			path: "foo.yaml",
			contextFile: "/a/b/c/d/bar.yaml",
			result: "/a/b/c/d/foo.yaml",
			description: "include relative path in AgnosticV repo",
			expectedErr: nil,
		},
		{
			root: "/a/b/c",
			path: "../bar.yaml",
			contextFile: "/a/b/c/d/foo.yaml",
			result: "/a/b/c/bar.yaml",
			description: "include relative path, with '..', in AgnosticV repo",
			expectedErr: nil,
		},
		{
			root: "/a/b/c",
			path: "../../../../bar.yaml",
			contextFile: "/a/b/c/d/foo.yaml",
			result: "",
			description: "include relative path, with many '..', in AgnosticV repo",
			expectedErr: ErrorIncludeOutOfChroot,
		},
	}

	for _, tc := range testCases {
		result, err := resolvePath(tc.root, tc.path, tc.contextFile)

		if err != tc.expectedErr {
			t.Error(tc.description, "with", tc.root, tc.path, tc.contextFile, ":", err, "!=", tc.expectedErr)
		}

		if tc.result != result {
			t.Error(tc.description, "with", tc.root, tc.path, tc.contextFile, ":", result, "!=", tc.result)
		}
	}
}

func TestIsPathCatalogItem(t *testing.T) {
	testCases := []struct {
		root string
		path string
		result bool
	}{
		{
			root: "/a/b/c",
			path: "/a/b/c/a.yaml",
			result: true,
		},
		{
			root: "/a/b/c",
			path: "/a/b/a.yaml",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/c/.dotdir/a.yaml",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/c/.dotfile.yaml",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/c/notyaml",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/cc/a.yaml",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/c/d/e/f/a.yaml",
			result: true,
		},
		{
			root: "/a/b/c",
			path: "/a/b/c/common.yaml",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/c/includes/e/f/a.yaml",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/c/d/includes/f/a.yaml",
			result: false,
		},
		{
			root: "/a/b/c",
			path: "/a/b/c/d/e/includes/a.yaml",
			result: false,
		},
	}

	for _, tc := range testCases {
		if tc.result != isPathCatalogItem(tc.root, tc.path) {
			t.Error(tc.root, tc.path, tc.result)
		}
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
			count: 11,
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

func TestParseInclude(t *testing.T) {
	testCases := []struct {
		line string
		found bool
		path string
	}{
		{
			line: "#include /path/ok",
			found: true,
			path: "/path/ok",

		},
		{
			line: "#include    /path/ok",
			found: true,
			path: "/path/ok",

		},
		{
			line: "#include \"/path/ok\"",
			found: true,
			path: "/path/ok",

		},
		{
			line: "#include \"/path/ok\"    ",
			found: true,
			path: "/path/ok",

		},
		{
			line: "  #include \"/path/ok\"    ",
			found: true,
			path: "/path/ok",

		},
		{
			line: "#iclude \"/path/ok\"    ",
			found: false,
			path: "",

		},
		{
			line: "",
			found: false,
			path: "",

		},
		{
			line: "#include \"/path  with space \" ",
			found: true,
			path: "/path  with space ",
		},
		{
			line: "#include /path  with space without quotes ",
			found: false,
			path: "",
		},
	}

	for _, tc := range testCases {
		found, include := parseInclude(tc.line)
		if found != tc.found || include.path != tc.path {
			t.Error(tc, found, include)
		}
	}
}


func TestInclude(t *testing.T) {

	merged, _, err := mergeVars("fixtures/gpte/OCP_CLIENTVM/dev.yaml", "v2")
	if err != nil {
		t.Error(err)
	}

	if val, ok := merged["from_include"]; ok {
		if val != "notcatalogitem" {
			t.Error("value 'from_include' is not 'notcatalogitem'")
		}
	} else {
		t.Error("value 'from_include' not found in merge")
	}

	if _, ok := merged["from_include1"]; !ok {
		t.Error("value 'from_include1' not found")
	}

	merged, _, err = mergeVars("fixtures/gpte/OCP_CLIENTVM/.testloop.yaml", "v2")

	if err != ErrorIncludeLoop {
		t.Error("ErrorIncludeLoop expected, got", err)
	}


}

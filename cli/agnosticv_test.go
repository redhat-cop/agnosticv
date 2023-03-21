package main

import (
	"io"
	"log"
	"os"
	"strings"
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

	for i := 0; i < len(input); i++ {
		if parentDir(input[i]) != expected[i] {
			t.Error(input[i], expected[i])
		}
	}
}

func TestPathContains(t *testing.T) {
	testCases := []struct {
		root   string
		path   string
		result bool
	}{
		{
			root:   "/ok",
			path:   "/",
			result: false,
		},
		{
			root:   "ok",
			path:   "/",
			result: false,
		},
		{
			root:   "foo/bar",
			path:   "foo/bar",
			result: true,
		},
		{
			root:   "foo/bar",
			path:   "foo/bar/a",
			result: true,
		},
		{
			root:   "/ok",
			path:   "/ok/foo/bar",
			result: true,
		},
		{
			root:   "/",
			path:   "/whatever",
			result: true,
		},
		{
			root:   "/ok",
			path:   "/ok",
			result: true,
		},
		{
			root:   "/foo",
			path:   "/bar",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/c/a.yaml",
			result: true,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/a.yaml",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/cc/a.yaml",
			result: false,
		},
	}

	for _, tc := range testCases {
		if tc.result != isRoot(tc.root, tc.path) {
			t.Error(tc.root, tc.path, tc.result)
		}
	}
}
func TestResolvePath(t *testing.T) {
	testCases := []struct {
		root        string
		path        string
		contextFile string
		result      string
		description string
		expectedErr error
	}{
		{
			root:        "/a/b/c",
			path:        "/d.yaml",
			contextFile: "whatever",
			result:      "/a/b/c/d.yaml",
			description: "include absolute path in AgnosticV repo",
			expectedErr: nil,
		},
		{
			root:        "/a/b/c",
			path:        "/d/e/f.yaml",
			contextFile: "whatever",
			result:      "/a/b/c/d/e/f.yaml",
			description: "include absolute path in AgnosticV repo",
			expectedErr: nil,
		},
		{
			root:        "/a/b/c",
			path:        "foo.yaml",
			contextFile: "/a/b/c/d/bar.yaml",
			result:      "/a/b/c/d/foo.yaml",
			description: "include relative path in AgnosticV repo",
			expectedErr: nil,
		},
		{
			root:        "/a/b/c",
			path:        "../bar.yaml",
			contextFile: "/a/b/c/d/foo.yaml",
			result:      "/a/b/c/bar.yaml",
			description: "include relative path, with '..', in AgnosticV repo",
			expectedErr: nil,
		},
		{
			root:        "/a/b/c",
			path:        "../../../../bar.yaml",
			contextFile: "/a/b/c/d/foo.yaml",
			result:      "",
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
		root   string
		path   string
		result bool
	}{
		{
			root:   "/a/b/c",
			path:   "/a/b/c/a.yaml",
			result: true,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/a.yaml",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/c/.dotdir/a.yaml",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/c/.dotfile.yaml",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/c/notyaml",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/cc/a.yaml",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/c/d/e/f/a.yaml",
			result: true,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/c/common.yaml",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/c/includes/e/f/a.yaml",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/c/d/includes/f/a.yaml",
			result: false,
		},
		{
			root:   "/a/b/c",
			path:   "/a/b/c/d/e/includes/a.yaml",
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
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	testCases := []struct {
		description    string
		hasFlags       []string
		relatedFlags   []string
		orRelatedFlags []string
		count          int
	}{
		{
			description: "No JMES filtering",
			hasFlags:    []string{},
			count:       11,
		},
		{
			description:  "Related includes/include1.yaml",
			hasFlags:     []string{},
			relatedFlags: []string{"fixtures/includes/include1.yaml"},
			count:        2,
		},
		{
			description:  "Related to fixtures/test/BABYLON_EMPTY_CONFIG_AWS/common.yaml",
			hasFlags:     []string{},
			relatedFlags: []string{"fixtures/test/BABYLON_EMPTY_CONFIG_AWS/common.yaml"},
			count:        3,
		},
		{
			description: "Related to fixtures/test/BABYLON_EMPTY_CONFIG_AWS/common.yaml and test.yaml",
			hasFlags:    []string{},
			relatedFlags: []string{
				"fixtures/test/BABYLON_EMPTY_CONFIG_AWS/common.yaml",
				"fixtures/test/BABYLON_EMPTY_CONFIG_AWS/test.yaml",
			},
			count: 1,
		},
		{
			description: "Related to fixtures/gpte/OCP_CLIENTVM/description.adoc",
			hasFlags:    []string{},
			relatedFlags: []string{
				"fixtures/gpte/OCP_CLIENTVM/description.adoc",
			},
			count: 2,
		},
		{
			description:    "Related (inclusive, --or-related) to /common.yaml",
			hasFlags:       []string{},
			relatedFlags:   []string{"fixtures/includes/include1.yaml"},
			orRelatedFlags: []string{"fixtures/common.yaml"},
			count:          11,
		},
		{
			description:    "Related (exclusive + inclusive) to /common.yaml and --has flag",
			hasFlags:       []string{"foodict"},
			relatedFlags:   []string{"fixtures/includes/include1.yaml"},
			orRelatedFlags: []string{"fixtures/common.yaml"},
			count:          1,
		},
		{
			description: "key foodict is present",
			hasFlags:    []string{"foodict"},
			count:       1,
		},
		{
			description: "env_type is clientvm",
			hasFlags:    []string{"env_type == 'ocp-clientvm'"},
			count:       2,
		},
		{
			description: "Is a Babylon catalog item",
			hasFlags:    []string{"__meta__.catalog"},
			count:       7,
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
		result, err := findCatalogItems(".", tc.hasFlags, tc.relatedFlags, tc.orRelatedFlags)
		if err != nil {
			t.Error()
		}
		if len(result) != tc.count {
			t.Error(tc.description, len(result), tc.count)
			t.Error(result, tc)
		}
	}
}

func TestParseInclude(t *testing.T) {
	testCases := []struct {
		line  string
		found bool
		path  string
	}{
		{
			line:  "#include /path/ok",
			found: true,
			path:  "/path/ok",
		},
		{
			line:  "#include    /path/ok",
			found: true,
			path:  "/path/ok",
		},
		{
			line:  "#include \"/path/ok\"",
			found: true,
			path:  "/path/ok",
		},
		{
			line:  "#include \"/path/ok\"    ",
			found: true,
			path:  "/path/ok",
		},
		{
			line:  "  #include \"/path/ok\"    ",
			found: true,
			path:  "/path/ok",
		},
		{
			line:  "#iclude \"/path/ok\"    ",
			found: false,
			path:  "",
		},
		{
			line:  "",
			found: false,
			path:  "",
		},
		{
			line:  "#include \"/path  with space \" ",
			found: true,
			path:  "/path  with space ",
		},
		{
			line:  "#include /path  with space without quotes ",
			found: false,
			path:  "",
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

	merged, _, err := mergeVars("fixtures/gpte/OCP_CLIENTVM/dev.yaml", mergeStrategies)
	if err != nil {
		t.Fatal(err)
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

	logErr = log.New(io.Discard, "!!! ", log.LstdFlags)
	_, _, err = mergeVars("fixtures/gpte/OCP_CLIENTVM/.testloop.yaml", mergeStrategies)

	if err != ErrorIncludeLoop {
		t.Error("ErrorIncludeLoop expected, got", err)
	}
}

func TestSchemaValidationPatternFailed(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	validateFlag = true
	initSchemaList()

	path := "fixtures/test/BABYLON_EMPTY_CONFIG/dev.yaml"
	merged, _, err := mergeVars(path, mergeStrategies)
	if err != nil {
		t.Error("Error not expected")
	}
	errValidation := validateAgainstSchemas(path, merged)

	if errValidation == nil {
		t.Error("Error expected")
	} else {
		if !strings.Contains(
			errValidation.Error(),
			"Error at \"/__meta__/lifespan/default\": string doesn't match the regular expression") {
			t.Error("ErrorSchema not found", errValidation)
		}
	}
}

func TestSchemaValidationOK(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	validateFlag = true
	initSchemaList()

	path := "fixtures/test/BABYLON_EMPTY_CONFIG/prod.yaml"
	merged, _, errMerge := mergeVars(path, mergeStrategies)
	if errMerge != nil {
		t.Error("Error not expected")
	}
	err := validateAgainstSchemas(path, merged)

	if err != nil {
		t.Error("Error", err)
	}
}

func TestGetMergeList(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	validateFlag = true
	initSchemaList()

	l, err := getMergeList(abs("fixtures/test/BABYLON_EMPTY_CONFIG/dev.yaml"))
	if err != nil {
		t.Fatal("getMergeList failed")
	}

	if len(l) != 5 {
		t.Log(l)
		t.Error("merge list is wrong")
	}
}

func TestGetMetaPath(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	validateFlag = true
	initSchemaList()

	testCases := []struct {
		path        string
		meta        string
		expectedErr error
	}{
		{
			path:        "/ok/dev.yaml",
			meta:        "/ok/dev.meta.yaml",
			expectedErr: nil,
		},
		{
			path:        "/ok/dev.yml",
			meta:        "/ok/dev.meta.yml",
			expectedErr: nil,
		},
		{
			path:        "dev.yaml",
			meta:        "dev.meta.yaml",
			expectedErr: nil,
		},
		{
			path:        "dev.yaml",
			meta:        "dev.meta.yaml",
			expectedErr: nil,
		},
		{
			path:        "",
			meta:        "",
			expectedErr: ErrorEmptyPath,
		},
	}

	for _, tc := range testCases {
		result, err := getMetaPath(tc.path)

		if err != tc.expectedErr {
			t.Error("with", tc.path, tc.meta, ":", err, "!=", tc.expectedErr)
		}

		if tc.meta != result {
			t.Error("with", tc.path, ":", result, "!=", tc.meta)
		}
	}

}

func TestIsMetaPath(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	validateFlag = true
	initSchemaList()

	testCases := []struct {
		path   string
		result bool
	}{
		{
			path:   "/ok/dev.yaml",
			result: false,
		},
		{
			path:   "/ok/dev.meta.yaml",
			result: true,
		},
		{
			path:   "dev.meta.yml",
			result: true,
		},
		{
			path:   ".yml",
			result: false,
		},
		{
			path:   "",
			result: false,
		},
	}

	for _, tc := range testCases {
		result := isMetaPath(tc.path)

		if result != tc.result {
			t.Error("with", tc.path, ":", result, "!=", tc.result)
		}
	}
}

func TestWrongMetaFile(t *testing.T) {
	rootFlag = abs("incorrect-fixtures")
	logErr = log.New(io.Discard, "!!! ", log.LstdFlags)

	_, _, err := mergeVars("incorrect-fixtures/test/TEST_WRONG_META_FILE/dev.yaml", mergeStrategies)

	if err != ErrorIncorrectMeta {
		t.Error("ErrorIncorrectMeta expected, got", err)
	}
}

func TestFindRoot(t *testing.T) {
	wd, _ := os.Getwd()

	wd = abs(wd)
	parent := parentDir(wd)

	testCases := []struct {
		path   string
		result string
	}{
		{
			path:   "fixtures",
			result: parent,
		},
	}

	for _, tc := range testCases {
		result := findRoot(tc.path)

		if result != tc.result {
			t.Error("with", tc.path, ":", result, "!=", tc.result)
		}
	}
}

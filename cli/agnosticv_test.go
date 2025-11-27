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
	initLoggers()
	prevDir, _ := os.Getwd()
	// Restore the current directory at the end of the function
	defer func() {
		if err := os.Chdir(prevDir); err != nil {
			logErr.Printf("%v\n", err)
		}
	}()

	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initMergeStrategies()
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
			count:       20,
		},
		{
			description:  "Related includes/include1.yaml",
			hasFlags:     []string{},
			relatedFlags: []string{"includes/include1.yaml"},
			count:        2,
		},
		{
			description:  "Related to test/BABYLON_EMPTY_CONFIG_AWS/common.yaml",
			hasFlags:     []string{},
			relatedFlags: []string{"test/BABYLON_EMPTY_CONFIG_AWS/common.yaml"},
			count:        4,
		},
		{
			description: "Related to test/BABYLON_EMPTY_CONFIG_AWS/common.yaml and test.yaml",
			hasFlags:    []string{},
			relatedFlags: []string{
				"test/BABYLON_EMPTY_CONFIG_AWS/common.yaml",
				"test/BABYLON_EMPTY_CONFIG_AWS/test.yaml",
			},
			count: 1,
		},
		{
			description: "Related to gpte/OCP_CLIENTVM/description.adoc",
			hasFlags:    []string{},
			relatedFlags: []string{
				"gpte/OCP_CLIENTVM/description.adoc",
			},
			count: 2,
		},
		{
			description:    "Related (inclusive, --or-related) to /common.yaml",
			hasFlags:       []string{},
			relatedFlags:   []string{"includes/include1.yaml"},
			orRelatedFlags: []string{"common.yaml"},
			count:          20,
		},
		{
			description:    "Related (exclusive + inclusive) to /common.yaml and --has flag",
			hasFlags:       []string{"foodict"},
			relatedFlags:   []string{"includes/include1.yaml"},
			orRelatedFlags: []string{"common.yaml"},
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
			count:       20,
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
		result, err := findCatalogItems(rootFlag, tc.hasFlags, tc.relatedFlags, tc.orRelatedFlags)
		if err != nil {
			t.Error(err)
		}
		if len(result) != tc.count {
			t.Error(tc.description, len(result), tc.count)
			t.Error(result, tc)
		}
	}
}

func TestParseInclude(t *testing.T) {
	testCases := []struct {
		line      string
		found     bool
		path      string
		recursive bool
	}{
		{
			line:      "#include /path/ok",
			found:     true,
			path:      "/path/ok",
			recursive: true,
		},
		{
			line:      "#include    /path/ok",
			found:     true,
			path:      "/path/ok",
			recursive: true,
		},
		{
			line:      "#include \"/path/ok\"",
			found:     true,
			path:      "/path/ok",
			recursive: true,
		},
		{
			line:      "#include \"/path/ok\"    ",
			found:     true,
			path:      "/path/ok",
			recursive: true,
		},
		{
			line:      "  #include \"/path/ok\"    ",
			found:     true,
			path:      "/path/ok",
			recursive: true,
		},
		{
			line:      "#iclude \"/path/ok\"    ",
			found:     false,
			path:      "",
			recursive: true,
		},
		{
			line:      "",
			found:     false,
			path:      "",
			recursive: true,
		},
		{
			line:      "#include \"/path  with space \" ",
			found:     true,
			path:      "/path  with space ",
			recursive: true,
		},
		{
			line:      "#include /path  with space without quotes ",
			found:     false,
			path:      "",
			recursive: true,
		},
		// New test cases for #merge directive
		{
			line:      "#merge /path/ok",
			found:     true,
			path:      "/path/ok",
			recursive: true,
		},
		{
			line:      "#merge recursive=false /path/ok",
			found:     true,
			path:      "/path/ok",
			recursive: false,
		},
		{
			line:      "#merge recursive=true /path/ok",
			found:     true,
			path:      "/path/ok",
			recursive: true,
		},
		{
			line:      "#merge recursive=false \"/path/with space\"",
			found:     true,
			path:      "/path/with space",
			recursive: false,
		},
		{
			line:      "  #merge  recursive=false  /path/ok  ",
			found:     true,
			path:      "/path/ok",
			recursive: false,
		},
	}

	for _, tc := range testCases {
		found, include := parseInclude(tc.line)
		if found != tc.found {
			t.Errorf("TestCase failed: %v, got found=%v", tc, found)
			continue
		}
		if found && (include.path != tc.path || include.recursive != tc.recursive) {
			t.Errorf("TestCase failed: %v, got found=%v path=%q recursive=%v", tc, found, include.path, include.recursive)
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

func TestMergeRecursive(t *testing.T) {
	initLoggers()
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()

	// Test recursive=true (default): should include third.yaml
	mergedTrue, _, err := mergeVars("fixtures/test/MERGE_DEPTH_TEST/test-recursive-true.yaml", mergeStrategies)
	if err != nil {
		t.Fatal("recursive=true merge failed:", err)
	}

	// With recursive=true, foo should come from third.yaml
	if val, ok := mergedTrue["foo"]; ok {
		if val != "from-third" {
			t.Errorf("With recursive=true, expected foo='from-third', got '%v'", val)
		}
	} else {
		t.Error("With recursive=true, 'foo' not found in merged vars")
	}

	if _, ok := mergedTrue["only-in-second"]; !ok {
		t.Error("With recursive=true, 'only-in-second' should be defined")
	}

	if _, ok := mergedTrue["only-in-third"]; !ok {
		t.Error("With recursive=true, 'only-in-third' should be defined")
	}
	if _, ok := mergedTrue["only-in-first"]; !ok {
		t.Error("With recursive=true, 'only-in-first' should be defined")
	}
	// Ensure config.level1.level2.foo == "from-third"
	if level1, ok := mergedTrue["config"].(map[string]any); ok {
		if level2, ok := level1["level1"].(map[string]any); ok {
			if val, ok := level2["level2"].(map[string]any); ok {
				if fooVal, ok := val["foo"]; ok {
					if fooVal != "from-third" {
						t.Errorf("With recursive=true, expected config.level1.level2.foo='from-third', got '%v'", fooVal)
					}
				} else {
					t.Error("With recursive=true, 'config.level1.level2.foo' not found")
				}
			} else {
				t.Error("With recursive=true, 'config.level1.level2' not found")
			}
		} else {
			t.Error("With recursive=true, 'config.level1' not found")
		}
	} else {
		t.Error("With recursive=true, 'config' not found")
	}

	// Test recursive=false: should NOT include third.yaml
	mergedFalse, _, err := mergeVars("fixtures/test/MERGE_DEPTH_TEST/test-recursive-false.yaml", mergeStrategies)
	if err != nil {
		t.Fatal("recursive=false merge failed:", err)
	}

	// With recursive=false, foo should come from test-recursive-false.yaml itself
	if val, ok := mergedFalse["foo"]; ok {
		if val != "from-second" {
			t.Errorf("With recursive=false, expected foo='from-second', got '%v'", val)
		}
	} else {
		t.Error("With recursive=false, 'foo' not found in merged vars")
	}

	if _, ok := mergedFalse["only-in-third"]; ok {
		t.Error("With recursive=false, 'only-in-third' should be undefined")
	}
	if _, ok := mergedFalse["only-in-second"]; !ok {
		t.Error("With recursive=false, 'only-in-second' should be defined")
	}

	if _, ok := mergedFalse["only-in-first"]; !ok {
		t.Error("With recursive=false, 'only-in-first' should be defined")
	}

	// Ensure config.level1.level2.foo == "from-second"
	if level1, ok := mergedFalse["config"].(map[string]any); ok {
		if level2, ok := level1["level1"].(map[string]any); ok {
			if val, ok := level2["level2"].(map[string]any); ok {
				if fooVal, ok := val["foo"]; ok {
					if fooVal != "from-second" {
						t.Errorf("With recursive=false, expected config.level1.level2.foo='from-second', got '%v'", fooVal)
					}
				} else {
					t.Error("With recursive=false, 'config.level1.level2.foo' not found")
				}
			} else {
				t.Error("With recursive=false, 'config.level1.level2' not found")
			}
		} else {
			t.Error("With recursive=false, 'config.level1' not found")
		}
	} else {
		t.Error("With recursive=false, 'config' not found")
	}
}

func TestIncludeRecursiveFalse(t *testing.T) {
	initLoggers()
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()

	// Test with #include recursive=false
	mergedFalse, _, err := mergeVars(
		"fixtures/test/MERGE_DEPTH_TEST/test-include-recursive-false.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	// With recursive=false on #include, third.yaml should NOT be included
	// So foo should be from-first (the main file overrides second.yaml)
	if fooVal, ok := mergedFalse["foo"]; !ok {
		t.Error("With #include recursive=false, 'foo' not found in merged vars")
	} else if fooVal != "from-first" {
		t.Errorf("With #include recursive=false, expected foo='from-first', got '%v'", fooVal)
	}

	// only-in-third should NOT be present (third.yaml was not processed)
	if _, ok := mergedFalse["only-in-third"]; ok {
		t.Error("With #include recursive=false, 'only-in-third' should be undefined")
	}

	// only-in-second should be present (second.yaml was included)
	if _, ok := mergedFalse["only-in-second"]; !ok {
		t.Error("With #include recursive=false, 'only-in-second' should be defined")
	}

	// onlyinfirst should be present
	if _, ok := mergedFalse["onlyinfirst"]; !ok {
		t.Error("With #include recursive=false, 'onlyinfirst' should be defined")
	}
}

func TestMixedIncludeMerge(t *testing.T) {
	initLoggers()
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()

	// Test with mixed #include and #merge directives
	merged, includeList, err := mergeVars(
		"fixtures/test/MIXED_DIRECTIVES/test.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Expected merge order for the MIXED_DIRECTIVES files should be:
	// 1. account.yaml (from getMergeList for catalog items)
	// 2. base.yaml (from #include - comes before test.yaml)
	// 3. test.yaml (the main file)
	// 4. override.yaml (from #merge - comes after test.yaml)
	expectedOrder := []string{
		"/test/MIXED_DIRECTIVES/account.yaml",
		"/test/MIXED_DIRECTIVES/base.yaml",
		"/test/MIXED_DIRECTIVES/test.yaml",
		"/test/MIXED_DIRECTIVES/override.yaml",
	}

	// Find the MIXED_DIRECTIVES account.yaml in the include list
	relevantStart := -1
	for i, inc := range includeList {
		if strings.HasSuffix(inc.path, "/test/MIXED_DIRECTIVES/account.yaml") {
			relevantStart = i
			break
		}
	}

	if relevantStart == -1 {
		t.Fatal("Could not find MIXED_DIRECTIVES/account.yaml in merge list")
	}

	for i, expected := range expectedOrder {
		actualIdx := relevantStart + i
		if actualIdx >= len(includeList) {
			t.Fatalf("Merge list too short, expected %s at position %d", expected, actualIdx)
		}
		if !strings.HasSuffix(includeList[actualIdx].path, expected) {
			t.Errorf("Expected %s at position %d, got %s", expected, actualIdx, includeList[actualIdx].path)
		}
	}

	// Check merged values
	// 'foo' should be from-override (override.yaml comes last)
	if val, ok := merged["foo"]; !ok {
		t.Error("'foo' not found in merged vars")
	} else if val != "from-override" {
		t.Errorf("Expected foo='from-override', got '%v'", val)
	}

	// All three *_only fields should be present
	if _, ok := merged["base_only"]; !ok {
		t.Error("'base_only' should be defined")
	}
	if _, ok := merged["test_only"]; !ok {
		t.Error("'test_only' should be defined")
	}
	if _, ok := merged["override_only"]; !ok {
		t.Error("'override_only' should be defined")
	}
}

func TestIncludeCatalogItemNoCommonFiles(t *testing.T) {
	initLoggers()
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()

	// Test that when including/merging a catalog item, its parent common files are NOT included
	merged, includeList, err := mergeVars(
		"fixtures/test/INCLUDE_CATALOG_ITEM_CONSUMER/including.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	// The merge list should NOT contain common.yaml from INCLUDE_CATALOG_ITEM
	// It should contain:
	// - /common.yaml (top-level)
	// - /test/account.yaml
	// - /test/INCLUDE_CATALOG_ITEM/account.yaml
	// - /test/INCLUDE_CATALOG_ITEM/target.yaml (from #merge)
	// - /test/INCLUDE_CATALOG_ITEM/including.yaml (the main file)

	// Verify common.yaml from INCLUDE_CATALOG_ITEM is NOT in the list
	for _, inc := range includeList {
		if strings.Contains(inc.path, "INCLUDE_CATALOG_ITEM/common.yaml") {
			t.Error("common.yaml from INCLUDE_CATALOG_ITEM should NOT be in merge list when target.yaml is merged")
		}
	}

	// Verify that target.yaml IS in the list
	foundTarget := false
	for _, inc := range includeList {
		if strings.HasSuffix(inc.path, "INCLUDE_CATALOG_ITEM/target.yaml") {
			foundTarget = true
			break
		}
	}
	if !foundTarget {
		t.Error("target.yaml should be in the merge list")
	}

	// Check merged values
	// 'foo' should be from-target (target.yaml is merged AFTER including.yaml, so it overrides)
	if val, ok := merged["foo"]; !ok {
		t.Error("'foo' not found in merged vars")
	} else if val != "from-target" {
		t.Errorf("Expected foo='from-target', got '%v'", val)
	}

	// common_value from common.yaml should NOT be present
	if _, ok := merged["common_value"]; ok {
		t.Error("'common_value' from common.yaml should NOT be present when target.yaml is merged")
	}

	// Both target_only and including_only should be present
	if _, ok := merged["target_only"]; !ok {
		t.Error("'target_only' should be defined")
	}
	if _, ok := merged["including_only"]; !ok {
		t.Error("'including_only' should be defined")
	}
}

func TestIncludeCatalogItemWithCommonFiles(t *testing.T) {
	initLoggers()
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()

	// Test that when including/merging a catalog item with recursive=true (default),
	// its parent common files ARE included in the merge list
	merged, includeList, err := mergeVars(
		"fixtures/test/INCLUDE_CATALOG_ITEM_CONSUMER/including-recursive-true.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	// The merge list SHOULD contain common.yaml from INCLUDE_CATALOG_ITEM
	// because we're using recursive=true (default) and target.yaml is a catalog item
	foundCommon := false
	for _, inc := range includeList {
		if strings.Contains(inc.path, "INCLUDE_CATALOG_ITEM/common.yaml") {
			foundCommon = true
			break
		}
	}
	if !foundCommon {
		t.Error("common.yaml from INCLUDE_CATALOG_ITEM SHOULD be in merge list when target.yaml is merged with recursive=true")
	}

	// Verify that target.yaml IS in the list
	foundTarget := false
	for _, inc := range includeList {
		if strings.HasSuffix(inc.path, "INCLUDE_CATALOG_ITEM/target.yaml") {
			foundTarget = true
			break
		}
	}
	if !foundTarget {
		t.Error("target.yaml should be in the merge list")
	}

	// Check merged values
	// 'foo' should be from-including (target.yaml is merged BEFORE including-recursive-true.yaml)
	// becaues we're using an '#include' in this case.
	if val, ok := merged["foo"]; !ok {
		t.Error("'foo' not found in merged vars")
	} else if val != "from-including" {
		t.Errorf("Expected foo='from-including', got '%v'", val)
	}

	// common_value from common.yaml SHOULD be present with recursive=true
	if val, ok := merged["common_value"]; !ok {
		t.Error("'common_value' from common.yaml SHOULD be present when target.yaml is merged with recursive=true")
	} else if val != "from-common-file" {
		t.Errorf("Expected common_value='from-common-file', got '%v'", val)
	}

	// Both target_only and including_only should be present
	if _, ok := merged["target_only"]; !ok {
		t.Error("'target_only' should be defined")
	}
	if _, ok := merged["including_only"]; !ok {
		t.Error("'including_only' should be defined")
	}
}

func TestMergeCommonFileWalksUpParents(t *testing.T) {
	initLoggers()
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()

	// Test that when explicitly merging a common.yaml file (even though it's not a catalog item),
	// it should still walk up the directory tree and merge parent common files
	// This is a regression test for the bug where getMergeList was returning early for non-catalog items
	merged, includeList, err := mergeVars(
		"fixtures/test/COMMON_MERGE_TEST/subdir/common.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	// The merge list should contain both parent and child common.yaml files
	foundParent := false
	foundChild := false
	for _, inc := range includeList {
		if strings.HasSuffix(inc.path, "COMMON_MERGE_TEST/common.yaml") {
			foundParent = true
		}
		if strings.HasSuffix(inc.path, "COMMON_MERGE_TEST/subdir/common.yaml") {
			foundChild = true
		}
	}
	if !foundParent {
		t.Error("Parent common.yaml should be in merge list when explicitly merging a child common.yaml")
	}
	if !foundChild {
		t.Error("Child common.yaml should be in merge list")
	}

	// Check merged values
	// parent_value should be present (from parent common.yaml)
	if val, ok := merged["parent_value"]; !ok {
		t.Error("'parent_value' from parent common.yaml should be present")
	} else if val != "from-parent-common" {
		t.Errorf("Expected parent_value='from-parent-common', got '%v'", val)
	}

	// child_value should be present (from child common.yaml)
	if val, ok := merged["child_value"]; !ok {
		t.Error("'child_value' from child common.yaml should be present")
	} else if val != "from-child-common" {
		t.Errorf("Expected child_value='from-child-common', got '%v'", val)
	}

	// shared_value should be from child (child overrides parent)
	if val, ok := merged["shared_value"]; !ok {
		t.Error("'shared_value' should be present")
	} else if val != "from-child" {
		t.Errorf("Expected shared_value='from-child' (child should override parent), got '%v'", val)
	}
}

func TestSchemaValidationPatternFailed(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initMergeStrategies()
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
	initMergeStrategies()
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
	initMergeStrategies()
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
	initMergeStrategies()
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
	initMergeStrategies()
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

func TestLoadInto(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initMergeStrategies()
	validateFlag = true
	initSchemaList()
	m, _, _ := mergeVars("fixtures/test/BABYLON_EMPTY_CONFIG/prod.yaml", mergeStrategies)
	_, value, _, _ := Get(m, "/__meta__/catalog")

	// Ensure other keys are still there
	// 3 keys initially, including .description that will be overriden
	// +1 key from the related_file load_into (descriptionFormat)
	// 3 + 1 = 4
	elems := len(value.(map[string]any))
	if elems != 3 {
		t.Error("__meta__.catalog should have 4 keys after merging, found", elems)
	}

	if value.(map[string]any)["description"] != "test adoc content\n" {
		t.Error("__meta__.catalog.description is not correct")
	}
}

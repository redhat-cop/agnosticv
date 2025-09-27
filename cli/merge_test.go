package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

var exampleDoc = map[string]any{
	"foo": "bar",
	"__meta__": map[string]any{
		"foo": "bar",
		"array": []any{
			"1",
			"2",
			map[string]any{
				"foo": "bar",
			},
		},
	},
}

func BenchmarkGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, _, _, err := Get(exampleDoc, "/foo"); err != nil {
			return
		}
	}
}

func TestGet(t *testing.T) {
	testCases := []struct {
		doc     map[string]any
		pointer string
		found   bool
		value   any
		err     error
	}{
		{
			doc:     exampleDoc,
			pointer: "",
			found:   true,
			value:   exampleDoc,
			err:     nil,
		},
		{
			doc:     exampleDoc,
			pointer: "/foo",
			found:   true,
			value:   "bar",
			err:     nil,
		},
		{
			doc:     exampleDoc,
			pointer: "/foo2",
			found:   false,
			value:   map[string]any{},
			err:     nil,
		},
		{
			doc:     exampleDoc,
			pointer: "/__meta__/foo",
			found:   true,
			value:   "bar",
			err:     nil,
		},
		{
			doc:     exampleDoc,
			pointer: "/__meta__/array/0",
			found:   true,
			value:   "1",
			err:     nil,
		},
		{
			doc:     exampleDoc,
			pointer: "/__meta__/array/2/foo",
			found:   true,
			value:   "bar",
			err:     nil,
		},
		{
			doc:     exampleDoc,
			pointer: "/__meta__/array/3",
			found:   false,
			value:   map[string]any{},
			err:     nil,
		},
		{
			doc:     exampleDoc,
			pointer: "__me",
			found:   false,
			value:   map[string]any{},
			err:     fmt.Errorf("JSON pointer must be empty or start with a \"/"),
		},
	}

	for _, tc := range testCases {
		found, value, _, err := Get(tc.doc, tc.pointer)
		if err != nil {
			if tc.err == nil {
				t.Fatal("error found when not expected:", err)
			}

			if tc.err.Error() != err.Error() {
				t.Fatal("error not the same", err, tc.err)
			}
		}

		if found != tc.found {
			t.Fatal(tc, found)
		}
		if found {
			if reflect.TypeOf(value) != reflect.TypeOf(tc.value) {
				t.Fatal("value not same type", value, tc.value)
			}
			if !reflect.DeepEqual(value, tc.value) {
				t.Fatal("value not the same", value, tc.value)
			}
		}
	}
}

func TestInitMapJSON(t *testing.T) {
	dst := make(map[string]any)
	var expected map[string]any = map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": map[string]any{},
				},
			},
		},
	}

	initMap(dst, []string{"a", "b", "c", "d"})

	if !reflect.DeepEqual(dst, expected) {
		t.Error("initMap", dst, expected)
	}
}

func TestSet(t *testing.T) {
	dst := make(map[string]any)
	var source map[string]any = map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"key": "value",
				},
			},
		},
	}

	if err := Set(dst, "/a/b/c/key", source); err != nil {
		t.Fatal("Set failed with:", err)
	}
	if !reflect.DeepEqual(dst, source) {
		t.Error("Set didn't work as expected.", dst, source)
	}
}

func BenchmarkMergeJSON(b *testing.B) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initMergeStrategies()
	validateFlag = true
	initSchemaList()

	for i := 0; i < b.N; i++ {
		_, _, err := mergeVars(
			"fixtures/test/BABYLON_EMPTY_CONFIG/dev.yaml",
			mergeStrategies,
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestMergeCatalogItemIncluded(t *testing.T) {
	initLoggers()
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()
	validateFlag = true
	merged, _, err := mergeVars(
		"fixtures/test/foo/prod.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, value, _, err := Get(merged, "/__meta__/catalog/description")
	if err != nil {
		t.Error(err)
	}
	if value != "from description.adoc\n" {
		t.Error("description is not 'from description.adoc'", value)
	}
}

func TestMergeCatalogItemIncludedWithOrder(t *testing.T) {
	initLoggers()
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()
	validateFlag = true
	merged, _, err := mergeVars(
		"fixtures/test/foo/order.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, value, _, err := Get(merged, "/foo")
	if err != nil {
		t.Error(err)
	}
	if value != "include3" {
		t.Error("foo should be 'include3'", value)
	}

	_, value, _, err = Get(merged, "/bar")
	if err != nil {
		t.Error(err)
	}
	if value != "order.yaml" {
		t.Error("bar should be 'order.yaml'", value)
	}
}

func TestMerge(t *testing.T) {
	initLoggers()
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()
	validateFlag = true
	merged, _, err := mergeVars(
		"fixtures/test/BABYLON_EMPTY_CONFIG/dev.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	found, value, _, err := Get(merged, "/__meta__/secrets")
	if err != nil {
		t.Error(err)
	}
	if !found || len(value.([]any)) != 5 {
		logErr.Println(value)
		t.Error("/__meta__/secrets is missing or doesn't countain 5 elements'",
			", found =", found, ", err =", err, ", #elem =", len(value.([]any)))
	}

	found, value, _, err = Get(merged, "/__meta__/secrets/0/value")
	if !found || err != nil || value.(string) != "from-top-common.yml" {
		t.Error("/__meta__/secrets/0/value should be 'from-top-common.yml'",
			", found =", found, ", err =", err, ", value =", value.(string))
	}

	found, value, _, err = Get(merged, "/alist")
	if !found || err != nil || len(value.([]any)) != 2 {
		t.Error("/alist  should be merged from account.yaml, and dev.yaml, and thus of length 2")
	}

	found, value, _, err = Get(merged, "/noappend_dict/a")
	if !found || err != nil || len(value.([]any)) != 1 {
		t.Error("/noappend_list should be of length 1")
	}

	// Ensure includes work as expected in meta file
	// see fixtures/test/BABYLON_EMPTY_CONFIG_AWS/test.__meta__yaml
	merged, includeList, err := mergeVars(
		"fixtures/test/BABYLON_EMPTY_CONFIG_AWS/test.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedMergeList := []string{
		"/common.yaml",
		"/test/account.yaml",
		"/test/BABYLON_EMPTY_CONFIG_AWS/common.yaml",
		"/includes/include1.meta.yaml",
		"/includes/include1.yaml",
		"/test/BABYLON_EMPTY_CONFIG_AWS/test.meta.yaml",
		"/includes/include2.meta.yml",
		"/test/BABYLON_EMPTY_CONFIG_AWS/test.yaml",
	}
	for i, v := range expectedMergeList {
		if !strings.HasSuffix(includeList[i].path, v) {
			t.Error(v, "not at the position", i, "in the merge list of ",
				"fixtures/test/BABYLON_EMPTY_CONFIG_AWS/test.yaml",
				"found", includeList[i].path, "instead")
		}
	}

	if v, ok := merged["from_include1"]; !ok || v != "value1" {
		t.Error("Value from include1.yaml not found in the merge result.")
	}
	// Ensure __meta__.from_include2_meta  is defined
	found, value, _, err = Get(merged, "/__meta__/from_include2_meta")
	if !found || err != nil || value != "value2" {
		t.Error("/__meta__/from_include2_meta  be merged from include")
	}

	found, value, _, err = Get(merged, "/__meta__/from_include1_meta")
	if !found || err != nil || value != "value1" {
		t.Error("/__meta__/from_include1_meta  be merged from detected meta")
	}

	_, includeList, err = mergeVars(
		"fixtures/test/foo/order.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	expectedMergeList = []string{
		"/common.yaml",
		"/test/account.yaml",
		"/includes/order1.yaml",
		"/includes/order21.yaml",
		"/includes/order22.yaml",
		"/includes/order23.yaml",
		"/includes/order2.yaml",
		"/includes/order3.yaml",
		"/test/foo/order.yaml",
	}
	for i, v := range expectedMergeList {
		if !strings.HasSuffix(includeList[i].path, v) {
			t.Error(v, "not at the position", i, "in the merge list of ",
				"fixtures/test/foo/order.yaml",
				"found", includeList[i].path, "instead")
		}
	}
}

func TestMergeStrategyOverwrite(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()
	validateFlag = true
	merged, _, err := mergeVars(
		"fixtures/test/BABYLON_EMPTY_CONFIG/prod.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, value, _, err := Get(merged, "/__meta__/access_control/allow_groups")
	if err != nil {
		t.Error(err)
	}
	expected := []any{
		"myspecialgroup",
	}
	if !reflect.DeepEqual(value, expected) {
		t.Error("Only myspecialgroup should be present", value, expected)
	}
}

func TestMergeStrategicMergeList(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()
	validateFlag = true
	merged, _, err := mergeVars(
		"fixtures/test/BABYLON_EMPTY_CONFIG/prod.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, value, _, err := Get(merged, "/adict/strategic_list")
	if err != nil {
		t.Error(err)
	}
	expected := []any{
		map[string]any{
			"name":  "foo",
			"value": "prod",
		},
	}
	if !reflect.DeepEqual(value, expected) {
		t.Error("Only prod should be present", value, expected)
	}

	_, value, _, err = Get(merged, "/strategic_dict/alist")
	if err != nil {
		t.Error(err)
	}
	expected = []any{"1", "2", "3", "4"}
	if !reflect.DeepEqual(value, expected) {
		t.Error("lists do not match", value, expected)
	}
	_, value, _, err = Get(merged, "/__meta__/catalog/keywords")
	if err != nil {
		t.Error(err)
	}
	expected = []any{"keyword1", "keyword2"}
	if !reflect.DeepEqual(value, expected) {
		t.Error("lists do not match", value, expected)
	}
}

func TestRelativeFileLoadInto(t *testing.T) {
	rootFlag = abs("fixtures")
	initConf(rootFlag)
	initSchemaList()
	initMergeStrategies()
	validateFlag = true
	merged, _, err := mergeVars(
		"fixtures/test/BABYLON_EMPTY_CONFIG/prod.yaml",
		mergeStrategies,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, value, _, err := Get(merged, "/__meta__/service_ready")
	if err != nil {
		t.Error(err)
	}
	expected := map[string]any{
		"format":        "jinja2",
		"template":      "jinja2 content",
		"output_format": "html",
	}

	if !reflect.DeepEqual(value, expected) {
		t.Error("Values do not match", value, expected)
	}

}

func TestParseInsert(t *testing.T) {
	testCases := []struct {
		line     string
		expected Insert
		valid    bool
	}{
		{
			line: `#insert "file.yaml"`,
			expected: Insert{
				path: "file.yaml",
			},
			valid: true,
		},
		{
			line: `#insert file.yaml`,
			expected: Insert{
				path: "file.yaml",
			},
			valid: true,
		},
		{
			line:  `not an insert`,
			valid: false,
		},
		{
			line:  `#include "file.yaml"`,
			valid: false,
		},
	}

	for _, tc := range testCases {
		valid, insert := parseInsert(tc.line)
		if valid != tc.valid {
			t.Errorf("Expected valid=%v, got %v for line: %s", tc.valid, valid, tc.line)
			continue
		}
		if valid && !reflect.DeepEqual(insert, tc.expected) {
			t.Errorf("Expected %+v, got %+v for line: %s", tc.expected, insert, tc.line)
		}
	}
}

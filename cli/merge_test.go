package main

import (
	"fmt"
	"testing"
	"reflect"
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
		Get(exampleDoc, "/foo")
	}
}

func TestGet(t *testing.T) {
	testCases := []struct {
		doc map[string]any
		pointer string
		found bool
		value any
		err error
	}{
		{
			doc: exampleDoc,
			pointer: "",
			found: true,
			value: exampleDoc,
			err: nil,
		},
		{
			doc: exampleDoc,
			pointer: "/foo",
			found: true,
			value: "bar",
			err: nil,
		},
		{
			doc: exampleDoc,
			pointer: "/foo2",
			found: false,
			value: map[string]any{},
			err: nil,
		},
		{
			doc: exampleDoc,
			pointer: "/__meta__/foo",
			found: true,
			value: "bar",
			err: nil,
		},
		{
			doc: exampleDoc,
			pointer: "/__meta__/array/0",
			found: true,
			value: "1",
			err: nil,
		},
		{
			doc: exampleDoc,
			pointer: "/__meta__/array/2/foo",
			found: true,
			value: "bar",
			err: nil,
		},
		{
			doc: exampleDoc,
			pointer: "/__meta__/array/3",
			found: false,
			value: map[string]any{},
			err: nil,
		},
		{
			doc: exampleDoc,
			pointer: "__me",
			found: false,
			value: map[string]any{},
			err: fmt.Errorf("JSON pointer must be empty or start with a \"/"),
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
		"a": map[string]any {
			"b": map[string]any {
				"c": map[string]any {
					"d": map[string]any {
					},
				},
			},
		},
	}

	initMap(dst, []string{"a", "b", "c", "d"} )

	if !reflect.DeepEqual(dst, expected) {
		t.Error("initMap", dst, expected)
	}
}

func TestSet(t *testing.T) {
	dst := make(map[string]any)
	var source map[string]any = map[string]any{
		"a": map[string]any {
			"b": map[string]any {
				"c": map[string]any {
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

func TestMerge(t *testing.T) {
	rootFlag = abs("fixtures")
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
}

func TestMergeStrategyOverwrite(t *testing.T) {
	rootFlag = abs("fixtures")
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

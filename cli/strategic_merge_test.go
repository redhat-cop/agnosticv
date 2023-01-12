package main

import (
	"reflect"
	"testing"

	yamljson "github.com/ghodss/yaml"
)

func TestCleanupSlice(t *testing.T) {
	testCases := []struct {
		doc      []byte
		expected []byte
		err      error
	}{
		{
			doc:      []byte(`[]`),
			expected: []byte(`[]`),
			err:      nil,
		},
		{
			doc:      []byte(`[1,2,3]`),
			expected: []byte(`[1,2,3]`),
			err:      nil,
		},
		{
			doc: []byte(`
- name: foo
  value: bar
- name: foo
  value: bar2
- name: foo
  value: bar3
`),
			expected: []byte(`
- name: foo
  value: bar3
`),
			err: nil,
		},
		// Test when there are several reductions to make
		{
			doc: []byte(`
- name: foo
  value: bar
- name: foo
  value: bar2
- name: foo
  value: bar2
- name: foo2
  value: bar
- name: foo
  value: bar3
- name: foo2
  value: bar2
- name: foo2
  value: bar3
`),
			expected: []byte(`
- name: foo
  value: bar3
- name: foo2
  value: bar3
`),
			err: nil,
		},
		{
			doc: []byte(`
- name: foo
  value: bar
- name: anotherkey
  value: bar
- name: anotherkey2
  value: bar
- name: foo
  value: bar2
- name: foo
  value: bar3
`),
			expected: []byte(`
- name: foo
  value: bar3
- name: anotherkey
  value: bar
- name: anotherkey2
  value: bar
`),
			err: nil,
		},
		{
			doc: []byte(`
- name: foo
  value: bar
- foo
- foo
- bar
- name: foo
  value: bar2
- bar
- name: foo
  value: bar3
- bar
- name: foo
  value: bar4
`),
			expected: []byte(`
- name: foo
  value: bar4
- foo
- foo
- bar
- bar
- bar
`),
			err: nil,
		},
	}

	for _, tc := range testCases {
		doc := []any{}
		expected := []any{}
		if err := yamljson.Unmarshal(tc.doc, &doc); err != nil {
			t.Fatal("cannot unmarshal", tc.doc)
		}
		if err := yamljson.Unmarshal(tc.expected, &expected); err != nil {
			t.Fatal("cannot unmarshal", tc.expected)
		}

		cleanDoc, err := strategicCleanupSlice(doc)
		if err != nil {
			t.Fatal("error in strategicCleanupSlice: ", err)
		}

		if !reflect.DeepEqual(expected, cleanDoc) {
			t.Error("strategicCleanupSlice: ", cleanDoc, "!=", expected)
		}

	}
}

func TestCleanupMap(t *testing.T) {
	testCases := []struct {
		doc      []byte
		expected []byte
		err      error
	}{
		{
			doc:      []byte(`{}`),
			expected: []byte(`{}`),
			err:      nil,
		},
		{
			doc: []byte(`
foo: bar
alist:
  - foo
  - name: foo
    value: bar
    key: present
  - bar
  - name: foo
    value: bar2
    key: changed
`),
			expected: []byte(`
foo: bar
alist:
  - foo
  - name: foo
    value: bar2
    key: changed
  - bar
`),
			err: nil,
		},
	}

	for _, tc := range testCases {
		doc := map[string]any{}
		expected := map[string]any{}
		if err := yamljson.Unmarshal(tc.doc, &doc); err != nil {
			t.Fatal("cannot unmarshal", tc.doc)
		}
		if err := yamljson.Unmarshal(tc.expected, &expected); err != nil {
			t.Fatal("cannot unmarshal", tc.expected)
		}

		err := strategicCleanupMap(doc)
		if err != nil {
			t.Fatal("error in strategicCleanupMap: ", err)
		}

		if !reflect.DeepEqual(expected, doc) {
			t.Error("strategicCleanupMap: ", doc, "!=", expected)
		}

	}
}

func TestStrategicMerge(t *testing.T) {
	testCases := []struct {
		src      []byte
		dst      []byte
		expected []byte
		err      error
	}{
		{
			src:      []byte(`{}`),
			dst:      []byte(`{}`),
			expected: []byte(`{}`),
			err:      nil,
		},
		{
			src: []byte(`
foo: bar
alist:
  - name: foo
    value: 1
  - name: foo
    value: 2

`),
			dst: []byte(`
foo: bar2
alist: []`),
			expected: []byte(`
foo: bar
alist:
  - name: foo
    value: 2
`),
			err: nil,
		},
		{
			src: []byte(`
foo: bar
nested:
  foo: bar
  alist:
    - name: foo
      value: 1
    - name: foo
      value: 2
`),
			dst: []byte(`foo: bar2`),
			expected: []byte(`
foo: bar
nested:
  foo: bar
  alist:
    - name: foo
      value: 2
`),
			err: nil,
		},
		{
			src: []byte(`
foosrc: bar
nested:
  foo: src
  alist:
    - name: foo
      value: src
`),
			dst: []byte(`
foodst: bar
nested:
  alist:
    - name: foo
      value: dst
`),
			expected: []byte(`
foodst: bar
foosrc: bar
nested:
  foo: src
  alist:
    - name: foo
      value: src
`),
			err: nil,
		},
	}

	for _, tc := range testCases {
		src := map[string]any{}
		dst := map[string]any{}
		expected := map[string]any{}
		if err := yamljson.Unmarshal(tc.src, &src); err != nil {
			t.Fatal("cannot unmarshal", tc.src)
		}
		if err := yamljson.Unmarshal(tc.dst, &dst); err != nil {
			t.Fatal("cannot unmarshal", tc.dst)
		}
		if err := yamljson.Unmarshal(tc.expected, &expected); err != nil {
			t.Fatal("cannot unmarshal", tc.expected)
		}

		err := strategicMerge(dst, src)
		if err != nil {
			t.Fatal("error in strategicCleanupMap: ", err)
		}

		if !reflect.DeepEqual(dst, expected) {
			t.Error("strategicCleanupMap: ", dst, "!=", expected)
		}

	}
}

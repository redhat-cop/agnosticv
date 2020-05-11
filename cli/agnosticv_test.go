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

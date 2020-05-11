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

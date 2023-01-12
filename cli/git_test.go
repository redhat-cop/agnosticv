package main

import (
	"testing"
)

func TestIsRepo(t *testing.T) {
	if isRepo("/tmp") {
		t.Error("/tmp is a repo???")
	}

	if isRepo(".") == false {
		t.Error(". is not in a repo")
	}

	if isRepo("agnosticv.go") == false {
		t.Error("agnosticv.go is not in a repo")
	}
}

func TestFindMostRecentCommit(t *testing.T) {

	if commit := findMostRecentCommit("agnosticv.go", []Include{}); commit.Hash.IsZero() {
		t.Error(commit)
	}

}

package main

import (
	"io"
	"testing"
)

func TestFlag(t *testing.T) {
	testCases := []struct {
		args        []string
		description string
		result      controlFlow
	}{
		{
			args:        []string{"agnosticv", "--list"},
			description: "List catalog items",
			result:      controlFlow{false, 0},
		},
		{
			args:        []string{"agnosticv", "-version"},
			description: "Get the version",
			result:      controlFlow{true, 0},
		},
		{
			args:        []string{"agnosticv", "--list", "--has", "__meta__"},
			description: "List with 'has' flag",
			result:      controlFlow{false, 0},
		},
		{
			args:        []string{"agnosticv", "--merge", "fixtures/test/BABYLON_EMPTY_CONFIG/dev.yaml"},
			description: "Simple merge",
			result:      controlFlow{false, 0},
		},
		{
			args: []string{"agnosticv", "--list", "--has", "__meta__",
				"--related", "fixtures/test/BABYLON_EMPTY_CONFIG/dev.yaml",
				"--or-related", "fixtures/test/BABYLON_EMPTY_CONFIG/prod.yaml"},
			description: "List with 'has' flag and 'related' and 'or-related'",
			result:      controlFlow{false, 0},
		},
		{
			args:        []string{"agnosticv", "--has", "__meta__"},
			description: "Just 'has' flag without list should fail",
			result:      controlFlow{true, 2},
		},
		{
			args:        []string{"agnosticv", "--related", "foo"},
			description: "related without list should fail",
			result:      controlFlow{true, 2},
		},
		{
			args:        []string{"agnosticv", "--or-related", "foo"},
			description: "or-related without list should fail",
			result:      controlFlow{true, 2},
		},
		{
			args:        []string{"agnosticv", "--root", "fixtures"},
			description: "-merge and -list both missing",
			result:      controlFlow{true, 2},
		},
		{
			args: []string{"agnosticv", "--list",
				"--merge", "fixtures/test/BABYLON_EMPTY_CONFIG/dev.yaml"},
			description: "-merge and -list both provided",
			result:      controlFlow{true, 2},
		},
		{
			args: []string{"agnosticv", "--list",
				"--output", "json"},
			description: "-output and -list, json output",
			result:      controlFlow{false, 0},
		},
		{
			args: []string{"agnosticv", "--list",
				"--output", "yaml"},
			description: "-output and -list, yaml output ",
			result:      controlFlow{false, 0},
		},
		{
			args: []string{"agnosticv", "--list",
				"--output", "unknown"},
			description: "-output and -list, wrong output",
			result:      controlFlow{true, 2},
		},
		{
			args: []string{"agnosticv",
				"--merge", "fixtures/test/BABYLON_EMPTY_CONFIG/dev.yaml",
				"--output", "unknown"},
			description: "-output and -merge, wrong output",
			result:      controlFlow{true, 2},
		},
		{
			args: []string{"agnosticv",
				"--merge", "fixtures/test/BABYLON_EMPTY_CONFIG/dev.yaml",
				"--output", "yaml"},
			description: "-output and -merge, yaml output",
			result:      controlFlow{false, 0},
		},
	}

	for _, tc := range testCases {
		// Reinit Flags
		listFlag = false
		relatedFlags = arrayFlags{}
		orRelatedFlags = arrayFlags{}
		hasFlags = arrayFlags{}
		mergeFlag = ""
		debugFlag = false
		rootFlag = ""
		validateFlag = false
		versionFlag = false
		gitFlag = false

		result := parseFlags(tc.args, io.Discard)
		if tc.result != result {
			t.Error(tc.description, "Expected", tc.result, "but got", result)
		}
	}

}

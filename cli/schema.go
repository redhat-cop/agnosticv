package main

import (
	"fmt"
	"github.com/getkin/kin-openapi/jsoninfo"
	"github.com/getkin/kin-openapi/openapi3"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	jsonyaml "github.com/ghodss/yaml"
)

// Schema type
type Schema struct {
	path string
	schema *AgnosticvSchema
	data []byte
}

// AgnosticvSchema is openapi schema plus some extensions
type AgnosticvSchema struct {
	openapi3.Schema

	XMerge []MergeStrategy `json:"x-merge,omitempty" yaml:"x-merge,omitempty"`
}

// UnmarshalJSON sets AnosticvSchema to a copy of data.
func (schema *AgnosticvSchema) UnmarshalJSON(data []byte) error {
	return jsoninfo.UnmarshalStrictStruct(data, schema)
}

// MarshalJSON returns the JSON encoding of Schema.
func (schema *AgnosticvSchema) MarshalJSON() ([]byte, error) {
	return jsoninfo.MarshalStrictStruct(schema)
}

var schemas []Schema

func initSchemaList() {
	list, err := getSchemaList()
	if err != nil {
		logErr.Fatalf("error listing schemas: %v\n", err)
	}
	schemas = list
}

func getSchemaList() ([]Schema, error) {
	schemaDir := filepath.Join(rootFlag, ".schemas")
	if !fileExists(schemaDir) {
		logDebug.Println("schema dir not found.", schemaDir)
		return []Schema{}, nil
	}

	result := []Schema{}

	err := filepath.Walk(schemaDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			logErr.Printf("%q: %v\n", p, err)
			return err
		}
		// Work only with YAML files
		if !strings.HasSuffix(p, ".yml") && !strings.HasSuffix(p, ".yaml") {
			return nil
		}

		// 1. Read content of schema in YAML

		pAbs := abs(p)

		content, err := ioutil.ReadFile(pAbs)
		if err != nil {
			return err
		}

		// 2. convert to object

		schema := new(AgnosticvSchema)

		if err := jsonyaml.Unmarshal(content, schema); err != nil {
			return err
		}

		data, err := jsonyaml.YAMLToJSON(content)
		if err != nil {
			return err
		}

		// 3. merge Default schema if __meta__ is found

		schemaMap := make(map[string]any)
		if err := jsonyaml.Unmarshal(content, &schemaMap); err != nil {
			return err
		}

		if found, _, _, err := Get(schemaMap, "/properties/__meta__"); err == nil && found {
			defaultSchemaMap := make(map[string]any)
			if err := jsonyaml.Unmarshal([]byte(defaultSchema), &defaultSchemaMap); err != nil {
				return err
			}

			// Merge default scema into current schema

			if err := customStrategyMerge(
				schemaMap,
				defaultSchemaMap,
				MergeStrategy{
					Path: "/properties/__meta__",
					Strategy: "merge",
				},
			); err != nil {
				logErr.Println("Error merging default schema")
				return err
			}

			// rewrite schema and data

			if content, err = jsonyaml.Marshal(schemaMap); err != nil {
				return err
			}

			if err := jsonyaml.Unmarshal(content, schema); err != nil {
				return err
			}

			if data, err = jsonyaml.YAMLToJSON(content); err != nil {
				return err
			}
		}

		// Add schema

		result = append(result, Schema{
			path: pAbs,
			schema: schema,
			data: data,
		})

		return nil
	})

	return result, err
}

// DefaultSchema is the OpenAPI schema for built-in properties.
// If a schema has __meta__, this default schema will be merged into it.
var defaultSchema string = `
type: object
properties:
  __meta__:
    type: object
    properties:
      last_update:
        description: >-
          Information about last update, injected by agnosticv CLI.
        type: object
        additionalProperties: false
        properties:
          git:
            description: >-
              Information about last update from git, injected by agnosticv CLI.
            type: object
`

func validateAgainstSchemas(path string, data map[string]any) error {
	logDebug.Println("len(schemas) =", len(schemas))
	for _, schema := range schemas {
		logDebug.Println("Validating", path, "against", schema.path)
		err := schema.schema.VisitJSON(data)
		if err != nil {
			err = fmt.Errorf("%s - %w", path, err)
			return err
		}
	}
	return nil
}

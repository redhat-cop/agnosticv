package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	jsonyaml "github.com/ghodss/yaml"
)

// Schema type
type Schema struct {
	path   string
	schema *AgnosticvSchema
	data   []byte
}

// MergeStrategy type to define custom merge strategies.
// Strategy: the name of the strategy
// Path: the path in the structure of the vars to apply the strategy against.
type MergeStrategy struct {
	Strategy string `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
}

type MergeStrategies struct {
	XMerge []MergeStrategy `json:"x-merge,omitempty" yaml:"x-merge,omitempty"`
}

// AgnosticvSchema is openapi schema plus some extensions
type AgnosticvSchema struct {
	MergeStrategies
	openapi3.Schema
}

func (m *MergeStrategies) UnmarshalJSON(data []byte) error {
	type Alias MergeStrategies
	var alias Alias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	m.XMerge = alias.XMerge
	return nil
}

// UnmarshalJSON sets AnosticvSchema to a copy of data.
func (schema *AgnosticvSchema) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &schema.Schema); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &schema.MergeStrategies); err != nil {
		return err
	}
	return nil
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

		content, err := os.ReadFile(pAbs)
		if err != nil {
			return err
		}

		// 2. convert to object

		schema := new(AgnosticvSchema)

		data, err := jsonyaml.YAMLToJSON(content)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(data, schema); err != nil {
			return err
		}

		// 3. merge Default schema if __meta__ is found

		schemaMap := make(map[string]any)
		if err := json.Unmarshal(data, &schemaMap); err != nil {
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
					Path:     "/properties/__meta__",
					Strategy: "merge",
				},
			); err != nil {
				logErr.Println("Error merging default schema")
				return err
			}

			// rewrite schema and data

			if data, err = json.Marshal(schemaMap); err != nil {
				return err
			}

			if err := json.Unmarshal(data, schema); err != nil {
				return err
			}
		}

		// Add schema

		// validate schema
		if err := schema.Validate(context.Background()); err != nil {
			return err
		}
		result = append(result, Schema{
			path:   pAbs,
			schema: schema,
			data:   data,
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

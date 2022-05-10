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
			logErr.Println(err)
			return err
		}

		data, err := jsonyaml.YAMLToJSON(content)
		if err != nil {
			return err
		}

		result = append(result, Schema{
			path: pAbs,
			schema: schema,
			data: data,
		})

		return nil
	})

	return result, err
}

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

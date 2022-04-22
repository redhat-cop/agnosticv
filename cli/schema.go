package main

import (
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Schema type
type Schema struct {
	path string
	schema *openapi3.Schema
}

var schemas []Schema

func initSchemaList() error {
	list, err := getSchemaList()
	if err != nil {
		return err
	}
	schemas = list
	return nil
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

		pAbs, err := filepath.Abs(p)

		content, err := ioutil.ReadFile(pAbs)
		if err != nil {
			return err
		}

		// 2. convert to object

		schema := openapi3.NewSchema()

		if err := yaml.Unmarshal(content, schema); err != nil {
			return err
		}

		result = append(result, Schema{
			path: pAbs,
			schema: schema,
		})

		return nil
	})

	return result, err
}

// ErrorSchema for when a catalog item doesn't pass a schema
var ErrorSchema = errors.New("schema not passed")

func validateAgainstSchemas(path string, data map[string]interface{}) error {
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

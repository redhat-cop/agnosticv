package main

import (
	"errors"
	"github.com/icza/dyno"
	"github.com/xeipuuv/gojsonschema"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	yaml "gopkg.in/yaml.v2"
)

// Schema type
type Schema struct {
	path string
	schema *gojsonschema.Schema
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

	//os.Chdir(schemaDir)

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

		pAbs, err := filepath.Abs(p)

		content, err := ioutil.ReadFile(pAbs)
		if err != nil {
			return err
		}

		logDebug.Println("len(content)", len(content))

		schemaObject := make(map[string]interface{})

		err = yaml.Unmarshal([]byte(content), &schemaObject)

		if err != nil {
			return err
		}

		loader := gojsonschema.NewGoLoader(dyno.ConvertMapI2MapS(schemaObject))
		schema, err := gojsonschema.NewSchema(loader)
		if err != nil {
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
	document := gojsonschema.NewGoLoader(dyno.ConvertMapI2MapS(data))

	for _, schema := range schemas {
		logDebug.Println("Validating", path, "against", schema.path)
		result, err := schema.schema.Validate(document)
		if err != nil {
			panic(err.Error())
		}

		if result.Valid() {
			continue
		}

		logErr.Printf("%s does not validate against schema %s", path, schema.path)
		for _, desc := range result.Errors() {
			logErr.Printf("- %s\n", desc)
		}
		return ErrorSchema
	}
	return nil
}

package main

import (
	"log"
	"os"
	"path/filepath"

	yamljson "github.com/ghodss/yaml"
)

type RelatedFile struct {
	File string `json:"file"`
	LoadInto string `json:"load_into"`
	ContentKey string `json:"content_key"`
	Set map[string]interface{} `json:"set"`
}

type Config struct {
	// For any leaf file, consider those in same directory as related files:
	RelatedFiles []string `json:"related_files"`
	RelatedFilesV2 []RelatedFile`json:"related_files_v2"`

	// Plumbing variable to know when config was loaded from disk.
	initialized bool
}

func getConf(root string) Config {
	c := Config{}

	path := filepath.Join(root, ".agnosticv.yaml")

	if !fileExists(path) {
		return c
	}

	yamlFile, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Can't read config file: #%v", err)
	}

	err = yamljson.Unmarshal(yamlFile, &c)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	c.initialized = true

	return c
}

var config Config

func initConf(root string) {
	config = getConf(root)
}

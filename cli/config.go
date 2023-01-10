package main

import (
	"io/ioutil"
	"path/filepath"
	"log"

	yamljson "github.com/ghodss/yaml"
)


type Config struct {
	// For any leaf file, consider those in same directory as related files:
	RelatedFiles []string `json:"related_files"`

	// Plumbing variable to know when config was loaded from disk.
	initialized bool
}

func getConf(root string) Config {
	c := Config{}

	path := filepath.Join(root,".agnosticv.yaml")

	if !fileExists(path) { return c }

	yamlFile, err := ioutil.ReadFile(path)
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

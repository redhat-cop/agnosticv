package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	yamljson "github.com/ghodss/yaml"
	"github.com/go-openapi/jsonpointer"
	"github.com/imdario/mergo"
)


// MergeStrategy type to define custom merge strategies.
// Strategy: the name of the strategy
// Path: the path in the structure of the vars to apply the strategy against.
type MergeStrategy struct {
	Strategy string `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

// initMap initialize a map using a bunch of keys.
func initMap(m map[string]any, keys []string) {
	logDebug.Printf("(initMap) %v", keys)
	if len(keys) == 0 {
		return
	}

	if v, ok := m[keys[0]]; ok {
		next := v.(map[string]any)
		initMap(next, keys[1:])
		return
	}
	m[keys[0]] = make(map[string]any)
	next := m[keys[0]].(map[string]any)
	initMap(next, keys[1:])
	return
}


// Set copy the value of src defined by path into dst
// both dst and src are the entire maps
func Set(dst map[string]any, path string, src map[string]any) error {
	found, srcObj, _, err := Get(src, path)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	pointer, err := jsonpointer.New(path)
	if err != nil {
		return err
	}

	keys := strings.Split(path, "/")
	if len(keys) > 1 {
		// get rid of the first key that is ""
		if keys[0] == "" {
			keys = keys[1:]
		}
		// Get rid of last key too: we don't want to initialize the last element
		keys = keys[0:len(keys) - 1]
	}

	logDebug.Printf("(Set) keys %v", keys)

	// Init the map using all the keys except the last one
	initMap(dst, keys)

	logDebug.Printf("(Set) map init result: %v", dst)

	if _, err := pointer.Set(dst, srcObj); err != nil {
		return err
	}
	return nil
}

// Get check if a json pointer exists in a document
// First return value: true or false
// Second return value:  error
func Get(doc map[string]any, str string) (bool, any, reflect.Kind, error) {
	pointer, err := jsonpointer.New(str)
	if err != nil {
		return false, nil, 0, err
	}
	result, typ, err := pointer.Get(doc)
	if err != nil {
		if strings.HasPrefix(err.Error(), "object has no key") {
			return false, result, typ, nil
		}
		if strings.HasPrefix(err.Error(), "object has no field") {
			return false, result, typ, nil
		}
		if strings.HasPrefix(err.Error(), "index out of bounds") {
			return false, result, typ, nil
		}

		return false, result, typ, err
	}

	return true, result, typ, nil
}

func customStrategyMerge(final map[string]any, source map[string]any, strategy MergeStrategy) error {
	logDebug.Printf("customStrategyMerge(%v)", strategy)


	pointer, err := jsonpointer.New(strategy.Path)
	if err != nil {
		return err
	}

	srcFound, src, srcType, srcErr := Get(source, strategy.Path)
	if srcErr != nil {
		logErr.Fatal(srcErr)
	}

	dstFound, dst, dstType, dstErr := Get(final, strategy.Path)
	if dstErr != nil {
		logErr.Fatal(dstErr)
	}

	if srcFound {
		if !dstFound {
			if err := Set(final, strategy.Path, source); err != nil {
				return err
			}
			return nil
		}

		if srcType != dstType {
			return fmt.Errorf("MergeStrategy error for %v: destination and src are not the same type, %v and %v", strategy, srcType, dstType)
		}

		// Slice
		logDebug.Printf("customStrategyMerge() %v Type is %v", strategy.Path, srcType)

		if srcType == reflect.Slice {
			dst := dst.([]any)
			src := src.([]any)

			switch(strategy.Strategy) {
			case "overwrite":
				logDebug.Printf("customStrategyMerge(%v)  overwrite list", strategy)
				dst = src
			case "merge":
				logDebug.Printf("customStrategyMerge(%v)  append list", strategy)
				logDebug.Println("src", src)
				logDebug.Println("dst", dst)
				dst = append(dst, src...)

			case "strategic-merge":
				logDebug.Printf("customStrategyMerge(%v)  strategic merge", strategy)
				logDebug.Println("src", src)
				logDebug.Println("dst", dst)
				dst = append(dst, src...)
				if dst, err = strategicCleanupSlice(dst); err != nil {
					return err
				}

			default:
				logErr.Fatal("Unknown merge strategy for list: ", strategy.Strategy)
			}

			if _, err := pointer.Set(final, dst); err != nil {
				return err
			}
			return nil
		}

		// Map

		if srcType != reflect.Map {
			return fmt.Errorf("You can change merge strategy only for maps (dictionaries)")
		}

		var dstPtr any
		dstMap := dst.(map[string]any)
		dstPtr = &dstMap

		logDebug.Printf("customStrategyMerge(%v)", strategy)
		switch(strategy.Strategy) {
		case "overwrite":
			if _, err := pointer.Set(final, src); err != nil {
				return err
			}

		case "merge":
			if err := mergo.Merge(
				dstPtr,
				src,
				mergo.WithOverride,
				mergo.WithOverwriteWithEmptyValue,
				mergo.WithAppendSlice,
			); err != nil {
				return err
			}

		case "merge-no-append":
			if err := mergo.Merge(
				dstPtr,
				src,
				mergo.WithOverride,
				mergo.WithOverwriteWithEmptyValue,
			); err != nil {
				return err
			}

		case "strategic-merge":
			if err := strategicMerge(dstMap, src.(map[string]any)); err != nil {
				return err
			}

		default:
			logErr.Fatal("Unknown merge strategy ", strategy.Strategy)
		}
	}

	return nil
}

func mergeVars(p string, mergeStrategies []MergeStrategy) (map[string]any, []Include, error) {
	logDebug.Printf("mergeVars(%v)", p)

	// Work with Absolute paths
	if ! filepath.IsAbs(p) {
		if abs, errAbs := filepath.Abs(p); errAbs == nil {
			p = abs
		} else {
			return map[string]any{}, []Include{}, errAbs
		}
	}

	if rootFlag == "" {
		rootFlag = findRoot(p)
	}


	mergeList, err := getMergeList(p)
	if err != nil {
		return map[string]any{}, []Include{}, err
	}

	final := make(map[string]any)
	mergeListObjects := []map[string]any{}
	for i := 0 ; i < len(mergeList); i = i + 1 {
		current := make(map[string]any)

		content, err := ioutil.ReadFile(mergeList[i].path)
		if err != nil {
			return map[string]any{}, []Include{}, err
		}

		err = yamljson.Unmarshal(content, &current)
		if err != nil {
			logErr.Println("cannot unmarshal data when merging",
				p,
				". Error is in",
				mergeList[i].path)
			return map[string]any{}, []Include{}, err
		}

		logDebug.Println("(mergelist) append", mergeList[i])
		mergeListObjects = append(mergeListObjects, current)
	}


	for _, current := range mergeListObjects {
		// Initialization using default overwrite
		for k,v := range current {
			final[k] = v
		}
	}

	logDebug.Println(mergeListObjects)
	merged := make(map[string]any)
	for _, current := range mergeListObjects {
		// Iterate over all the custom merge strategies and apply them in order
		for _, mergeStrategy := range mergeStrategies {
			if err := customStrategyMerge(merged, current, mergeStrategy); err != nil {
				logErr.Println(
					"Error in custom strategy when merging",
					p,
					"with strategy",
					mergeStrategy,
				)
				return map[string]any{}, []Include{}, err
			}
		}
	}

	// Override final with merged vars
	for k,v := range merged {
		final[k] = v
	}

	return final, mergeList, nil
}

func initMergeStrategies() {
	mergeStrategies = []MergeStrategy{
		{
			Path: "/__meta__",
			Strategy: "strategic-merge",
		},
		{
			Path: "/agnosticv_meta",
			Strategy: "strategic-merge",
		},
	}

	if len(schemas) == 0 {
		initSchemaList()
	}

	logDebug.Println("(INIT parse merge strategies) ")
	for _, schema := range(schemas) {
		mergeStrategies = append(mergeStrategies, schema.schema.XMerge...)
		logDebug.Println("(INIT parse merge strategies) added", schema.schema.XMerge)
	}
	logDebug.Println("(INIT merge strategies) ", mergeStrategies)
}

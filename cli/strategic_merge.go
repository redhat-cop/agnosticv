package main

import (
	"fmt"
	"reflect"

	"github.com/imdario/mergo"
)


func strategicCleanupSlice(elems []any) ([]any, error) {
	result := []any{}
	done := map[string]int{}

	for index, elem := range elems {
		if reflect.TypeOf(elem).Kind() != reflect.Map {
			// strategic merge works only on map, if it's not a map, just add the elem and continue
			result = append(result, elem)
			continue
		}

		elemMap := elem.(map[string]any)


		if name, ok := elemMap["name"]; ok {
			if reflect.TypeOf(elemMap["name"]).Kind() != reflect.String {
				return result, fmt.Errorf("strategic merge cannot work if 'name' is not a string")
			}
			nameStr := name.(string)
			if doneIndex, ok := done[nameStr]; ok {
				// An element with the same name exists, replace that element
				result1 := append(result[:doneIndex], elem)
				result = append(result1, result[doneIndex+1:]...)
				continue
			}
			// Append element
			result = append(result, elem)
			done[nameStr] = index
			continue
		}
		result = append(result, elem)
	}

	return result, nil
}
func strategicCleanupMap(m map[string]any) error {
	for key, v := range m {
		if v == nil {
			continue
		}
		if reflect.TypeOf(v).Kind() == reflect.Map {
			vMap := v.(map[string]any)

			if err := strategicCleanupMap(vMap); err != nil {
				return err
			}
			continue
		}

		if reflect.TypeOf(v).Kind() == reflect.Slice {
			vSlice := v.([]any)
			res, err := strategicCleanupSlice(vSlice)
			if err != nil {
				return err
			}

			m[key] = res
			continue
		}
	}

	return nil
}

func strategicMerge(dst map[string]any, src map[string]any) error {
	if err := mergo.Merge(
		&dst,
		src,
		mergo.WithOverride,
		mergo.WithOverwriteWithEmptyValue,
		mergo.WithAppendSlice,
	); err != nil {
		return err
	}

	if err := strategicCleanupMap(dst); err != nil {
		return err
	}
	return nil
}

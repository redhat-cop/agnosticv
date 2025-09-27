package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Include represent the include file
type Include struct {
	path string
	// options []Option
}

// Insert represent the insert file
type Insert struct {
	path string
}

// ErrorIncludeLoop happens in case of an infinite loop between included files
var ErrorIncludeLoop = errors.New("include loop")

// ErrorEmptyPath happens when a path is an empty string
var ErrorEmptyPath = errors.New("empty path")

// ErrorIncludeOutOfChroot happens when an include is not in the inside the agnosticV repo.
var ErrorIncludeOutOfChroot = errors.New("include path is out of chroot")

func containsPath(l []Include, p string) bool {
	for _, a := range l {
		if a.path == p {
			return true
		}
	}
	return false
}

// getMetaPath builds the path of the meta by prepending '.meta' to
// the original extension of the included file.
//
// dev.yaml => dev.meta.yaml
// dev.yml => dev.meta.yml
func getMetaPath(path string) (string, error) {
	if path == "" {
		return "", ErrorEmptyPath
	}

	extension := filepath.Ext(path)
	meta := strings.TrimSuffix(path, extension) + ".meta"

	// Detect which extension to use based on file existence
	if fileExists(meta + ".yml") {
		return meta + ".yml", nil
	}

	if fileExists(meta + ".yaml") {
		return meta + ".yaml", nil
	}

	// Return same extension as file
	return meta + extension, nil
}

func isMetaPath(path string) bool {
	ext := filepath.Ext(path)
	if ext == ".yml" || ext == ".yaml" {
		if filepath.Ext(strings.TrimSuffix(path, ext)) == ".meta" {
			return true
		}
	}

	return false
}

// function getMergeList return the merge list for a catalog items
// merge list contains: common files and includes.
func getMergeList(path string) ([]Include, error) {
	result := []Include{}
	done := map[string]bool{}
	for previous, next := "", path; next != "" && next != previous; next = nextCommonFile(next) {
		allIncludes, innerDone, err := parseAllIncludes(next, done)
		done = innerDone
		if err != nil {
			logErr.Println("Error loading includes for", next)
			return result, err
		}
		result = append([]Include{{path: next}}, result...)
		result = append(allIncludes, result...)
		previous = next
	}

	return result, nil
}

func printPaths(mergeList []Include, workdir string) {
	if len(mergeList) > 0 {
		fmt.Println("# MERGED:")
	}
	for i := 0; i < len(mergeList); i = i + 1 {
		if relativePath, err := filepath.Rel(workdir, mergeList[i].path); err == nil && len(relativePath) < len(mergeList[i].path) {
			fmt.Printf("#   %s\n", relativePath)
		} else {
			fmt.Printf("#   %s\n", mergeList[i].path)
		}
	}
}

var regexInclude = regexp.MustCompile(`^[ \t]*#include[ \t]+("(.*?[^\\])"|([^ \t]+))[ \t]*$`)
var regexInsert = regexp.MustCompile(`^[ \t]*#insert[ \t]+("(.*?[^\\])"|([^ \t]+))[ \t]*$`)

// parseInclude function parses the includes in a line
func parseInclude(line string) (bool, Include) {
	result := regexInclude.FindAllStringSubmatch(line, -1)

	if len(result) == 0 {
		return false, Include{}
	}

	if len(result) > 1 {
		logErr.Println("Could not parse include line:", line)
		return false, Include{}
	}

	if len(result[0]) < 4 {
		logErr.Println("Could not parse include line:", line)
		return false, Include{}
	}

	if result[0][2] == "" {
		if result[0][3] == "" {
			return false, Include{}
		}
		return true, Include{
			path: result[0][3],
		}
	}

	return true, Include{
		path: result[0][2],
	}
}

// parseInsert function parses the inserts in a line
func parseInsert(line string) (bool, Insert) {
	result := regexInsert.FindAllStringSubmatch(line, -1)

	if len(result) == 0 {
		return false, Insert{}
	}

	if len(result) > 1 {
		logErr.Println("Could not parse insert line:", line)
		return false, Insert{}
	}

	if len(result[0]) < 4 {
		logErr.Println("Could not parse insert line:", line)
		return false, Insert{}
	}

	var path string

	if result[0][2] == "" {
		if result[0][3] == "" {
			return false, Insert{}
		}
		path = result[0][3]
	} else {
		path = result[0][2]
	}

	return true, Insert{
		path: path,
	}
}

// parseAllInserts parses all inserts in a file
func parseAllInserts(path string, done map[string]bool) ([]Insert, map[string]bool, error) {
	logDebug.Println("parseAllInserts(", path, done, ")")
	if !fileExists(path) {
		logErr.Println(path, "path does not exist")
		return []Insert{}, done, errors.New("path insert does not exist")
	}

	if val, ok := done[path]; ok && val {
		logErr.Println(path, "is inserted more than once")
		return []Insert{}, done, ErrorIncludeLoop
	}

	done[path] = true

	result := []Insert{}

	file, err := os.Open(path)
	if err != nil {
		return []Insert{}, done, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if ok, insert := parseInsert(line); ok {
			logDebug.Println("parseInsert(", line, ")")
			insert.path, err = resolvePath(rootFlag, insert.path, path)
			if err != nil {
				return []Insert{}, done, err
			}

			// For inserts, we don't recursively parse - just add the insert
			result = append(result, insert)
		}
	}
	return result, done, nil
}

// parseAllIncludes parses all includes in a file
func parseAllIncludes(path string, done map[string]bool) ([]Include, map[string]bool, error) {
	logDebug.Println("parseAllIncludes(", path, done, ")")
	if !fileExists(path) {
		logErr.Println(path, "path does not exist")
		return []Include{}, done, errors.New("path include does not exist")
	}

	if val, ok := done[path]; ok && val {
		logErr.Println(path, "is included more than once")
		return []Include{}, done, ErrorIncludeLoop
	}

	done[path] = true

	result := []Include{}

	// Check if path has a meta file
	if meta, err := getMetaPath(path); err == nil && fileExists(meta) {
		innerIncludes, innerDone, err := parseAllIncludes(meta, done)
		done = innerDone
		if err != nil {
			return []Include{}, done, err
		}
		innerIncludes = append(innerIncludes, Include{path: meta})
		result = append(innerIncludes, result...)
	}

	file, err := os.Open(path)
	if err != nil {
		return []Include{}, done, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if ok, include := parseInclude(line); ok {
			logDebug.Println("parseInclude(", line, ")")
			include.path, err = resolvePath(rootFlag, include.path, path)
			if err != nil {
				return []Include{}, done, err
			}

			var innerIncludes []Include
			var innerDone map[string]bool
			if isCatalogItem(rootFlag, include.path) {
				innerIncludes, err = getMergeList(include.path)
				// Remove last element, which is the current file
				innerIncludes = innerIncludes[:len(innerIncludes)-1]
				if err != nil {
					return []Include{}, done, err
				}
			} else {
				innerIncludes, innerDone, err = parseAllIncludes(include.path, done)
				done = innerDone
				if err != nil {
					return []Include{}, done, err
				}
			}

			innerIncludes = append(innerIncludes, include)
			result = append(result, innerIncludes...)
		}
	}
	return result, done, nil
}

// getAllInserts collects all insert directives from a file and its includes
func getAllInserts(path string, done map[string]bool) ([]Insert, map[string]bool, error) {
	logDebug.Println("getAllInserts(", path, done, ")")
	if !fileExists(path) {
		logErr.Println(path, "path does not exist")
		return []Insert{}, done, errors.New("path does not exist")
	}

	if val, ok := done[path]; ok && val {
		logErr.Println(path, "is processed more than once")
		return []Insert{}, done, ErrorIncludeLoop
	}

	done[path] = true

	result := []Insert{}

	file, err := os.Open(path)
	if err != nil {
		return []Insert{}, done, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if ok, insert := parseInsert(line); ok {
			logDebug.Println("parseInsert(", line, ")")
			insert.path, err = resolvePath(rootFlag, insert.path, path)
			if err != nil {
				return []Insert{}, done, err
			}

			// For inserts, we don't recursively parse - just add the insert
			result = append(result, insert)
		}

		// Also check for includes and recursively get their inserts
		if ok, include := parseInclude(line); ok {
			include.path, err = resolvePath(rootFlag, include.path, path)
			if err != nil {
				return []Insert{}, done, err
			}

			// Recursively get inserts from included files
			innerInserts, innerDone, err := getAllInserts(include.path, done)
			done = innerDone
			if err != nil {
				return []Insert{}, done, err
			}
			result = append(result, innerInserts...)
		}
	}
	return result, done, nil
}

// resolvePath return the absolute path, with context
func resolvePath(root string, includePath string, contextFile string) (string, error) {
	if includePath[0] == '/' {
		return filepath.Join(root, filepath.Clean(includePath)), nil
	}
	result := filepath.Join(path.Dir(contextFile), filepath.Clean(includePath))

	if !isRoot(root, result) {
		return "", ErrorIncludeOutOfChroot
	}
	return result, nil
}

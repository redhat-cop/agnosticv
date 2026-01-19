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
	path      string
	recursive bool // true = process #include in included file (default), false = don't process includes in included file
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
// Note: This function is called either:
//   1. From main code when user explicitly merges a file (should walk up parent common files)
//   2. From parseAllIncludes when including a catalog item with recursive=true
// The caller in parseAllIncludes already checks isCatalogItem before calling this.
func getMergeList(path string) ([]Include, error) {
	result := []Include{}
	done := map[string]bool{}

	for previous, next := "", path; next != "" && next != previous; next = nextCommonFile(next) {
		allIncludes, innerDone, err := parseAllIncludes(next, done, true)
		done = innerDone
		if err != nil {
			logErr.Println("Error loading includes for", next)
			return result, err
		}
		result = append([]Include{{path: next, recursive: true}}, result...)
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

// Regex to match #include directive
// Supports: #include /path, #include recursive=false /path, #include recursive=true /path
var regexInclude = regexp.MustCompile(`^[ \t]*#include(?:[ \t]+recursive=(true|false))?[ \t]+("(.*?[^\\])"|([^ \t]+))[ \t]*$`)

// parseInclude function parses the includes in a line
// Returns: (found, Include)
func parseInclude(line string) (bool, Include) {
	result := regexInclude.FindAllStringSubmatch(line, -1)

	if len(result) == 0 {
		return false, Include{}
	}

	if len(result) > 1 {
		logErr.Println("Could not parse include line:", line)
		return false, Include{}
	}

	if len(result[0]) < 5 {
		logErr.Println("Could not parse include line:", line)
		return false, Include{}
	}

	// Extract recursive parameter if present (default is true)
	recursive := true
	recursiveParam := result[0][1]
	if recursiveParam != "" {
		recursive = (recursiveParam == "true")
	}

	// Extract file path (either quoted or unquoted)
	var path string
	if result[0][3] != "" {
		// Quoted path
		path = result[0][3]
	} else if result[0][4] != "" {
		// Unquoted path
		path = result[0][4]
	} else {
		return false, Include{}
	}

	return true, Include{
		path:      path,
		recursive: recursive,
	}
}

// parseAllIncludes parses all includes in a file
// Returns: (includes, done, error)
// processIncludes: if true, process #include directives within this file
//
// Meta file behavior:
//   - Always includes the direct .meta file if it exists (e.g., file.yaml → file.meta.yaml)
//   - Meta files themselves don't get meta files (no meta.meta.yaml)
//   - processIncludes controls whether #include directives in the meta file are processed
//
// For catalog items/common files: processIncludes is always true
// For #include files: processIncludes matches the recursive parameter (true by default)
func parseAllIncludes(path string, done map[string]bool, processIncludes bool) ([]Include, map[string]bool, error) {
	logDebug.Println("parseAllIncludes(", path, done, "processIncludes=", processIncludes, ")")
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

	// Always check if path has a meta file (unless current file is already a meta file)
	// The meta file itself is always included, but processIncludes controls whether
	// we process #include directives within the meta file
	if !isMetaPath(path) {
		if meta, err := getMetaPath(path); err == nil && fileExists(meta) {
			innerIncludes, innerDone, err := parseAllIncludes(meta, done, processIncludes)
			done = innerDone
			if err != nil {
				return []Include{}, done, err
			}
			innerIncludes = append(innerIncludes, Include{path: meta, recursive: true})
			result = append(innerIncludes, result...)
		}
	}

	// If processIncludes is false, don't scan for #include directives
	if !processIncludes {
		return result, done, nil
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
			logDebug.Println("parseInclude(", line, ") recursive=", include.recursive)

			include.path, err = resolvePath(rootFlag, include.path, path)
			if err != nil {
				return []Include{}, done, err
			}

			var innerIncludes []Include
			var innerDone map[string]bool

			// With recursive=false: treat the file as a simple include (no parent common files)
			// With recursive=true: if it's a catalog item, include its full merge list (with common files)
			if !include.recursive || !isCatalogItem(rootFlag, include.path) {
				// recursive=false OR not a catalog item: just parse the file itself
				innerIncludes, innerDone, err = parseAllIncludes(include.path, done, include.recursive)
				done = innerDone
				if err != nil {
					return []Include{}, done, err
				}
			} else {
				// recursive=true AND catalog item: get full merge list including common files
				innerIncludes, err = getMergeList(include.path)
				// Remove last element, which is the current file
				innerIncludes = innerIncludes[:len(innerIncludes)-1]
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

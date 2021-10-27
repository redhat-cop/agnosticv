package main

import (
	"flag"
	"fmt"
	"github.com/imdario/mergo"
	"github.com/jmespath/go-jmespath"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"regexp"
	"errors"
	"bufio"
	yaml "gopkg.in/yaml.v2"
	yaml3 "gopkg.in/yaml.v3"
)

// Logs
var logErr *log.Logger
var logOut *log.Logger
var logDebug *log.Logger
var logReport *log.Logger

// Flags
type arrayFlags []string
var listFlag bool
var relatedFlags arrayFlags
var orRelatedFlags arrayFlags
var hasFlags arrayFlags
var mergeFlag string
var debugFlag bool
var rootFlag string

// Methods to be able to use the flag multiple times
func (i *arrayFlags) String() string {
    return fmt.Sprintf("%s", *i)
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// Global variables

var workDir string

func parseFlags() {
	flag.BoolVar(&listFlag, "list", false, "List all the catalog items present in current directory.")
	flag.Var(&relatedFlags, "related", `Use with --list only. Filter output and display only related catalog items.
A catalog item is related to FILE if:
- it includes FILE as a common file
- it includes FILE via #include
- FILE is description.adoc or description.html

Example:
--list --related dir/common.yaml --related includes/foo.yaml
   List all catalog items under dir/ that also include includes/foo.yaml

Can be used several times (act like AND).`)
	flag.Var(&orRelatedFlags, "or-related", `Use with --list only. Same as --related except it appends the related files to the list instead of reducing it.

Example:
--list --related dir/common.yaml --or-related includes/foo.yaml
   List all catalog items under dir/ and also all catalog items that include includes/foo.yaml

Can be used several times (act like OR).`)
	flag.Var(&hasFlags, "has", `Use with --list only. Filter catalog items using a JMESPath expression.
Can be used several times (act like AND).

Examples:
--has __meta__.catalog
--has "env_type == 'ocp-clientvm'"
--has "to_string(worker_instance_count) == '2'"
`)
	flag.BoolVar(&debugFlag, "debug", false, "Debug mode")
	flag.StringVar(&mergeFlag, "merge", "", "Merge and print variables of a catalog item.")
	flag.StringVar(&rootFlag, "root", "", `The top directory of the agnosticv files. Files outside of this directory will not be merged.
By default, it's empty, and the scope of the git repository is used, so you should not
need this parameter unless your files are not in a git repository, or if you want to use a subdir. Use -root flag with -merge.`)

	flag.Parse()

	if len(hasFlags) > 0 && listFlag == false {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if len(relatedFlags) > 0 && listFlag == false {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if len(orRelatedFlags) > 0 && listFlag == false {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if mergeFlag == "" && listFlag == false {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if mergeFlag != "" && listFlag == true {
		log.Fatal("You cannot use --merge and --list simultaneously.")
	}

	if rootFlag != "" {
		if ! fileExists(rootFlag) {
			log.Fatalf("File %s does not exist", rootFlag)
		}

		if rootAbs, err := filepath.Abs(rootFlag) ; err == nil {
			rootFlag = rootAbs
		}
	}
}

func initLoggers() {
	logErr = log.New(os.Stderr, "!!! ", log.LstdFlags)
	logOut = log.New(os.Stdout, "    ", log.LstdFlags)
	if debugFlag {
		logDebug = log.New(os.Stdout, "(d) ", log.LstdFlags)
	} else {
		logDebug = log.New(ioutil.Discard, "(d) ", log.LstdFlags)
	}
	logReport = log.New(os.Stdout, "+++ ", log.LstdFlags)
}


// isPathCatalogItem checks if p is a catalog item by looking at its path.
// returns true or false
// root = the root directory of the agnosticV repo.
func isPathCatalogItem(root, p string) bool {

	if !chrooted(root, p) {
		return false
	}


	// Ignore all catalog items that are in a directory starting with a "."
	// or are dotfiles.

	for _, file := range strings.Split(p[len(root)+1:], string(os.PathSeparator)) {
		// pass special dirs
		if file == "." || file == ".." {
			continue
		}

		// Ignore includes directories or file
		if file == "includes" {
			return false
		}

		// Ignore dotfiles
		if strings.HasPrefix(file, ".") {
			return false
		}
	}

	switch path.Base(p) {
	case "common.yml", "common.yaml", "account.yml", "account.yaml":
		return false
	}

	// Catalog items are yaml files only.
	if !strings.HasSuffix(p, ".yml") && !strings.HasSuffix(p, ".yaml") {
		return false
	}

	return true
}

var regexNotCatalogItem = regexp.MustCompile(`^#[ \t]*agnosticv[ \t]+catalog_item[ \t]+false[ \t]*$`)

// isCatalogItem checks if a path is a valid catalog item.
// root is the root directory of the local agnosticV repo.
// returns true|false
func isCatalogItem(root, p string) bool {
	if !isPathCatalogItem(root, p) {
		return false
	}

	if !fileExists(p) {
		return false
	}
	file, err := os.Open(p)
	defer file.Close()

	if err != nil {
		logErr.Printf("%v\n", err)
		return false
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if regexNotCatalogItem.MatchString(line) {
			return false
		}

	}

	return true
}

func findCatalogItems(workdir string, hasFlags []string, relatedFlags []string, orRelatedFlags []string) ([]string, error) {
	logDebug.Println("findCatalogItems(", workdir, hasFlags, ")")
	result := []string{}
	os.Chdir(workdir)
	if rootFlag == "" {
		rootFlag = findRoot(workdir)
	}
	err := filepath.Walk(".", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			logErr.Printf("%q: %v\n", p, err)
			return err
		}

		// Ignore .dotfiles (.git, .travis.yml, ...)
		if strings.HasPrefix(info.Name(), ".") || strings.HasPrefix(p, ".") {
			return nil
		}

		pAbs, err := filepath.Abs(p)
		if err == nil {
			if !isCatalogItem(rootFlag, pAbs) {
				return nil
			}
		} else {
			logErr.Printf("%v\n", err)
			return nil
		}

		if len(relatedFlags) > 0 || len(orRelatedFlags) > 0 {
			mergeList, err := getMergeList(pAbs)

			// Related == merge list + description.{adoc,html}
			related := append(
				mergeList,
				Include{path: filepath.Join(filepath.Dir(pAbs),"description.adoc")},
				Include{path: filepath.Join(filepath.Dir(pAbs),"description.html")},
			)

			logDebug.Println("getMergeList(", pAbs, ") =", mergeList)
			logDebug.Println("related =", related)
			if err != nil {
				logErr.Printf("%v\n", err)
				return nil
			}

			// related files, inclusive version
			if len(orRelatedFlags) > 0 {
				for _, orRelatedFlag := range orRelatedFlags {

					orRelatedAbs, err := filepath.Abs(orRelatedFlag)
					if err != nil {
						logErr.Printf("%v\n", err)
						return nil
					}

					if containsPath(related, orRelatedAbs) {
						// Add catalog item to result
						result = append(result, p)
						return nil
					}
				}
			}

			// related files, exclusive version
			if len(relatedFlags) > 0 {
				for _, relatedFlag := range relatedFlags {

					relatedAbs, err := filepath.Abs(relatedFlag)
					if err != nil {
						logErr.Printf("%v\n", err)
						return nil
					}

					if !containsPath(related, relatedAbs) {
						// If not related, do not select catalog item
						return nil
					}

				}
			}
		}

		if len(hasFlags) > 0 {
			logDebug.Println("hasFlags", hasFlags)
			// Here we need yaml.v3 in order to use jmespath
			merged, _, err := mergeVars(p, "v3")
			if err != nil {
				// Print the error and move to next file
				logErr.Println(err)
				return nil
			}

			for _, hasFlag := range hasFlags {
				r, err := jmespath.Search(hasFlag, merged)
				if err != nil {
					logErr.Printf("ERROR: JMESPath '%q' not correct, %v", hasFlag, err)
					return err
				}

				logDebug.Printf("merged=%#v\n", merged)
				logDebug.Printf("r=%#v\n", r)

				// If JMESPath expression does not match, skip file
				if r == nil || r == false {
					return nil
				}
			}
		}

		result = append(result, p)
		return nil
	})

	return result, err
}

func fileExists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		logErr.Printf("%v\n", err)
		os.Exit(2)
	}
	return false
}

// This function works with both Relative and Absolute path
func parentDir(path string) string {
	logDebug.Println("parentDir(", path, ")")
	fileinfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return filepath.Dir(path)
		}
		logErr.Println("Error with stat")
		logErr.Fatal(err)
	}

	var currentDir string

	// Determine current Directory
	if fileinfo.IsDir() {
		currentDir = filepath.Clean(path)
	} else {
		currentDir = filepath.Clean(filepath.Dir(path))
	}

	return filepath.Dir(currentDir)
}

// chrooted function compares strings and returns true if
// path is chrooted in root.
// It's a poor man's chroot
func chrooted(root string, path string) bool {
	if root == path {
		return true
	}
	suffix := ""
	if !strings.HasSuffix(root, "/") {
		suffix = "/"
	}
	return strings.HasPrefix(path, root + suffix)
}

func findRoot(item string) string {
	if itemAbs, err := filepath.Abs(item) ; err == nil {
		item = itemAbs
	}

	if item == "/" {
		log.Fatal("Root not found.")
	}

	fileinfo, err := os.Stat(item)

	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal(item, "File does not exist.")
		}

		log.Fatal(item, err.Error())
	}

	// If it's a dir, run with current directory
	if fileinfo.IsDir() {
		if fileExists(filepath.Join(item, ".git")) {
			// .git dir exists, root found.
			return item
		}
	}

	return findRoot(parentDir(item))
}

// This function return the next file to be included in the merge.
// it returns the empty string "" if not found.
// pos can be a directory or a file
func nextCommonFile(position string) string {
	logDebug.Println("nextCommonFile position:", position)
	validCommonFileNames := []string{
		"common.yaml",
		"common.yml",
		"account.yaml",
		"account.yml",
	}

	// If position is a common file, try with parent dir
	for _, commonFile := range validCommonFileNames {
		if path.Base(position) == commonFile {
			// If parent is out of chroot, stop
			if !chrooted(rootFlag, parentDir(position)) {
				logDebug.Println("parent of", position, ",", parentDir(position),
					"is out of chroot", rootFlag)
				return ""
			}

			return nextCommonFile(parentDir(position))
		}
	}

	fileinfo, err := os.Stat(position)

	if os.IsNotExist(err) {
		logErr.Fatal(position, "File does not exist.")
	}

	// If it's a file, run with current directory
	if !fileinfo.IsDir() {
		return nextCommonFile(filepath.Dir(position))
	}

	for _, commonFile := range validCommonFileNames {
		candidate := filepath.Join(position, commonFile)
		if fileExists(candidate) {
			logDebug.Println("nextCommonFile found:", candidate)
			return candidate
		}
	}

	if position == "/" { return "" }

	// If parent is out of chroot, stop
	if !chrooted(rootFlag, parentDir(position)) {
		logDebug.Println("parent of", position, ",", parentDir(position),
			"is out of chroot", rootFlag)
		return ""
	}

	return nextCommonFile(parentDir(position))

}

func containsPath(l []Include, p string) bool {
    for _, a := range l {
        if a.path == p {
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
		result = append([]Include{Include{path: next}}, result...)
		result = append(allIncludes, result...)
		previous = next
	}

	return result, nil
}

func printPaths(mergeList []Include) {
	if len(mergeList) > 0 {
		fmt.Println("# MERGED:")
	}
	for i := 0; i < len(mergeList); i = i + 1 {
		if relativePath, err := filepath.Rel(workDir, mergeList[i].path) ; err == nil && len(relativePath) < len(mergeList[i].path) {
			fmt.Printf("# %s\n", relativePath)
		} else {
			fmt.Printf("# %s\n", mergeList[i].path)
		}
	}
}

var regexInclude = regexp.MustCompile(`^[ \t]*#include[ \t]+("(.*?[^\\])"|([^ \t]+))[ \t]*$`)

// Include represent the include file
type Include struct {
	path string
	// options []Option
}

// ErrorIncludeLoop happens in case of an infinite loop between included files
var ErrorIncludeLoop = errors.New("include loop")

// ErrorIncludeOutOfChroot happens when an include is not in the inside the agnosticV repo.
var ErrorIncludeOutOfChroot = errors.New("include path is out of chroot")

// resolvePath return the absolute path, with context
func resolvePath(root string, includePath string, contextFile string) (string, error) {
	if includePath[0] == '/' {
		return filepath.Join(root, filepath.Clean(includePath)), nil
	}
	result := filepath.Join(path.Dir(contextFile), filepath.Clean(includePath))

	if !chrooted(root, result) {
		return "", ErrorIncludeOutOfChroot
	}
	return result, nil
}

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

// parseAllIncludes parses all includes in a file
func parseAllIncludes(path string, done map[string]bool) ([]Include, map[string]bool, error) {
	logDebug.Println("parseAllIncludes(", path, done, ")")
	if !fileExists(path) {
		logErr.Println(path, "path does not exist")
		return []Include{}, done, errors.New("path include does not exist")
	}

	if val, ok := done[path]; ok && val {
		logErr.Println(path, "include loop detected")
		return []Include{}, done, ErrorIncludeLoop
	}

	done[path] = true

	result := []Include{}

	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return []Include{}, done, err
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if ok, include := parseInclude(line); ok {
			logDebug.Println("parseInclude(", line, ")")
			include.path, err = resolvePath(rootFlag, include.path, path)
			if err != nil {
				return []Include{}, done, err
			}
			innerIncludes, innerDone, err := parseAllIncludes(include.path, done)
			done = innerDone

			if err != nil {
				return []Include{}, done, err
			}

			innerIncludes = append(innerIncludes, include)
			result = append(innerIncludes, result...)
		}
	}
	return result, done, nil
}

func mergeVars(p string, version string) (map[string]interface{}, []Include, error) {
	// Work with Absolute paths
	if ! filepath.IsAbs(p) {
		if abs, errAbs := filepath.Abs(p); errAbs == nil {
			p = abs
		} else {
			return map[string]interface{}{}, []Include{}, errAbs
		}
	}

	if rootFlag == "" {
		rootFlag = findRoot(p)
	}

	mergeList, err := getMergeList(p)
	if err != nil {
		return map[string]interface{}{}, []Include{}, err
	}

	logDebug.Printf("%+v\n", mergeList)

	final := make(map[string]interface{})
	meta := make(map[string]interface{})

	for i := 0 ; i < len(mergeList); i = i + 1 {
		current := make(map[string]interface{})

		logDebug.Println("reading", mergeList[i])
		content, err := ioutil.ReadFile(mergeList[i].path)
		if err != nil {
			return map[string]interface{}{}, []Include{}, err
		}

		switch version {
		case "v2":
			err = yaml.Unmarshal(content, &current)
		case "v3":
			err = yaml3.Unmarshal(content, &current)
		}
		logDebug.Println("len(current)", len(current))

		if err != nil {
			logErr.Println("cannot unmarshal data when merging",
				p,
				". Error is in",
				mergeList[i].path)
			return map[string]interface{}{}, []Include{}, err
		}

		for k,v := range current {
			final[k] = v
		}

		if err = mergo.Merge(
			&meta,
			current,
			mergo.WithOverride,
			mergo.WithOverwriteWithEmptyValue,
			mergo.WithAppendSlice,
		); err != nil {
			logErr.Println("Error in mergo.Merge() when merging", p)
			return map[string]interface{}{}, []Include{}, err
		}
		logDebug.Println("len(meta)", len(meta))
		logDebug.Println("len(final)", len(final))
	}

	if val, ok := meta["__meta__"]; ok {
		final["__meta__"] = val
	}

	if val, ok := meta["agnosticv_meta"]; ok {
		final["agnosticv_meta"] = val
	}

	return final, mergeList, nil
}

func main() {

	parseFlags()
	initLoggers()

	// Save current work directory
	if wd, errWorkDir := os.Getwd() ; errWorkDir == nil {
		workDir = wd
	} else {
		logErr.Fatal(errWorkDir)
	}

	if listFlag {
		// always determine the chroot
		if rootFlag == "" {
			rootFlag = findRoot(workDir)
		}
		catalogItems, err := findCatalogItems(workDir, hasFlags, relatedFlags, orRelatedFlags)

		if err != nil {
			logErr.Printf("error walking the path %q: %v\n", ".", err)
			return
		}
		for _, ci := range catalogItems {
			fmt.Println(ci)
		}
	}

	if mergeFlag != "" {
		// always determine the chroot
		if rootFlag == "" {
			rootFlag = findRoot(mergeFlag)
		}

		merged, mergeList, err := mergeVars(mergeFlag, "v2")
		if err != nil {
			logErr.Fatal(err)
		}
		out, _:= yaml.Marshal(merged)

		fmt.Printf("---\n")
		printPaths(mergeList)
		fmt.Printf("%s", out)
	}
}

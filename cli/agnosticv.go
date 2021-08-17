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
	flag.Var(&hasFlags, "has", `Use with --list only. Filter catalog items using a JMESPath expression.
Can be used several time (act like AND).

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

	if mergeFlag == "" && listFlag == false {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if mergeFlag != "" && listFlag == true {
		log.Fatal("You cannot use --merge and --list simultaneously.")
	}

	if rootFlag != "" {
		if listFlag {
			log.Fatal("You cannot use --root with --list, list is relative to current directory.")
		}

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

func findCatalogItems(workdir string, hasFlags []string) ([]string, error) {
	result := []string{}
	os.Chdir(workdir)
	err := filepath.Walk(".", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			logErr.Printf("%q: %v\n", p, err)
			return err
		}

		// Ignore .dotfiles (.git, .travis.yml, ...)
		if strings.HasPrefix(info.Name(), ".") || strings.HasPrefix(p, ".") {
			return nil
		}

		switch info.Name() {
		case "common.yml", "common.yaml", "account.yml", "account.yaml":
			return nil
		}
		switch path.Ext(info.Name()) {
		case ".yml", ".yaml":
			if len(hasFlags) > 0 {
				logDebug.Println("hasFlags", hasFlags)
				// Here we need yaml.v3 in order to use jmespath
				err, merged, _ := mergeVars(p, "v3")
				if err != nil {
					// Print the error and move to next file
					logErr.Println(err)
					return nil
				}

				for _, hasFlag := range hasFlags {
					result, err := jmespath.Search(hasFlag, merged)
					if err != nil {
						logErr.Printf("ERROR: JMESPath '%q' not correct, %v", hasFlag, err)
						return err
					}

					logDebug.Printf("merged=%#v\n", merged)
					logDebug.Printf("result=%#v\n", result)

					// If JMESPath expression does not match, skip file
					if result == nil || result == false {
						return nil
					}
				}
			}
			result = append(result, p)
		}

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
		fmt.Println("Error with stat")
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

func chrooted(root string, path string) bool {
	return strings.HasPrefix(path, root)
}

func findRoot(item string) string {
	if itemAbs, err := filepath.Abs(item) ; err == nil {
		item = itemAbs
	}

	if item == "/" {
		log.Fatal("Root not found.")
	}

	fileinfo, err := os.Stat(item)

	if os.IsNotExist(err) {
		log.Fatal(item, "File does not exist.")
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

// This function prints the variables for the catalog item passed as parameter.
func getMergeList(path string) []string {
	result := []string{path}
	for previous, next := "", nextCommonFile(path); next != "" && next != previous; next = nextCommonFile(next) {
		result = append(result, next)
		previous = next
	}

	return result
}

func printPaths(mergeList []string) {
	if len(mergeList) > 0 {
		fmt.Println("# MERGED:")
	}
	for i := len(mergeList) - 1; i >= 0; i = i -1 {
		if relativePath, err := filepath.Rel(workDir, mergeList[i]) ; err == nil && len(relativePath) < len(mergeList[i]) {
			fmt.Printf("# %s\n", relativePath)
		} else {
			fmt.Printf("# %s\n", mergeList[i])
		}
	}
}

func mergeVars(p string, version string) (error, map[string]interface{}, []string) {
	// Work with Absolute paths
	if ! filepath.IsAbs(p) {
		if abs, errAbs := filepath.Abs(p); errAbs == nil {
			p = abs
		} else {
			return errAbs, map[string]interface{}{}, []string{}
		}
	}

	mergeList := getMergeList(p)
	logDebug.Printf("%+v\n", mergeList)

	final := make(map[string]interface{})
	meta := make(map[string]interface{})

	for i := len(mergeList) - 1; i >= 0; i = i -1 {
		current := make(map[string]interface{})

		logDebug.Println("reading", mergeList[i])
		content, err := ioutil.ReadFile(mergeList[i])
		if err != nil {
			return err, map[string]interface{}{}, []string{}
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
				mergeList[i])
			return err, map[string]interface{}{}, []string{}
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
			return err, map[string]interface{}{}, []string{}
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

	return nil, final, mergeList
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
		catalogItems, err := findCatalogItems(workDir, hasFlags)

		if err != nil {
			logErr.Printf("error walking the path %q: %v\n", ".", err)
			return
		}
		for _, ci := range catalogItems {
			fmt.Println(ci)
		}
	}

	if mergeFlag != "" {

		// For merge, always determine the chroot
		if rootFlag == "" {
			rootFlag = findRoot(mergeFlag)
		}

		err, merged, mergeList := mergeVars(mergeFlag, "v2")
		if err != nil {
			logErr.Fatal(err)
		}
		out, _:= yaml.Marshal(merged)

		fmt.Printf("---\n")
		printPaths(mergeList)
		fmt.Printf("%s", out)
	}
}

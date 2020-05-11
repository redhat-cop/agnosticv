package main
import (
	"fmt"
	"os"
	"log"
	"io/ioutil"
	"flag"
	"path"
	"path/filepath"
	"gopkg.in/yaml.v2"
	"github.com/imdario/mergo"
)


// Logs
var logErr *log.Logger
var logOut *log.Logger
var logDebug *log.Logger
var logReport *log.Logger

// Flags
var listFlag bool
var mergeFlag string
var debugFlag bool

// Global variables

var workDir string

func parseFlags() {
	flag.BoolVar(&listFlag, "list", false, "List all the catalog items present in current directory.")
	flag.BoolVar(&debugFlag, "debug", false, "Debug mode")
	flag.StringVar(&mergeFlag, "merge", "", "Merge and print variables of a catalog item.")

	flag.Parse()
	if mergeFlag == "" && listFlag == false {
		flag.PrintDefaults()
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

// walkList is the walkFunc that will print all the catalog items
func walkList(p string, info os.FileInfo, err error) error {
	if err != nil {
		logErr.Printf("%q: %v\n", p, err)
		return err
	}

	switch info.Name() {
	case "common.yml", "common.yaml", "account.yml", "account.yaml":
		return nil
	}
	switch path.Ext(info.Name()) {
	case ".yml", ".yaml":
		fmt.Printf("%s\n", p)
	}
	return nil
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
		} else {
			fmt.Println("Error with stat")
			logErr.Fatal(err)
		}
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
			// If it's already the root of the git directory, then stop.
			if fileExists(filepath.Join(filepath.Dir(position), ".git")) {
				// No common file found in current repo
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

	if fileExists(filepath.Join(filepath.Dir(position), ".git")) {
		// No common file found in current repo
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

func mergeVars(mergeList []string) interface{} {
	final := make(map[string]interface{})
	meta := make(map[string]interface{})

	for i := len(mergeList) - 1; i >= 0; i = i -1 {
		current := make(map[string]interface{})

		logDebug.Println("reading", mergeList[i])
		content, err := ioutil.ReadFile(mergeList[i])
		if err != nil {
			logErr.Fatal(err)
		}

		err = yaml.Unmarshal(content, &current)
		logDebug.Println("len(current)", len(current))

		if err != nil {
			logErr.Fatalf("cannot unmarshal data: %v", err)
		}

		for k,v := range current {
			final[k] = v
		}

		if err = mergo.Merge(&meta, current, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue, mergo.WithAppendSlice ); err != nil {
			logErr.Fatal(err)
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

	return final
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
		err := filepath.Walk(".", walkList)

		if err != nil {
			logErr.Printf("error walking the path %q: %v\n", ".", err)
			return
		}
	}

	if mergeFlag != "" {
		// Work with Absolute paths
		if ! filepath.IsAbs(mergeFlag) {
			if abs, errAbs := filepath.Abs(mergeFlag); errAbs == nil {
				mergeFlag = abs
			} else {
				logErr.Fatal(errAbs)
			}
		}

		mergeList := getMergeList(mergeFlag)
		logDebug.Printf("%+v\n", mergeList)

		merged := mergeVars(mergeList)
		out, _:= yaml.Marshal(merged)

		fmt.Printf("---\n")
		printPaths(mergeList)
		fmt.Printf("%s", out)
	}
}

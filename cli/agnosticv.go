package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jmespath/go-jmespath"
	yaml "gopkg.in/yaml.v2"
)

// Logs
var logErr *log.Logger
var logDebug *log.Logger

// Flags
type arrayFlags []string
var listFlag bool
var relatedFlags arrayFlags
var orRelatedFlags arrayFlags
var hasFlags arrayFlags
var mergeFlag string
var debugFlag bool
var rootFlag string
var validateFlag bool
var versionFlag bool
var gitFlag bool

// Build info
var Version = "development"
var buildTime = "undefined"
var buildCommit = "HEAD"

// Methods to be able to use the flag multiple times
func (i *arrayFlags) String() string {
    return fmt.Sprintf("%s", *i)
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// Global variables

var mergeStrategies []MergeStrategy

func parseFlags() {
	flag.BoolVar(&listFlag, "list", false, "List all the catalog items present in current directory.")
	flag.BoolVar(&validateFlag, "validate", true, "Validate variables against schemas present in .schemas directory.")
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
	flag.BoolVar(&versionFlag, "version", false, "Print build version.")
	flag.BoolVar(&gitFlag, "git", true, "Perform git operations to gather and inject information into the merged vars like 'last_update'. Git operations are slow so this option is automatically disabled for listing.")

	flag.Parse()

	if versionFlag {
		fmt.Println("Version:", Version)
		fmt.Println("Build time:", buildTime)
		fmt.Println("Build commit:", buildCommit)
		os.Exit(0)
	}

	if len(hasFlags) > 0 && !listFlag {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if len(relatedFlags) > 0 && !listFlag {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if len(orRelatedFlags) > 0 && !listFlag {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if mergeFlag == "" && !listFlag {
		flag.PrintDefaults()
		os.Exit(2)
	}

	if mergeFlag != "" && listFlag {
		log.Fatal("You cannot use --merge and --list simultaneously.")
	}

	if rootFlag != "" {
		if ! fileExists(rootFlag) {
			log.Fatalf("File %s does not exist", rootFlag)
		}

		rootFlag = abs(rootFlag)
	} else {
		// init rootFlag by discovering depending on other flags
		if listFlag {
			// use current workdir
			var workdir string
			if wd, errWorkDir := os.Getwd() ; errWorkDir == nil {
				workdir = wd
			} else {
				logErr.Fatal(errWorkDir)
			}

			rootFlag = findRoot(workdir)

		} else if mergeFlag != "" {
			// Use root of the file to merge
			rootFlag = findRoot(mergeFlag)
		}
	}

	// Do not perform git operations when listing
	if listFlag {
		gitFlag = false
	}

	if debugFlag {
		logDebug = log.New(os.Stdout, "(d) ", log.LstdFlags)
	}
}

func initLoggers() {
	logErr = log.New(os.Stderr, "!!! ", log.LstdFlags)
	logDebug = log.New(io.Discard, "(d) ", log.LstdFlags)
}


// isPathCatalogItem checks if p is a catalog item by looking at its path.
// returns true or false
// root = the root directory of the agnosticV repo.
func isPathCatalogItem(root, p string) bool {

	if root == p {
		return false
	}

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

	// Don't consider related file as catalog items
	if config.initialized {
		for _, el := range config.RelatedFiles {
			if path.Base(p) == el {
				return false
			}
		}
	}

	// Catalog items are yaml files only.
	if !strings.HasSuffix(p, ".yml") && !strings.HasSuffix(p, ".yaml") {
		return false
	}

	if isMetaPath(p) {
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

func extendMergeListWithRelated(pAbs string, mergeList []Include) []Include {
	// Related == merge list + description.{adoc,html} + config.related_files

	result := append(
		mergeList,
		Include{path: filepath.Join(filepath.Dir(pAbs),"description.adoc")},
		Include{path: filepath.Join(filepath.Dir(pAbs),"description.html")},
	)

	if config.initialized {
		for _, el := range config.RelatedFiles {
			result = append(
				result,
				Include{path: filepath.Join(filepath.Dir(pAbs), el)},
			)
		}
	}

	return result
}

func findCatalogItems(workdir string, hasFlags []string, relatedFlags []string, orRelatedFlags []string) ([]string, error) {
	logDebug.Println("findCatalogItems(", workdir, hasFlags, ")")
	result := []string{}
	if err := os.Chdir(workdir); err != nil {
		return result, err
	}
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

		// TODO: create a CatalogItem type that will use absolute path and make the validations isCatalogItem()
		pAbs, err := filepath.Abs(p)
		if err == nil {
			if !isCatalogItem(rootFlag, pAbs) {
				return nil
			}
		} else {
			logErr.Printf("%v\n", err)
			return nil
		}

		if len(hasFlags) > 0 {
			logDebug.Println("hasFlags", hasFlags)
			// Here we need yaml.v3 in order to use jmespath
			merged, _, err := mergeVars(p, mergeStrategies)
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

		if len(relatedFlags) > 0 || len(orRelatedFlags) > 0 {
			mergeList, err := getMergeList(pAbs)
			related := extendMergeListWithRelated(pAbs, mergeList)

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

func abs(item string) string {
	itemAbs, err := filepath.Abs(item)
	if err != nil {
		return item
	}
	return itemAbs
}
func findRoot(item string) string {
	item = abs(item)

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

func main() {
	initLoggers()
	parseFlags()
	initConf(rootFlag)
	initMergeStrategies()

	// Save current work directory
	var workDir string
	if wd, errWorkDir := os.Getwd() ; errWorkDir == nil {
		workDir = wd
	} else {
		logErr.Fatal(errWorkDir)
	}

	if len(schemas) == 0 {
		initSchemaList()
	}

	if listFlag {
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
		merged, mergeList, err := mergeVars(mergeFlag, mergeStrategies)
		if err != nil {
			logErr.Fatal(err)
		}

		if validateFlag {
			if err := validateAgainstSchemas(mergeFlag, merged); err != nil {
				logErr.Fatal(err)
			}
		}

		out, _:= yaml.Marshal(merged)

		fmt.Printf("---\n")
		printMergeStrategies()
		printPaths(mergeList, workDir)
		fmt.Printf("%s", out)
	}
}

package main

import (
	"bufio"
	"encoding/json"
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
var outputFlag string
var dirFlag string

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

type controlFlow struct {
	stop bool
	rc   int
}

func parseFlags(args []string, output io.Writer) controlFlow {
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&listFlag, "list", false, "List all the catalog items present in current directory.")
	flags.StringVar(&dirFlag, "dir", "", "Directory to use as dir when listing catalog items. Default = current directory.")
	flags.BoolVar(&validateFlag, "validate", true, "Validate variables against schemas present in .schemas directory.")
	flags.Var(&relatedFlags, "related", `Use with --list only. Filter output and display only related catalog items.
A catalog item is related to FILE if:
- it includes FILE as a common file
- it includes FILE via #include
- FILE is description.adoc or description.html

Example:
--list --related dir/common.yaml --related includes/foo.yaml
   List all catalog items under dir/ that also include includes/foo.yaml

Can be used several times (act like AND).`)
	flags.Var(&orRelatedFlags, "or-related", `Use with --list only. Same as --related except it appends the related files to the list instead of reducing it.

Example:
--list --related dir/common.yaml --or-related includes/foo.yaml
   List all catalog items under dir/ and also all catalog items that include includes/foo.yaml

Can be used several times (act like OR).`)
	const hasHelp ="Use with --list only. Filter catalog items using a JMESPath `expression`."+`
Can be used several times (act like AND).

Examples:
--has __meta__.catalog
# Compare a variable value
--has "env_type == 'ocp-clientvm'"
# Compare a variable numeric value
--has 'worker_instance_count == `+"`2`"+`'
# List all catalog items with a secret named 'gpte-sandbox'
--has 'length(__meta__.secrets[?name=='\''gpte-sandbox'\'']) > `+"`0`"+`'
`
	flags.Var(&hasFlags, "has", hasHelp)
	flags.BoolVar(&debugFlag, "debug", false, "Debug mode")
	flags.StringVar(&mergeFlag, "merge", "", "Merge and print variables of a catalog item.")
	flags.StringVar(&rootFlag, "root", "", `The top directory of the agnosticv files. Files outside of this directory will not be merged.
By default, it's empty, and the scope of the git repository is used, so you should not
need this parameter unless your files are not in a git repository, or if you want to use a subdir. Use -root flag with -merge.`)
	flags.BoolVar(&versionFlag, "version", false, "Print build version.")
	flags.BoolVar(&gitFlag, "git", true, "Perform git operations to gather and inject information into the merged vars like 'last_update'. Git operations are slow so this option is automatically disabled for listing.")
	flags.StringVar(&outputFlag, "output", "", "Output format. Possible values: json or yaml. Default is 'yaml' for merging.")

	if err := flags.Parse(args[1:]); err != nil {
		flags.PrintDefaults()
		return controlFlow{true, 2}
	}

	if versionFlag {
		fmt.Fprintln(output, "Version:", Version)
		fmt.Fprintln(output, "Build time:", buildTime)
		fmt.Fprintln(output, "Build commit:", buildCommit)
		return controlFlow{true, 0}
	}

	if len(hasFlags) > 0 && !listFlag {
		flags.PrintDefaults()
		return controlFlow{true, 2}
	}

	if len(relatedFlags) > 0 && !listFlag {
		flags.PrintDefaults()
		return controlFlow{true, 2}
	}

	if len(orRelatedFlags) > 0 && !listFlag {
		flags.PrintDefaults()
		return controlFlow{true, 2}
	}

	if mergeFlag == "" && !listFlag {
		flags.PrintDefaults()
		return controlFlow{true, 2}
	}

	if mergeFlag != "" && listFlag {
		fmt.Fprintln(output, "You cannot use --merge and --list simultaneously.")
		return controlFlow{true, 2}
	}

	if mergeFlag != "" && outputFlag == "" {
		// Set to YAML by default when merging
		outputFlag = "yaml"
	}

	if mergeFlag != "" && dirFlag != "" {
		fmt.Fprintln(output, "You cannot use --merge and --dir simultaneously.")
		return controlFlow{true, 2}
	}
	if dirFlag != "" {
		// Ensure dir is a directory
		fi, err := os.Stat(dirFlag)
		if err != nil {
			fmt.Fprintln(output, "Error:", err)
			return controlFlow{true, 1}
		}
		if !fi.IsDir() {
			fmt.Fprintln(output, "Error:", dirFlag, "is not a directory")
			return controlFlow{true, 2}
		}
		dirFlag, err = filepath.Abs(dirFlag)
		if err != nil {
			fmt.Fprintln(output, "Error:", err)
			return controlFlow{true, 1}
		}

		dirFlag = filepath.Clean(dirFlag) // line to clean dirFlag

	} else {
		// Default to current directory
		var err error
		if dirFlag, err = os.Getwd(); err != nil {
			fmt.Fprintln(output, "Error:", err)
			return controlFlow{true, 1}
		}
	}

	if rootFlag != "" {
		if !fileExists(rootFlag) {
			log.Fatalf("File %s does not exist", rootFlag)
		}

		rootFlag = abs(rootFlag)
	} else {
		// init rootFlag by discovering depending on other flags
		if listFlag {
			if dirFlag != "" {
				rootFlag = findRoot(dirFlag)
			} else {
				// use current workdir to find root
				var workdir string
				if wd, errWorkDir := os.Getwd(); errWorkDir == nil {
					workdir = wd
				} else {
					logErr.Fatal(errWorkDir)
				}
				rootFlag = findRoot(workdir)
			}

		} else if mergeFlag != "" {
			// Use root of the file to merge
			rootFlag = findRoot(mergeFlag)
		}
	}

	// Validate rootflag is compatible with other flags
	if listFlag {
		// Ensure listing will be done inside root
		absDir, err := filepath.Abs(dirFlag)
		if err != nil {
			fmt.Fprintln(output, "Error:", err)
			return controlFlow{true, 1}
		}

		if !isRoot(rootFlag, absDir) {
			fmt.Fprintln(output, "Error: --dir", dirFlag, "is not inside --root", rootFlag)
			return controlFlow{true, 2}
		}
	}

	// Do not perform git operations when listing
	if listFlag {
		gitFlag = false
	}

	if debugFlag {
		logDebug = log.New(os.Stdout, "(d) ", log.LstdFlags)
	}

	switch outputFlag {
	case "yaml", "json", "":
	default:
		fmt.Fprintln(output, "Unsupported format for output: ", outputFlag)
		return controlFlow{true, 2}
	}

	return controlFlow{false, 0}
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

	if !isRoot(root, p) {
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
		for _, el := range config.RelatedFilesV2 {
			if path.Base(p) == el.File {
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

	if err != nil {
		logErr.Printf("%v\n", err)
		return false
	}
	defer file.Close()

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
		Include{path: filepath.Join(filepath.Dir(pAbs), "description.adoc")},
		Include{path: filepath.Join(filepath.Dir(pAbs), "description.html")},
	)

	done := map[string]bool{}
	if config.initialized {
		for _, el := range config.RelatedFiles {
			if _, ok := done[el]; ok {
				continue
			}
			result = append(
				result,
				Include{path: filepath.Join(filepath.Dir(pAbs), el)},
			)
			done[el] = true
		}
		for _, el := range config.RelatedFilesV2 {
			if _, ok := done[el.File]; ok {
				continue
			}
			result = append(
				result,
				Include{path: filepath.Join(filepath.Dir(pAbs), el.File)},
			)
			done[el.File] = true
		}
	}

	return result
}

func findCatalogItems(workdir string, hasFlags []string, relatedFlags []string, orRelatedFlags []string) ([]string, error) {
	logDebug.Println("findCatalogItems(", workdir, hasFlags, ")")
	result := []string{}
	// save current dir
	prevDir, _ := os.Getwd()
	if err := os.Chdir(workdir); err != nil {
		return result, err
	}
	// Restore the current directory at the end of the function
	defer func() {
		if err := os.Chdir(prevDir); err != nil {
			logErr.Printf("%v\n", err)
		}
	}()

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

// isRoot function compares strings and returns true if
// path is contained in root.
// It's a poor man's chroot
func isRoot(root string, path string) bool {
	if root == path {
		return true
	}
	suffix := ""
	if !strings.HasSuffix(root, "/") {
		suffix = "/"
	}
	return strings.HasPrefix(path, root+suffix)
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
			if !isRoot(rootFlag, parentDir(position)) {
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

	if position == "/" {
		return ""
	}

	// If parent is out of chroot, stop
	if !isRoot(rootFlag, parentDir(position)) {
		logDebug.Println("parent of", position, ",", parentDir(position),
			"is out of chroot", rootFlag)
		return ""
	}

	return nextCommonFile(parentDir(position))

}

func main() {
	initLoggers()
	if flow := parseFlags(os.Args, os.Stdout); flow.stop {
		os.Exit(flow.rc)
	}
	initConf(rootFlag)
	initMergeStrategies()

	if len(schemas) == 0 {
		initSchemaList()
	}

	if listFlag {
		catalogItems, err := findCatalogItems(dirFlag, hasFlags, relatedFlags, orRelatedFlags)

		if err != nil {
			logErr.Printf("error walking the path %q: %v\n", ".", err)
			return
		}

		switch outputFlag {
		case "yaml":
			out, _ := yaml.Marshal(catalogItems)
			fmt.Printf("%s", out)

		case "json":
			out, _ := json.Marshal(catalogItems)
			fmt.Printf("%s", out)

		default:
			for _, ci := range catalogItems {
				fmt.Println(ci)
			}
		}
	}

	if mergeFlag != "" {
		// Get current work directory
		var workDir string
		if wd, errWorkDir := os.Getwd(); errWorkDir == nil {
			workDir = wd
		} else {
			logErr.Fatal(errWorkDir)
		}

		merged, mergeList, err := mergeVars(mergeFlag, mergeStrategies)
		if err != nil {
			logErr.Fatal(err)
		}

		if validateFlag {
			if err := validateAgainstSchemas(mergeFlag, merged); err != nil {
				logErr.Fatal(err)
			}
		}

		switch outputFlag {
		case "json":
			out, _ := json.Marshal(merged)
			fmt.Printf("%s", out)
		case "yaml":
			out, _ := yaml.Marshal(merged)

			fmt.Printf("---\n")
			printMergeStrategies()
			printPaths(mergeList, workDir)
			fmt.Printf("%s", out)
		default:
			logErr.Fatal("Unsupported format for output:", outputFlag)
		}
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/darrenburns/posting/internal/curl"
	"github.com/darrenburns/posting/internal/importing"
	"github.com/darrenburns/posting/internal/importing/bruno"
	"github.com/darrenburns/posting/internal/importing/openapi"
	"github.com/darrenburns/posting/internal/importing/postman"
	"github.com/darrenburns/posting/internal/paths"
)

const importUsage = `Usage: posting import [options] SOURCE [ENVIRONMENT...]

Import an OpenAPI 3.x specification, Postman v2 collection, Bruno .bru request,
or Bruno collection directory, with any number of Postman environment exports.
The collection format is detected unless --type is given.

Collection variables are written to posting.env, the base environment. Each
environment is written to its own <name>.env, layered on the base. To add
environments to an existing collection, give only environment files and
--output.

Options:
  -t, --type FORMAT    Collection format: openapi, postman, or bruno
  -o, --output DIR     Destination (default: a named folder in the default collection)
  -h, --help           Show this help

Options may appear before or after the files. Existing files are never
overwritten. Unsupported source features are reported as warnings.
`

type importOptions struct {
	sources        []string
	format, output string
}

func parseImportOptions(args []string, stderr io.Writer) (importOptions, error) {
	var opts importOptions
	fs := flag.NewFlagSet("posting import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, importUsage) }
	fs.StringVar(&opts.format, "type", "", "")
	fs.StringVar(&opts.format, "t", "", "")
	fs.StringVar(&opts.output, "output", "", "")
	fs.StringVar(&opts.output, "o", "", "")
	// flag stops at the first positional argument. Keep option values attached
	// while allowing the familiar `posting import file -o directory` spelling.
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flags = append(flags, arg)
			if arg == "--type" || arg == "-type" || arg == "-t" || arg == "--output" || arg == "-output" || arg == "-o" {
				if i+1 < len(args) {
					i++
					flags = append(flags, args[i])
				}
			}
		} else {
			positional = append(positional, arg)
		}
	}
	if err := fs.Parse(flags); err != nil {
		return opts, err
	}
	if len(positional) == 0 {
		return opts, fmt.Errorf("expected a source file or directory")
	}
	opts.sources = positional
	opts.format = strings.ToLower(opts.format)
	if opts.format != "" && opts.format != "postman" && opts.format != "openapi" && opts.format != "bruno" {
		return opts, fmt.Errorf("unknown import type %q (choose openapi, postman, or bruno)", opts.format)
	}
	return opts, nil
}

func importCommand(args []string, stdout, stderr io.Writer) int {
	opts, err := parseImportOptions(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "posting import:", importDiagnostic(err.Error()))
		return 2
	}
	result, format, err := readImports(opts.sources, opts.format)
	if err != nil {
		fmt.Fprintln(stderr, "posting import:", importDiagnostic(err.Error()))
		return 1
	}
	if format == "" && opts.output == "" {
		fmt.Fprintln(stderr, "posting import: importing only environments requires --output DIR, the collection to add them to")
		return 2
	}
	if format != "" && len(result.Requests) == 0 {
		fmt.Fprintln(stderr, "posting import: source contains no importable HTTP requests")
		printImportWarnings(stderr, result.Warnings)
		return 1
	}
	output := opts.output
	if output == "" {
		output = filepath.Join(paths.DefaultCollectionDir(), importing.DirectoryName(result.Name))
	}
	output, err = filepath.Abs(output)
	if err != nil {
		fmt.Fprintln(stderr, "posting import:", importDiagnostic(err.Error()))
		return 1
	}
	written, err := importing.Write(result, output)
	if err != nil {
		fmt.Fprintln(stderr, "posting import:", importDiagnostic(err.Error()))
		return 1
	}
	printImportWarnings(stderr, append(result.Warnings, written.Warnings...))
	if format == "" {
		fmt.Fprintf(stdout, "Imported %d environment(s) into %q.\n", len(written.Environments), output)
	} else {
		fmt.Fprintf(stdout, "Imported %d %s request(s) into %q.\n", len(written.Files), format, output)
	}
	printImportedEnvironments(stdout, output, written.Environments)
	return 0
}

func printImportedEnvironments(stdout io.Writer, output string, environments []importing.WrittenEnvironment) {
	command := "posting -c " + curl.Quote(output)
	var files []string
	selected := ""
	for _, e := range environments {
		file := importDiagnostic(e.File)
		if e.Base {
			file += " (base)"
		} else if selected == "" {
			selected = e.Name
		}
		files = append(files, file)
	}
	if len(environments) > 0 {
		if selected == "" {
			selected = environments[0].Name
		}
		fmt.Fprintln(stdout, "Environments:", strings.Join(files, ", "))
		command += " --env " + curl.Quote(selected)
	}
	fmt.Fprintln(stdout, "Open with:", command)
}

func printImportWarnings(out io.Writer, warnings []string) {
	for _, warning := range warnings {
		// Diagnostics may contain names from an untrusted source document. Render
		// control characters literally rather than interpreting terminal escapes.
		fmt.Fprintln(out, "warning:", importDiagnostic(warning))
	}
}

func importDiagnostic(message string) string {
	quoted := strconv.Quote(message)
	return quoted[1 : len(quoted)-1]
}

// readImports reads at most one collection source and any number of Postman
// environment exports into one Result. The format is empty when only
// environments were given.
func readImports(sources []string, format string) (importing.Result, string, error) {
	var collection importing.Result
	var environments []importing.Environment
	var warnings []string
	collectionSource, collectionFormat := "", ""
	for _, source := range sources {
		data, isDir, err := readSource(source)
		if err != nil {
			return importing.Result{}, "", err
		}
		if !isDir && isPostmanEnvironment(data) {
			e, w, err := postman.ParseEnvironment(data)
			if err != nil {
				return importing.Result{}, "", fmt.Errorf("%s: %w", source, err)
			}
			if e.Name == "" {
				e.Name = strings.TrimSuffix(strings.TrimSuffix(filepath.Base(source), ".json"), ".postman_environment")
			}
			environments = append(environments, e)
			warnings = append(warnings, w...)
			continue
		}
		if collectionSource != "" {
			return importing.Result{}, "", fmt.Errorf("expected one collection, but %s and %s are both collections; other arguments must be Postman environment exports", collectionSource, source)
		}
		collectionSource = source
		if isDir {
			if format != "" && format != "bruno" {
				return importing.Result{}, "", fmt.Errorf("%s import requires a file", format)
			}
			collection, err = bruno.Load(source)
			collectionFormat = "bruno"
		} else {
			collection, collectionFormat, err = parseCollection(source, data, format)
		}
		if err != nil {
			return importing.Result{}, "", err
		}
	}
	collection.Environments = append(collection.Environments, environments...)
	collection.Warnings = append(collection.Warnings, warnings...)
	return collection, collectionFormat, nil
}

const maxImportSize = 32 << 20

// readSource reads a source file, or reports that it is a directory.
func readSource(source string) (data []byte, isDir bool, err error) {
	info, err := os.Stat(source)
	if err != nil {
		return nil, false, err
	}
	if info.IsDir() {
		return nil, true, nil
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("source must be a regular file or Bruno directory")
	}
	if info.Size() > maxImportSize {
		return nil, false, fmt.Errorf("source exceeds the 32 MiB import limit")
	}
	f, err := os.Open(source)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()
	data, err = io.ReadAll(io.LimitReader(f, maxImportSize+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxImportSize {
		return nil, false, fmt.Errorf("source exceeds the 32 MiB import limit")
	}
	return data, false, nil
}

// isPostmanEnvironment reports whether data looks like a Postman environment
// (or globals) export: a values array and no collection item array.
func isPostmanEnvironment(data []byte) bool {
	var document struct{ Values, Item json.RawMessage }
	if json.Unmarshal(data, &document) != nil {
		return false
	}
	isArray := func(raw json.RawMessage) bool { return bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) }
	return isArray(document.Values) && !isArray(document.Item)
}

func parseCollection(source string, data []byte, format string) (importing.Result, string, error) {
	var err error
	if format == "" {
		if strings.EqualFold(filepath.Ext(source), ".bru") {
			format = "bruno"
		} else {
			// JSON permits characters that YAML's scanner rejects. Inspect JSON
			// directly before falling back to YAML (including flow-style YAML).
			var document map[string]json.RawMessage
			if err := json.Unmarshal(data, &document); err != nil {
				var yamlDocument map[string]yaml.Node
				if err := yaml.Unmarshal(data, &yamlDocument); err != nil {
					return importing.Result{}, "", fmt.Errorf("cannot detect import format: %w", err)
				}
				document = make(map[string]json.RawMessage, len(yamlDocument))
				for name := range yamlDocument {
					document[name] = nil
				}
			}
			if _, ok := document["openapi"]; ok {
				format = "openapi"
			} else if _, ok := document["swagger"]; ok {
				format = "openapi"
			} else if _, ok := document["info"]; ok {
				if _, ok := document["item"]; ok {
					format = "postman"
				}
			}
		}
	}
	var result importing.Result
	switch format {
	case "openapi":
		result, err = openapi.Parse(data)
	case "postman":
		result, err = postman.Parse(data)
	case "bruno":
		result, err = bruno.Parse(data)
	default:
		err = fmt.Errorf("cannot detect import format; use --type openapi, postman, or bruno")
	}
	return result, format, err
}

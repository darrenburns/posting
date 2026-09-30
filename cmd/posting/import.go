package main

import (
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

const importUsage = `Usage: posting import [options] SOURCE

Import an OpenAPI 3.x specification, Postman v2 collection, Bruno .bru request,
or Bruno collection directory. The format is detected unless --type is given.

Options:
  -t, --type FORMAT    openapi, postman, or bruno
  -o, --output DIR     Destination (default: a named folder in the default collection)
  -h, --help           Show this help

Options may appear before or after SOURCE. Existing files are never overwritten.
Unsupported source features are reported as warnings.
`

type importOptions struct{ source, format, output string }

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
	if len(positional) != 1 {
		return opts, fmt.Errorf("expected exactly one source file or directory")
	}
	opts.source = positional[0]
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
		fmt.Fprintln(stderr, "posting import:", err)
		return 2
	}
	result, format, err := readImport(opts.source, opts.format)
	if err != nil {
		fmt.Fprintln(stderr, "posting import:", err)
		return 1
	}
	if len(result.Requests) == 0 {
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
		fmt.Fprintln(stderr, "posting import:", err)
		return 1
	}
	written, err := importing.Write(result, output)
	if err != nil {
		fmt.Fprintln(stderr, "posting import:", err)
		return 1
	}
	printImportWarnings(stderr, result.Warnings)
	fmt.Fprintf(stdout, "Imported %d %s request(s) into %q.\n", len(written.Files), format, output)
	command := "posting -c " + curl.Quote(output)
	if written.Environment != "" {
		environment := filepath.Join(output, written.Environment)
		fmt.Fprintf(stdout, "Collection variables: %q\n", environment)
		command += " -e " + curl.Quote(environment)
	}
	fmt.Fprintln(stdout, "Open with:", command)
	return 0
}

func printImportWarnings(out io.Writer, warnings []string) {
	for _, warning := range warnings {
		// Diagnostics may contain names from an untrusted source document. Render
		// control characters literally rather than interpreting terminal escapes.
		escaped := strconv.Quote(warning)
		fmt.Fprintln(out, "warning:", escaped[1:len(escaped)-1])
	}
}

func readImport(source, format string) (importing.Result, string, error) {
	info, err := os.Stat(source)
	if err != nil {
		return importing.Result{}, format, err
	}
	if info.IsDir() {
		if format != "" && format != "bruno" {
			return importing.Result{}, format, fmt.Errorf("%s import requires a file", format)
		}
		result, err := bruno.Load(source)
		return result, "bruno", err
	}
	if !info.Mode().IsRegular() {
		return importing.Result{}, format, fmt.Errorf("source must be a regular file or Bruno directory")
	}
	const maxImportSize = 32 << 20
	if info.Size() > maxImportSize {
		return importing.Result{}, format, fmt.Errorf("source exceeds the 32 MiB import limit")
	}
	f, err := os.Open(source)
	if err != nil {
		return importing.Result{}, format, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxImportSize+1))
	if err != nil {
		return importing.Result{}, format, err
	}
	if len(data) > maxImportSize {
		return importing.Result{}, format, fmt.Errorf("source exceeds the 32 MiB import limit")
	}
	if format == "" {
		if strings.EqualFold(filepath.Ext(source), ".bru") {
			format = "bruno"
		} else {
			var document map[string]yaml.Node
			if err := yaml.Unmarshal(data, &document); err != nil {
				return importing.Result{}, "", fmt.Errorf("cannot detect import format: %w", err)
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

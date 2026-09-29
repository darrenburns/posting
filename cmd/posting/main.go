// Command posting is the Posting 3 terminal HTTP client.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/paths"
	"github.com/darrenburns/posting/internal/ui"
)

const version = "3.0.0-dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// options are the command line flags for starting the app.
type options struct {
	collection string
	envFiles   []string
}

// multiFlag collects a flag that may be given more than once.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "locate":
			return locate(args[1:], stdout, stderr)
		case "version", "--version", "-v":
			fmt.Fprintln(stdout, "Posting "+version)
			return 0
		}
	}

	var opts options
	fs := flag.NewFlagSet("posting", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, `Usage: posting [options]
       posting locate config|collection|themes

A terminal HTTP client.

Options:
  -c, --collection DIR   Collection directory (default: the default collection)
  -e, --env FILE         Environment file; repeat to layer several
  -v, --version          Print the version
`)
	}
	fs.StringVar(&opts.collection, "c", "", "")
	fs.StringVar(&opts.collection, "collection", "", "")
	envs := (*multiFlag)(&opts.envFiles)
	fs.Var(envs, "e", "")
	fs.Var(envs, "env", "")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "posting: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return 2
	}

	cfg, err := appConfig(opts)
	if err != nil {
		fmt.Fprintln(stderr, "posting:", err)
		return 1
	}
	if err := ui.Run(cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// appConfig loads everything the UI needs from disk.
func appConfig(opts options) (ui.Config, error) {
	dir := opts.collection
	if dir == "" {
		dir = paths.DefaultCollectionDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return ui.Config{}, err
		}
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return ui.Config{}, err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ui.Config{}, fmt.Errorf("collection %s is not a directory", dir)
	}
	store := collection.Dir{Root: dir}
	root, problems := store.Load()
	var messages []string
	for _, p := range problems {
		messages = append(messages, "Couldn't load "+p.Error())
	}
	return ui.Config{
		Version:         version,
		Sender:          client.Fake{StageDelay: 60 * time.Millisecond},
		Collection:      root,
		Store:           store,
		Theme:           os.Getenv("POSTING_THEME"),
		StartupMessages: messages,
	}, nil
}

func locate(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: posting locate config|collection|themes")
		return 2
	}
	switch args[0] {
	case "config":
		fmt.Fprintln(stdout, "Config file:")
		fmt.Fprintln(stdout, paths.ConfigFile())
	case "collection":
		fmt.Fprintln(stdout, "Default collection directory:")
		fmt.Fprintln(stdout, paths.DefaultCollectionDir())
	case "themes":
		fmt.Fprintln(stdout, "Themes directory:")
		fmt.Fprintln(stdout, paths.ThemeDir())
	default:
		fmt.Fprintf(stderr, "Unknown thing to locate: %q\n", args[0])
		return 2
	}
	return 0
}

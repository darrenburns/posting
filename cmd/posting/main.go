// Command posting is the Posting 3 terminal HTTP client.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/config"
	"github.com/darrenburns/posting/v3/internal/env"
	"github.com/darrenburns/posting/v3/internal/history"
	"github.com/darrenburns/posting/v3/internal/model"
	"github.com/darrenburns/posting/v3/internal/paths"
	"github.com/darrenburns/posting/v3/internal/themes"
	"github.com/darrenburns/posting/v3/internal/ui"
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
		case "import":
			return importCommand(args[1:], stdout, stderr)
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
       posting import [--type FORMAT] [-o DIR] SOURCE [ENVIRONMENT...]

A terminal HTTP client.

Options:
  -c, --collection DIR   Collection directory (default: the default collection)
  -e, --env ENV          Environment name (staging) or file; repeat to layer
                         several
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
	settings, messages := config.Load()
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
	envDirs := []string{dir, paths.ConfigDir()}
	if cwd, err := os.Getwd(); err == nil {
		envDirs = append([]string{cwd}, envDirs...)
	}
	memory := env.Memory{File: paths.EnvironmentMemory()}
	envFiles, err := environmentFiles(opts.envFiles, envDirs)
	if err != nil {
		return ui.Config{}, err
	}
	if len(opts.envFiles) == 0 {
		if remembered := memory.Recall(dir); remembered != nil {
			envFiles = remembered
		}
	}
	for _, file := range envFiles {
		envDirs = append(envDirs, filepath.Dir(file))
	}

	store := collection.Dir{Root: dir}
	root, problems := store.Load()
	for _, p := range problems {
		messages = append(messages, "Couldn't load "+p.Error())
	}
	var userThemes []themes.Theme
	if settings.LoadUserThemes {
		var themeProblems []error
		userThemes, themeProblems = themes.LoadDir(settings.ThemeDirectory)
		for _, problem := range themeProblems {
			messages = append(messages, "Couldn't load theme "+problem.Error())
		}
	}
	var host []model.Variable
	if settings.UseHostEnvironment {
		host = env.Host()
	}
	if settings.SSL.Password != "" {
		messages = append(messages, "ssl.password isn't supported yet: use an unencrypted key file")
	}
	tlsSettings := client.TLSSettings{
		CABundle: settings.SSL.CABundle,
		CertFile: settings.SSL.CertificatePath,
		KeyFile:  settings.SSL.KeyFile,
	}
	var historyStore ui.HistoryStore
	if settings.History.Enabled {
		historyStore = history.ForCollection(paths.HistoryDir(), dir)
	}
	var watch ui.CollectionWatcher
	if settings.WatchCollectionFiles {
		watch = store
	}
	rpc := client.NewGRPC("posting/"+version, tlsSettings, store.Root)
	return ui.Config{
		Watch:            watch,
		Reload:           store,
		History:          historyStore,
		OpenURL:          terma.OpenURL,
		Version:          version,
		Settings:         &settings,
		NerdFonts:        settings.UseNerdFonts(os.LookupEnv),
		HostVariables:    host,
		UserThemes:       userThemes,
		Sender:           client.ByKind{HTTP: client.NewHTTP("posting/"+version, tlsSettings), GRPC: rpc},
		Describer:        rpc,
		Collection:       root,
		Store:            store,
		Environments:     env.Source{Dirs: envDirs},
		Environment:      envFiles,
		WatchEnvironment: settings.WatchEnvFiles,
		StartupMessages:  messages,

		// Forgetting the choice only costs starting in the default
		// environment next time, so a failed write isn't reported.
		RememberEnvironment: func(files []string) { memory.Remember(dir, files) },
	}, nil
}

// environmentFiles resolves the --env arguments, layered in order. Each is a
// file, or the name of an environment in dirs ("staging" for posting.env +
// staging.env + staging.local.env). Without any, the base environment in the
// working directory is used if it exists, as posting.env was in Posting 2.
func environmentFiles(given []string, dirs []string) ([]string, error) {
	if len(given) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, nil
		}
		return env.Stack(cwd, env.BaseName), nil
	}
	// A file given explicitly is layered where it's given, even if it's
	// already in the stack, so repeating it re-applies its values. Names
	// re-apply their own layers too, but share base layers already loaded.
	var files []string
	for _, arg := range given {
		abs, err := filepath.Abs(arg)
		if err != nil {
			return nil, err
		}
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			files = append(files, abs)
			continue
		}
		named := env.Named(dirs, arg)
		if named == nil {
			return nil, fmt.Errorf("environment %s is neither a file nor an environment in %s", arg, strings.Join(dirs, ", "))
		}
		base := env.Stack(filepath.Dir(named[0]), env.BaseName)
		for _, file := range named {
			if slices.Contains(base, file) && slices.Contains(files, file) {
				continue
			}
			files = append(files, file)
		}
	}
	return files, nil
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

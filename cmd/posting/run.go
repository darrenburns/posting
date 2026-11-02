package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/collection"
	"github.com/darrenburns/posting/v3/internal/config"
	"github.com/darrenburns/posting/v3/internal/env"
	"github.com/darrenburns/posting/v3/internal/model"
	"github.com/darrenburns/posting/v3/internal/remote"
)

const runUsage = `Usage: posting run [options] PATH
       posting run --collection DIR [PATH] [options]

Run saved requests for agents and scripts without a running Posting instance.
PATH is a .posting.yaml file or a directory, searched recursively. Requests
run one at a time in lexical file-path order, not the sidebar's method/name order.
Use numbered filenames to order dependent requests. Cookies and response
captures carry forward during this run only. No history is written.

Options:
  -c, --collection DIR  Collection root for uploads, proto files and environments.
                       PATH is relative to DIR; omit PATH to run all of DIR.
                       Without -c, the root is PATH if it's a directory, or
                       the request file's parent directory.
  -e, --env NAME|FILE   Environment in the collection root, or a file path
                       relative to the working directory. Repeat to layer.
                       Default: posting.env + posting.local.env in the root.
                       The UI's remembered environment is never used.
  --fail-fast          Stop after the first failed request
  --json               Print one JSON report, including response bodies
  -h, --help           Show this help

Options may appear before or after PATH. Text output gives a result per
request and a summary; --json includes results, passed, failed and skipped.
Use --json to inspect response bodies and headers, including error responses.
These may contain secrets. This command doesn't evaluate custom assertions.

Exit codes: 0 = all passed; 1 = a run/load error or failed request;
2 = invalid arguments. HTTP 4xx/5xx, GraphQL errors, non-OK gRPC statuses,
transport errors and failed captures fail the run. HTTP redirects aren't
failures. Empty collections and requests with legacy Python scripts are
rejected. Malformed request files abort the run before anything is sent.

Examples:
  posting run api/health.posting.yaml --json
  posting run api --env dev --json > results.json
  posting run -c api users/get.posting.yaml --env staging
  posting run -c api --fail-fast
`

type runOptions struct {
	target, collection string
	envFiles           []string
	json, failFast     bool
}

func parseRunOptions(args []string) (runOptions, error) {
	var opts runOptions
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		switch name {
		case "h", "help":
			return opts, flagHelp
		case "json", "fail-fast":
			if hasValue {
				return opts, fmt.Errorf("--%s takes no value", name)
			}
			if name == "json" {
				opts.json = true
			} else {
				opts.failFast = true
			}
			continue
		case "c", "collection", "e", "env":
		default:
			return opts, fmt.Errorf("unknown option %s", arg)
		}
		if !hasValue {
			if i+1 >= len(args) {
				return opts, fmt.Errorf("%s needs a value", arg)
			}
			i++
			value = args[i]
		}
		if strings.TrimSpace(value) == "" {
			return opts, fmt.Errorf("--%s needs a nonempty value", name)
		}
		if name == "c" || name == "collection" {
			opts.collection = value
		} else {
			opts.envFiles = append(opts.envFiles, value)
		}
	}
	if len(positional) > 1 {
		return opts, fmt.Errorf("expected one request file or directory")
	}
	if len(positional) == 0 {
		if opts.collection == "" {
			return opts, fmt.Errorf("give a request file, directory, or --collection DIR")
		}
		opts.target = "."
	} else {
		opts.target = positional[0]
		if strings.TrimSpace(opts.target) == "" {
			return opts, fmt.Errorf("request path must not be empty")
		}
	}
	return opts, nil
}

type runResult struct {
	File     string           `json:"file"`
	Name     string           `json:"name"`
	Outcome  string           `json:"outcome"`
	Error    string           `json:"error,omitempty"`
	Response *remote.Response `json:"response,omitempty"`
}

type runReport struct {
	Results []runResult `json:"results"`
	Passed  int         `json:"passed"`
	Failed  int         `json:"failed"`
	Skipped int         `json:"skipped"`
	Error   string      `json:"error,omitempty"`
}

func runCommand(args []string, stdout, stderr io.Writer) int {
	opts, err := parseRunOptions(args)
	if errors.Is(err, flagHelp) {
		fmt.Fprint(stdout, runUsage)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "posting run:", err)
		fmt.Fprintln(stderr, "Run posting run --help for usage.")
		return 2
	}
	report := runReport{Results: []runResult{}}
	fail := func(err error) int {
		report.Error = err.Error()
		return printRunReport(report, opts.json, stdout, stderr)
	}
	root, requests, err := loadRunRequests(opts)
	if err != nil {
		return fail(err)
	}
	settings, warnings := config.Load()
	if len(warnings) > 0 {
		// An unattended run must not silently fall back from invalid settings.
		return fail(errors.New(strings.Join(warnings, "\n")))
	}
	files := env.Stack(root, env.BaseName)
	if len(opts.envFiles) > 0 {
		files, err = environmentFiles(opts.envFiles, []string{root})
		if err != nil {
			return fail(err)
		}
	}
	loaded, err := env.Load(files)
	if err != nil {
		return fail(err)
	}
	for _, warning := range settingsInEnvironmentFiles(files) {
		fmt.Fprintln(stderr, "posting run:", warning)
	}
	var host []model.Variable
	if settings.UseHostEnvironment {
		host = env.Host()
	}
	variables := model.Merge(host, loaded.Variables)
	tls := client.TLSSettings{
		CABundle: settings.SSL.CABundle, CertFile: settings.SSL.CertificatePath,
		KeyFile: settings.SSL.KeyFile, Password: settings.SSL.Password,
	}
	httpClient := client.NewHTTP("posting/"+version, tls)
	httpClient.Root = root
	sender := client.ByKind{HTTP: httpClient, GRPC: client.NewGRPC("posting/"+version, tls, root), WebSocket: &client.WebSocket{HTTP: httpClient}}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	for i, req := range requests {
		if err := ctx.Err(); err != nil {
			report.Skipped = len(requests) - i
			return fail(err)
		}
		result, captures := runRequest(ctx, sender, req, variables)
		var layer []model.Variable
		for name, value := range captures {
			layer = append(layer, model.Variable{Name: name, Value: value, Source: "session", SessionOverride: true})
		}
		variables = model.Merge(variables, layer)
		if opts.json {
			report.Results = append(report.Results, result)
		} else {
			detail := result.Error
			if detail == "" {
				r := result.Response
				detail = fmt.Sprintf("%s %s (%.2f ms)", r.Status, r.Reason, r.ElapsedMS)
			}
			fmt.Fprintf(stdout, "%s %s: %s\n", strings.ToUpper(result.Outcome), req.File, detail)
		}
		if result.Outcome == "passed" {
			report.Passed++
		} else {
			report.Failed++
			if opts.failFast {
				report.Skipped = len(requests) - i - 1
				break
			}
		}
	}
	return printRunReport(report, opts.json, stdout, stderr)
}

// loadRunRequests reads the entire selection before any network activity, so
// unreadable files or unsupported scripts are reported before sending anything.
func loadRunRequests(opts runOptions) (string, []model.Request, error) {
	target := opts.target
	if opts.collection != "" && !filepath.IsAbs(target) {
		target = filepath.Join(opts.collection, target)
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", nil, err
	}
	root := opts.collection
	if root == "" {
		root = filepath.Dir(target)
		if info.IsDir() {
			root = target
		}
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", nil, err
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", nil, fmt.Errorf("collection %s is not a directory", root)
	}
	var requests []model.Request
	if info.IsDir() {
		tree, problems := (collection.Dir{Root: target}).Load()
		if len(problems) > 0 {
			var errs []error
			for _, problem := range problems {
				errs = append(errs, problem)
			}
			return "", nil, errors.Join(errs...)
		}
		tree.Walk(func(_ *model.Collection, req model.Request) {
			req.File = filepath.Join(target, filepath.FromSlash(req.File))
			requests = append(requests, req)
		})
	} else {
		if !strings.HasSuffix(target, collection.FileSuffix) {
			return "", nil, fmt.Errorf("request file must end in %s", collection.FileSuffix)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return "", nil, err
		}
		req, err := collection.ParseRequest(data, target)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", target, err)
		}
		requests = append(requests, req)
	}
	if len(requests) == 0 {
		return "", nil, fmt.Errorf("no request files found in %s", target)
	}
	for i := range requests {
		req := &requests[i]
		rel, err := filepath.Rel(root, req.File)
		if err != nil {
			return "", nil, err
		}
		req.File = filepath.ToSlash(rel)
		if strings.TrimSpace(req.URL) == "" {
			return "", nil, fmt.Errorf("%s: request has no URL", req.File)
		}
		if req.Scripts != (model.Scripts{}) {
			return "", nil, fmt.Errorf("%s: Posting 3 doesn't run Python scripts; remove or replace its scripts before running headlessly", req.File)
		}
	}
	slices.SortFunc(requests, func(a, b model.Request) int { return strings.Compare(a.File, b.File) })
	return root, requests, nil
}

func runRequest(ctx context.Context, sender client.Sender, req model.Request, variables []model.Variable) (runResult, map[string]string) {
	result := runResult{File: req.File, Name: req.Name, Outcome: "failed"}
	snapshot := model.NewVariableSnapshot(variables, req.VariableScope, os.LookupEnv)
	resolved, err := snapshot.ForRequest(req)
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	resp, err := sender.Send(ctx, client.Call{Request: req, Variables: model.Values(resolved), ResolveBody: snapshot.Substitute})
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	status := model.StatusOf(req, resp)
	result.Response = remote.NewResponse(resp, status)
	if status.Class == model.StatusClassError || status.Class == model.StatusClassWarning {
		result.Error = strings.TrimSpace(status.Code + " " + status.Text)
		return result, nil
	}
	captures, err := model.CaptureValues(req.Captures, resp.Body)
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	result.Outcome = "passed"
	return result, captures
}

func printRunReport(report runReport, asJSON bool, stdout, stderr io.Writer) int {
	if report.Error != "" {
		fmt.Fprintln(stderr, "posting run:", report.Error)
	}
	if asJSON {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(stderr, "posting run:", err)
			return 1
		}
	} else {
		if _, err := fmt.Fprintf(stdout, "%d passed, %d failed, %d skipped\n", report.Passed, report.Failed, report.Skipped); err != nil {
			fmt.Fprintln(stderr, "posting run:", err)
			return 1
		}
	}
	if report.Failed > 0 || report.Error != "" {
		return 1
	}
	return 0
}

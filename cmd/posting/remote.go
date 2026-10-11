package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/darrenburns/posting/internal/paths"
	"github.com/darrenburns/posting/internal/remote"
)

const remoteUsage = `Usage: posting remote COMMAND [REQUEST] [options]

Drive the Posting running in another terminal. Requests open and send in its
window, with its active environment, so whoever is watching sees each request
and its response, and the response is printed here too. This is how coding
agents can work through a Posting someone has open beside them.

Commands:
  list              List the running Postings
  requests          List the requests saved in the collection
  show              Print the request in the active tab, in Posting's YAML
                    request format
  open [REQUEST]    Open a request in a tab, without sending it
  send [REQUEST]    Send a request, wait for the response, and print it
  response          Print the active tab's latest response
  env [NAME|FILE...]
                    List the environments, marking the active one with *,
                    after switching to the one named, or layering the files
                    given. --none switches to no environment.
  save [FILE]       Save the request in the active tab: as FILE, or where it
                    was saved before, or else under its name. FILE is relative
                    to the collection unless it starts with /, ./ or ../, and
                    a request already saved there isn't replaced. --name NAME
                    names the request.

REQUEST is a request saved in the collection: its file (users/get.posting.yaml
or users/get), a path to it, or its name. A request file outside the collection
is opened unsaved. Without REQUEST, open and send use the active tab. Instead
of REQUEST, give one of:
  --curl COMMAND    a curl command
  --yaml FILE       a request in Posting's YAML format; - reads standard input

Requests that aren't saved open in a tab of their own, which is reused until
someone edits or saves it. Responses print as status line, headers, a blank line and
the body. Error statuses aren't failures: send exits 1 only when no response
arrives.

Options:
  --json            Print the result as JSON
  --instance PID    The Posting to drive, by process ID or socket, when several
                    are running. Defaults to $POSTING_INSTANCE, or else the only
                    one running, the one started in this directory, or the one
                    whose collection holds this directory.
  -h, --help        Show this help

Examples:
  posting remote requests
  posting remote send users/list
  posting remote send --curl 'curl -X POST https://example.com/users -d "{}"'
  posting remote show > draft.posting.yaml   # edit it, then
  posting remote send --yaml draft.posting.yaml
  posting remote send users/get --json | jq -r .response.body
  posting remote env staging
  posting remote open --curl 'curl https://example.com/health'
  posting remote save health --name "Health check"
`

type remoteOptions struct {
	command, ref, curl, yaml, instance string
	json                               bool
	// env: the environment's name or files, or none.
	env   []string
	noEnv bool
	// save: the file and name to save as.
	file, name string
}

func parseRemoteOptions(args []string) (remoteOptions, error) {
	var opts remoteOptions
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
		var target *string
		switch name {
		case "h", "help":
			return opts, flagHelp
		case "json", "none":
			if hasValue {
				return opts, fmt.Errorf("--%s takes no value", name)
			}
			if name == "json" {
				opts.json = true
			} else {
				opts.noEnv = true
			}
			continue
		case "name":
			target = &opts.name
		case "curl":
			target = &opts.curl
		case "yaml":
			target = &opts.yaml
		case "instance":
			target = &opts.instance
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
		*target = value
	}
	if len(positional) == 0 {
		return opts, errors.New("no command given")
	}
	opts.command, positional = positional[0], positional[1:]
	switch opts.command {
	case "open", "send":
		if len(positional) > 1 {
			return opts, fmt.Errorf("unexpected argument %q", positional[1])
		}
		if len(positional) == 1 {
			opts.ref = positional[0]
		}
		given := 0
		for _, v := range []string{opts.ref, opts.curl, opts.yaml} {
			if v != "" {
				given++
			}
		}
		if given > 1 {
			return opts, errors.New("give only one of REQUEST, --curl and --yaml")
		}
	case "env":
		opts.env = positional
		if opts.noEnv && len(positional) > 0 {
			return opts, errors.New("give an environment or --none, not both")
		}
	case "save":
		if len(positional) > 1 {
			return opts, fmt.Errorf("unexpected argument %q", positional[1])
		}
		if len(positional) == 1 {
			opts.file = positional[0]
		}
	case "list", "requests", "show", "response":
		if len(positional) > 0 {
			return opts, fmt.Errorf("unexpected argument %q", positional[0])
		}
	default:
		return opts, fmt.Errorf("unknown command %q", opts.command)
	}
	if (opts.curl != "" || opts.yaml != "") && opts.command != "open" && opts.command != "send" {
		return opts, errors.New("--curl and --yaml are for open and send")
	}
	if opts.noEnv && opts.command != "env" {
		return opts, errors.New("--none is for env")
	}
	if opts.name != "" && opts.command != "save" {
		return opts, errors.New("--name is for save")
	}
	return opts, nil
}

// flagHelp asks for the usage.
var flagHelp = errors.New("help")

func remoteCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, err := parseRemoteOptions(args)
	if errors.Is(err, flagHelp) {
		fmt.Fprint(stdout, remoteUsage)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "posting remote:", err)
		fmt.Fprintln(stderr, "Run posting remote --help for usage.")
		return 2
	}
	ctx := context.Background()
	instances, err := remote.Instances(ctx, paths.SocketDir())
	if err != nil {
		fmt.Fprintln(stderr, "posting remote:", err)
		return 1
	}
	if opts.command == "list" {
		return printInstances(instances, opts.json, stdout, stderr)
	}
	want := opts.instance
	if want == "" {
		want = os.Getenv("POSTING_INSTANCE")
	}
	cwd, _ := os.Getwd()
	instance, err := remote.Choose(instances, want, cwd)
	if err != nil {
		fmt.Fprintln(stderr, "posting remote:", err)
		return 1
	}
	req, err := remoteRequest(opts, instance, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "posting remote:", err)
		return 1
	}

	var result any
	switch opts.command {
	case "requests":
		result = &[]remote.SavedRequest{}
	case "env":
		result = &[]remote.Environment{}
	case "show", "open", "save":
		result = &remote.Shown{}
	case "send", "response":
		result = &remote.Exchange{}
	}
	if err := remote.Call(ctx, instance.Socket, req, result); err != nil {
		fmt.Fprintln(stderr, "posting remote:", err)
		return 1
	}
	if opts.json {
		if err := printJSON(stdout, result); err != nil {
			fmt.Fprintln(stderr, "posting remote:", err)
			return 1
		}
	}
	switch result := result.(type) {
	case *[]remote.SavedRequest:
		if !opts.json {
			printSavedRequests(*result, stdout)
		}
	case *remote.Shown:
		if !opts.json {
			switch opts.command {
			case "open":
				fmt.Fprintf(stdout, "Opened %s in tab %d\n", result.Tab.Title, result.Tab.ID)
			case "save":
				fmt.Fprintf(stdout, "Saved %s as %s\n", result.Tab.Title, result.Tab.File)
			default:
				fmt.Fprint(stdout, result.YAML)
			}
		}
	case *[]remote.Environment:
		if !opts.json {
			printEnvironments(*result, stdout)
		}
	case *remote.Exchange:
		return printExchange(*result, opts.json, stdout, stderr)
	}
	return 0
}

// remoteRequest is the command opts asks for. Request files are read here,
// since the running Posting reads nothing from outside its collection.
func remoteRequest(opts remoteOptions, instance remote.Info, stdin io.Reader) (remote.Request, error) {
	req := remote.Request{Command: opts.command, Ref: opts.ref, Curl: opts.curl, Name: opts.name, NoEnvironment: opts.noEnv}
	switch {
	case len(opts.env) > 0:
		return environmentRequest(req, opts.env)
	case opts.file != "":
		file := filepath.ToSlash(opts.file)
		if filepath.IsAbs(opts.file) || strings.HasPrefix(file, "./") || strings.HasPrefix(file, "../") {
			abs, err := filepath.Abs(opts.file)
			if err != nil {
				return req, err
			}
			rel, ok := inCollection(instance.Collection, abs)
			if !ok {
				return req, fmt.Errorf("%s isn't in the collection, %s", opts.file, instance.Collection)
			}
			file = rel
		}
		req.File = file
		return req, nil
	case opts.yaml == "-":
		data, err := io.ReadAll(stdin)
		if err != nil {
			return req, err
		}
		req.YAML = string(data)
	case opts.yaml != "":
		data, err := os.ReadFile(opts.yaml)
		if err != nil {
			return req, err
		}
		req.YAML = string(data)
	case opts.ref != "":
		abs, err := filepath.Abs(opts.ref)
		if err != nil {
			return req, nil
		}
		if info, err := os.Stat(abs); err != nil || info.IsDir() {
			// A file or name in the collection, wherever this is run.
			return req, nil
		}
		if rel, ok := inCollection(instance.Collection, abs); ok {
			req.Ref = rel
			return req, nil
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return req, err
		}
		req.Ref, req.YAML = "", string(data)
	}
	return req, nil
}

// environmentRequest asks to switch to the environment args name: files to
// layer, or else the name of one.
func environmentRequest(req remote.Request, args []string) (remote.Request, error) {
	var files []string
	for _, arg := range args {
		if info, err := os.Stat(arg); err == nil && !info.IsDir() {
			abs, err := filepath.Abs(arg)
			if err != nil {
				return req, err
			}
			files = append(files, abs)
		}
	}
	switch {
	case len(files) == len(args):
		req.EnvironmentFiles = files
	case len(args) == 1:
		req.Environment = args[0]
	default:
		return req, errors.New("give one environment name, or environment files to layer")
	}
	return req, nil
}

// inCollection is file's path relative to the collection dir, if it's in
// it.
func inCollection(dir, file string) (string, bool) {
	if dir == "" {
		return "", false
	}
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	if real, err := filepath.EvalSymlinks(file); err == nil {
		file = real
	}
	rel, err := filepath.Rel(dir, file)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func printJSON(w io.Writer, v any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func printInstances(instances []remote.Info, asJSON bool, stdout, stderr io.Writer) int {
	if asJSON {
		if instances == nil {
			instances = []remote.Info{}
		}
		if err := printJSON(stdout, instances); err != nil {
			fmt.Fprintln(stderr, "posting remote:", err)
			return 1
		}
		return 0
	}
	if len(instances) == 0 {
		fmt.Fprintln(stderr, "No Posting is running.")
		return 0
	}
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PID\tCOLLECTION\tSTARTED IN")
	for _, in := range instances {
		fmt.Fprintf(w, "%d\t%s\t%s\n", in.PID, in.Collection, in.Cwd)
	}
	w.Flush()
	return 0
}

func printEnvironments(environments []remote.Environment, stdout io.Writer) {
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\tNAME\tVARIABLES\tFILES")
	for _, e := range environments {
		marker, variables := "", strings.Join(e.Variables, " ")
		if e.Active {
			marker = "*"
		}
		if e.Error != "" {
			variables = "couldn't read: " + e.Error
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", marker, e.Name, variables, strings.Join(e.Files, " + "))
	}
	w.Flush()
}

func printSavedRequests(requests []remote.SavedRequest, stdout io.Writer) {
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "METHOD\tFILE\tNAME\tURL")
	for _, r := range requests {
		method := r.Method
		if method == "" {
			method = strings.ToUpper(r.Kind)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", method, r.File, r.Name, r.URL)
	}
	w.Flush()
}

// printExchange prints how an exchange went: as JSON if it was asked for,
// otherwise its response as `curl -i` would. Only a response is success.
func printExchange(e remote.Exchange, asJSON bool, stdout, stderr io.Writer) int {
	var problem string
	switch e.Outcome {
	case remote.OutcomeDone:
	case remote.OutcomeFailed:
		problem = "the request failed: " + e.Error
	case remote.OutcomeCancelled:
		problem = "the request was cancelled"
	case remote.OutcomeSending:
		problem = "the request is still being sent"
	default:
		problem = "there's no response yet"
	}
	if problem != "" {
		if !asJSON {
			fmt.Fprintln(stderr, "posting remote:", problem)
		}
		return 1
	}
	if asJSON {
		return 0
	}
	r := e.Response
	fmt.Fprintln(stdout, strings.Join(strings.Fields(r.Proto+" "+r.Status+" "+r.Reason), " "))
	for _, h := range r.Headers {
		fmt.Fprintf(stdout, "%s: %s\n", h.Name, h.Value)
	}
	fmt.Fprintln(stdout)
	switch {
	case r.BodyBase64 != "":
		body, _ := base64.StdEncoding.DecodeString(r.BodyBase64)
		fmt.Fprintf(stderr, "posting remote: the body is %d bytes of binary data; use --json to get it in base64\n", len(body))
	case strings.Contains(r.ContentType, "json"):
		var pretty bytes.Buffer
		if json.Indent(&pretty, []byte(r.Body), "", "  ") == nil {
			fmt.Fprintln(stdout, pretty.String())
			break
		}
		fmt.Fprintln(stdout, r.Body)
	case r.Body != "":
		fmt.Fprint(stdout, r.Body)
		if !strings.HasSuffix(r.Body, "\n") {
			fmt.Fprintln(stdout)
		}
	}
	for _, h := range r.Trailers {
		fmt.Fprintf(stdout, "%s: %s\n", h.Name, h.Value)
	}
	return 0
}

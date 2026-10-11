// Package remote lets other programs drive a running Posting: `posting
// remote send` in one terminal sends a request through the Posting open in
// another, which shows the request and its response as if the user had sent
// it, and hands the response back. It is how coding agents work alongside
// someone watching Posting.
//
// Each running Posting listens on a Unix socket of its own in
// paths.SocketDir, named for its process ID. A connection carries one
// Request, as JSON, and gets one Reply back.
package remote

import (
	"encoding/json"
	"time"
)

// Commands a running Posting understands.
const (
	// CommandInfo describes the instance. It is answered without the UI.
	CommandInfo = "info"
	// CommandRequests lists the requests saved in the collection.
	CommandRequests = "requests"
	// CommandShow shows the request in the active tab.
	CommandShow = "show"
	// CommandOpen opens a request in a tab without sending it.
	CommandOpen = "open"
	// CommandSend opens a request, as CommandOpen does, sends it, and
	// replies once the exchange ends.
	CommandSend = "send"
	// CommandResponse shows the active tab's latest response.
	CommandResponse = "response"
	// CommandEnv lists the environments, after switching to the one named
	// by Environment or EnvironmentFiles, or to none with NoEnvironment.
	CommandEnv = "env"
	// CommandSave saves the request in the active tab: where it was saved
	// before, or as File.
	CommandSave = "save"
)

// Request is a command for a running Posting. Open and send act on the
// request given by at most one of Ref, Curl and YAML, or the active tab's
// request when none is given.
type Request struct {
	Command string `json:"command"`
	// Ref names a request saved in the collection: its file relative to the
	// collection, an absolute path to it, or its name. An absolute path to a
	// request file outside the collection opens that file unsaved.
	Ref string `json:"ref,omitempty"`
	// Curl is a curl command to import.
	Curl string `json:"curl,omitempty"`
	// YAML is a request in Posting's request file format.
	YAML string `json:"yaml,omitempty"`

	// Environment names the environment to switch to.
	Environment string `json:"environment,omitempty"`
	// EnvironmentFiles are the files of the environment to switch to,
	// layered in order.
	EnvironmentFiles []string `json:"environmentFiles,omitempty"`
	// NoEnvironment switches to no environment.
	NoEnvironment bool `json:"noEnvironment,omitempty"`

	// File is where save puts the request, relative to the collection.
	// The .posting.yaml extension may be left off.
	File string `json:"file,omitempty"`
	// Name names the request being saved.
	Name string `json:"name,omitempty"`
}

// Environment is an environment that can be switched to. Its variables'
// values aren't given, since they're often secrets.
type Environment struct {
	Name string `json:"name"`
	// Files are layered in order.
	Files     []string `json:"files"`
	Active    bool     `json:"active"`
	Variables []string `json:"variables"`
	// Error says why the environment couldn't be read.
	Error string `json:"error,omitempty"`
}

// Reply answers a Request. Result holds the command's result when OK.
type Reply struct {
	OK     bool            `json:"ok"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

// Info describes a running instance.
type Info struct {
	PID     int    `json:"pid"`
	Version string `json:"version"`
	// Collection is the collection directory.
	Collection string `json:"collection"`
	// Cwd is the directory Posting was started in.
	Cwd     string    `json:"cwd"`
	Started time.Time `json:"started"`
	// Socket is where the instance listens. It is filled in by the client.
	Socket string `json:"socket,omitempty"`
}

// SavedRequest is a request saved in the collection.
type SavedRequest struct {
	// File is relative to the collection directory.
	File   string `json:"file"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Method string `json:"method,omitempty"`
	URL    string `json:"url"`
}

// Tab is an open request tab.
type Tab struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	// File is the saved request the tab holds, relative to the collection,
	// or empty when the request isn't saved.
	File string `json:"file,omitempty"`
	// Dirty is set when the tab has changes that aren't saved.
	Dirty bool `json:"dirty"`
}

// Shown is a request open in a tab: the result of show and open.
type Shown struct {
	Tab Tab `json:"tab"`
	// Environment is the name of the active environment, if there is one.
	Environment string `json:"environment,omitempty"`
	// YAML is the request as edited, in Posting's request file format.
	// Variables in it aren't substituted.
	YAML string `json:"yaml"`
}

// Outcomes of an exchange.
const (
	OutcomeDone      = "done"
	OutcomeFailed    = "failed"
	OutcomeCancelled = "cancelled"
	OutcomeSending   = "sending"
	OutcomeNone      = "none"
)

// Exchange is a request sent from a tab and how it ended: the result of
// send and response.
type Exchange struct {
	Tab         Tab    `json:"tab"`
	Environment string `json:"environment,omitempty"`
	// Outcome is OutcomeDone when a response arrived, whatever its status.
	Outcome string `json:"outcome"`
	// Error says why the exchange failed.
	Error    string    `json:"error,omitempty"`
	Response *Response `json:"response,omitempty"`
}

// Response is a response as Posting received it.
type Response struct {
	// Method and URL are those sent, after variables were substituted and
	// redirects followed.
	Method string `json:"method,omitempty"`
	URL    string `json:"url"`
	// Status is the status code ("200"), or a gRPC status name ("OK").
	Status string `json:"status"`
	// Reason is the status's text: "OK", "Not Found", "2 errors".
	Reason string `json:"reason,omitempty"`
	// StatusCode is the HTTP status code; gRPC responses have none.
	StatusCode  int      `json:"statusCode,omitempty"`
	Proto       string   `json:"proto,omitempty"`
	Headers     []Header `json:"headers"`
	Trailers    []Header `json:"trailers,omitempty"`
	ContentType string   `json:"contentType,omitempty"`
	// Body is the body as text. A body that isn't UTF-8 is in BodyBase64
	// instead.
	Body       string    `json:"body"`
	BodyBase64 string    `json:"bodyBase64,omitempty"`
	Size       int       `json:"size"`
	ElapsedMS  float64   `json:"elapsedMs"`
	ReceivedAt time.Time `json:"receivedAt"`
}

// Header is a response header or trailer.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

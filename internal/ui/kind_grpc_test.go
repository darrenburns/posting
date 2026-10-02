package ui

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
)

// recordingDescriber describes with the fake, recording each address it was
// asked about.
type recordingDescriber struct {
	mu    sync.Mutex
	asked []string
}

func (d *recordingDescriber) Describe(ctx context.Context, call client.Call) (client.Schema, error) {
	d.mu.Lock()
	d.asked = append(d.asked, model.Substitute(call.Request.URL, call.Lookup))
	d.mu.Unlock()
	return client.Fake{}.Describe(ctx, call)
}

func (d *recordingDescriber) calls() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.asked...)
}

func grpcRequest() model.Request {
	r := model.GRPCKind.New()
	r.URL = "localhost:50051"
	r.Headers = []model.KeyValue{{Name: "x-tenant", Value: "core", Enabled: true}}
	r.Auth = model.Auth{Type: model.AuthBearer, Token: "${API_TOKEN}"}
	return r
}

// grpcScreen is the test app with an unsaved gRPC request open on screen.
// Discovery results wait in updates until the test runs them.
func grpcScreen(tt *testing.T, req model.Request) (*screen, *grpcEditor, *recordingDescriber, chan func()) {
	tt.Helper()
	app := testApp()
	describer := &recordingDescriber{}
	app.describer = describer
	app.openRequest(req)
	s := app.current()
	updates := make(chan func(), 8)
	s.dispatch = func(fn func()) { updates <- fn }
	return newScreen(app, snapW, snapH), s.payloads[model.KindGRPC].(*grpcEditor), describer, updates
}

func (s *screen) shows(text string) bool {
	return strings.Contains(s.renderer.ScreenText(), text)
}

func TestGRPCRequestTabs(tt *testing.T) {
	app := testApp()
	app.openRequest(grpcRequest())
	s := app.current()
	if got, want := tabLabels(s), "Message,Proto,Metadata,Auth,Info,Options"; got != want {
		tt.Fatalf("gRPC tabs = %s, want %s", got, want)
	}
	if s.requestTab.Peek() != "grpc-message" {
		tt.Fatalf("a gRPC request opens on %q, want its Message tab", s.requestTab.Peek())
	}
	if got := s.contentFocusID("grpc-message"); got != grpcMethodID {
		tt.Fatalf("the Message tab's first field is %s, want the method", got)
	}
}

func TestMethodSelectorHotkeyPicksGRPC(tt *testing.T) {
	app := testApp()
	for _, bind := range (methodSelector{app: app}).Keybinds() {
		if bind.Key == "r" {
			bind.Action()
		}
	}
	if got := app.current().kind.Peek(); got != model.KindGRPC {
		tt.Fatalf("r made the request %s", got)
	}
}

func TestGRPCOptionsHaveNoHTTPOnlyRows(tt *testing.T) {
	for _, c := range []struct {
		req  model.Request
		want []string
	}{
		{grpcRequest(), []string{"req-opt-verify", "req-opt-substitute", "req-opt-timeout"}},
		{model.HTTPKind.Example(), []string{"req-opt-follow", "req-opt-verify", "req-opt-cookies", "req-opt-substitute", "req-opt-proxy", "req-opt-timeout"}},
	} {
		app := testApp()
		app.openRequest(c.req)
		app.current().selectRequestTab("options")
		var got []string
		for _, entry := range newScreen(app, snapW, snapH).renderer.Render(app) {
			if strings.HasPrefix(entry.ID, "req-opt-") {
				got = append(got, entry.ID)
			}
		}
		if !reflect.DeepEqual(got, c.want) {
			tt.Errorf("%s options = %v, want %v", c.req.Kind().Label, got, c.want)
		}
		if first := app.current().contentFocusID("options"); first != c.want[0] {
			tt.Errorf("%s options focus %s first, want %s", c.req.Kind().Label, first, c.want[0])
		}
	}
}

func TestGRPCDiscoveryListsTheServersMethods(tt *testing.T) {
	sc, e, describer, updates := grpcScreen(tt, grpcRequest())
	if !sc.shows("Loading methods from localhost:50051…") {
		tt.Fatalf("showing the Message tab should start discovery:\n%s", sc.renderer.ScreenText())
	}
	(<-updates)()
	sc.render()
	if !sc.shows("4 methods via reflection · plaintext") {
		tt.Fatalf("no method count on screen:\n%s", sc.renderer.ScreenText())
	}
	var listed []string
	for _, suggestion := range e.methods.Suggestions.Peek() {
		listed = append(listed, suggestion.Label+" "+suggestion.Description)
	}
	want := []string{
		"posting.example.v1.Greeter/Chat bidi stream · ChatMessage → ChatMessage",
		"posting.example.v1.Greeter/CollectNames client stream · HelloRequest → NameCount",
		"posting.example.v1.Greeter/SayHello unary · HelloRequest → HelloReply",
		"posting.example.v1.Greeter/StreamGreetings server stream · HelloRequest → HelloReply",
	}
	if !reflect.DeepEqual(listed, want) {
		tt.Fatalf("method suggestions = %q", listed)
	}

	sc.focusID(tt, grpcMethodID)
	sc.render()
	if got := describer.calls(); !reflect.DeepEqual(got, []string{"localhost:50051"}) {
		tt.Fatalf("redrawing and focusing the method asked again for the same server: %v", got)
	}
	sc.pressKey(tt, "ctrl+r")
	(<-updates)()
	if got := describer.calls(); len(got) != 2 {
		tt.Fatalf("ctrl+r in the method field should ask again, asked %v", got)
	}
}

func TestOpeningAGRPCRequestNeverContactsTheServer(tt *testing.T) {
	sc, _, describer, _ := grpcScreen(tt, grpcRequest())
	s := sc.app.current()
	for _, tab := range []string{"grpc-proto", "headers", "grpc-message"} {
		s.selectRequestTab(tab)
		sc.render()
	}
	if got := describer.calls(); got != nil {
		tt.Fatalf("opening the request and showing its tabs asked the server for its methods: %v", got)
	}
	if !sc.shows("Move to this field to list the server's methods") {
		tt.Fatalf("the line under the method should say how to list the methods:\n%s", sc.renderer.ScreenText())
	}
}

func TestGRPCDiscoveryRunsOnExplicitEvents(tt *testing.T) {
	sc, _, describer, updates := grpcScreen(tt, grpcRequest())
	s := sc.app.current()
	asked := func(want int, why string) {
		tt.Helper()
		if got := len(describer.calls()); got != want {
			tt.Fatalf("%s: the server was asked %d times, want %d", why, got, want)
		}
	}
	sc.focusID(tt, grpcMethodID)
	asked(1, "the method field gaining focus")
	(<-updates)()
	sc.focusID(tt, urlInputID)
	sc.focusID(tt, grpcMethodID)
	asked(1, "focusing the method again with nothing changed")

	s.token.SetText("fixed-token")
	sc.focusID(tt, urlInputID)
	sc.focusID(tt, grpcMethodID)
	asked(2, "focusing the method after fixing the token")
	(<-updates)()

	sc.pressKey(tt, "ctrl+r")
	asked(3, "ctrl+r in the method field")
	(<-updates)()

	for _, item := range sc.app.paletteItems() {
		if item.Label == "Refresh gRPC methods" {
			item.Action()
		}
	}
	asked(4, "the palette's Refresh gRPC methods")
	(<-updates)()

	sc.app.send()
	for s.phase.Peek() == exchangeSending {
		(<-updates)()
	}
	asked(5, "a send of the request completing")
}

func TestGRPCProtoFilesAreListedOnOpenAndReadAgainOnEachTrigger(tt *testing.T) {
	req := grpcRequest()
	req.Payload = model.GRPC{Protos: model.ProtoSet{Files: []string{"protos/greeter.proto"}}}
	sc, e, describer, updates := grpcScreen(tt, req)
	if got := len(describer.calls()); got != 1 {
		tt.Fatalf("proto files never contact the server, so opening the request should list them: asked %d times", got)
	}
	(<-updates)()
	sc.render()
	if !sc.shows("4 methods from 1 proto file") {
		tt.Fatalf("no method count on screen:\n%s", sc.renderer.ScreenText())
	}
	sc.focusID(tt, grpcMethodID)
	if got := len(describer.calls()); got != 2 {
		tt.Fatalf("each trigger should read the proto files again, so an edited one is seen: asked %d times", got)
	}
	(<-updates)()
	if c := e.catalog.Peek(); c.phase != catalogReady {
		tt.Fatalf("catalog = %+v", c)
	}
}

func TestGRPCDiscoveryFollowsTheAddressAndDropsStaleResults(tt *testing.T) {
	sc, e, describer, updates := grpcScreen(tt, grpcRequest())
	s := sc.app.current()
	s.url.SetText("localhost:${PORT}")
	sc.app.sessionVars.Set(map[string]string{"PORT": "6000"})
	sc.focusID(tt, grpcMethodID)
	if got := describer.calls(); !reflect.DeepEqual(got, []string{"localhost:50051", "localhost:6000"}) {
		tt.Fatalf("focusing the method after changing the address should ask the new server: %v", got)
	}
	(<-updates)()
	(<-updates)()
	sc.render()
	if c := e.catalog.Peek(); c.phase != catalogReady || c.source.address != "localhost:6000" {
		tt.Fatalf("catalog = %+v; the first server's late answer must not replace the second's", c)
	}
	if !sc.shows("4 methods via reflection · plaintext") {
		tt.Fatalf("no method count on screen:\n%s", sc.renderer.ScreenText())
	}
}

func TestGRPCDiscoveryProblemsShowUnderTheMethod(tt *testing.T) {
	req := grpcRequest()
	req.URL = "${GRPC_HOST}:50051"
	sc, _, describer, _ := grpcScreen(tt, req)
	if !sc.shows("Variable not defined: $GRPC_HOST. Press ctrl+r to try again") {
		tt.Fatalf("an address that can't be resolved should say why:\n%s", sc.renderer.ScreenText())
	}
	if got := describer.calls(); got != nil {
		tt.Fatalf("an unresolved address was asked for its methods: %v", got)
	}

	req.URL = ""
	sc, _, _, _ = grpcScreen(tt, req)
	if !sc.shows("Enter the server's address to list its methods") {
		tt.Fatalf("a blank address should say how to get methods:\n%s", sc.renderer.ScreenText())
	}
}

func TestChoosingAGRPCMethodFillsInItsMessage(tt *testing.T) {
	sc, e, _, updates := grpcScreen(tt, grpcRequest())
	(<-updates)()
	sc.focusID(tt, grpcMethodID)
	if !e.methods.Visible.Peek() || !sc.shows("server stream · HelloRequest → HelloReply") {
		tt.Fatalf("the method list should open as soon as the method field has focus:\n%s", sc.renderer.ScreenText())
	}
	input := sc.focus.Focused().(t.PasteHandler)
	input.HandlePaste("sayhel")
	sc.render()
	sc.pressKey(tt, "enter")

	s := sc.app.current()
	got := s.Snapshot().Payload.(model.GRPC)
	if got.Method != "posting.example.v1.Greeter/SayHello" || got.Message != "{\n  \"name\": \"\"\n}" {
		tt.Fatalf("choosing SayHello made the payload %+v", got)
	}
	if !sc.shows("unary · HelloRequest → HelloReply") || s.title.Peek() != "Greeter/SayHello" || !s.dirty.Peek() {
		tt.Fatalf("title %q, dirty %v, screen:\n%s", s.title.Peek(), s.dirty.Peek(), sc.renderer.ScreenText())
	}

	methods := e.catalog.Peek().schema.Methods
	e.pick(methods[0])
	if got := e.message.GetText(); got != methods[0].Template {
		tt.Fatalf("an untouched template should follow the method, message = %q", got)
	}
	e.message.SetText(`[{"text": "mine"}]`)
	e.pick(methods[1])
	if got := e.message.GetText(); got != `[{"text": "mine"}]` {
		tt.Fatalf("choosing a method replaced a message the user wrote: %q", got)
	}
	if !e.insertTemplate() || e.message.GetText() != methods[1].Template {
		tt.Fatalf("inserting the template on demand gave %q", e.message.GetText())
	}
}

func TestEnterInTheGRPCMethodKeepsAMethodChosenInAnotherTab(tt *testing.T) {
	app := testApp()
	app.describer = &recordingDescriber{}
	updates := make(chan func(), 8)
	open := func(method string) {
		req := grpcRequest()
		req.Payload = model.GRPC{Method: method}
		app.openSession(req).dispatch = func(fn func()) { updates <- fn }
	}
	open("posting.example.v1.Greeter/SayHello")
	sc := newScreen(app, snapW, snapH)
	(<-updates)()
	open("posting.example.v1.Greeter/StreamGreetings")
	sc.render()
	(<-updates)()
	sc.focusID(tt, grpcMethodID)

	app.cycleSession(-1)
	sc.render()
	if sc.focus.FocusedID() != grpcMethodID {
		tt.Fatalf("switching tabs moved focus to %s", sc.focus.FocusedID())
	}
	// A listed method is a row naming the method and its signature.
	row := regexp.MustCompile(`(posting\.example\.v1\.Greeter/\w+) .* → `)
	listed := func() (methods []string) {
		for _, line := range strings.Split(sc.renderer.ScreenText(), "\n") {
			if m := row.FindStringSubmatch(line); m != nil {
				methods = append(methods, m[1])
			}
		}
		return methods
	}
	e := app.current().payloads[model.KindGRPC].(*grpcEditor)
	for _, at := range []int{0, len("posting.example.v1.Greeter/")} {
		e.method.CursorIndex.Set(at)
		sc.render()
		if got := listed(); !reflect.DeepEqual(got, []string{"posting.example.v1.Greeter/SayHello"}) {
			tt.Errorf("with the cursor at %d the method list shows %q, want just the field's SayHello:\n%s", at, got, sc.renderer.ScreenText())
		}
	}
	sc.pressKey(tt, "enter")
	if got := app.current().Snapshot().Payload.(model.GRPC).Method; got != "posting.example.v1.Greeter/SayHello" {
		tt.Fatalf("enter replaced the tab's method with %s", got)
	}
}

func TestGRPCAuthOffersNoDigest(tt *testing.T) {
	digest := model.HTTPKind.Example()
	digest.Auth = model.Auth{Type: model.AuthDigest, Username: "ada", Password: "secret"}
	for _, c := range []struct {
		name   string
		open   func(*App)
		digest bool
		hint   string
		auth   model.AuthType
	}{
		{"HTTP", func(a *App) { a.openRequest(digest) }, true, "The Authorization header is generated when the request is sent.", model.AuthDigest},
		{"gRPC", func(a *App) { a.openRequest(grpcRequest()) }, false, "The authorization metadata is generated when the request is sent.", model.AuthBearer},
		{"HTTP with digest made gRPC", func(a *App) { a.openRequest(digest); a.setKind(model.KindGRPC) }, false, "No authentication", model.AuthNone},
	} {
		app := testApp()
		c.open(app)
		app.current().selectRequestTab("auth")
		sc := newScreen(app, snapW, snapH)
		if sc.shows("Digest") != c.digest || !sc.shows(c.hint) {
			tt.Errorf("%s: Digest offered = %v, want %v; want %q on screen:\n%s", c.name, sc.shows("Digest"), c.digest, c.hint, sc.renderer.ScreenText())
		}
		if got := app.current().Snapshot().Auth.Type; got != c.auth {
			tt.Errorf("%s: auth sent is %s, want %s", c.name, got, c.auth)
		}
	}
}

func TestGRPCPaletteCommands(tt *testing.T) {
	labels := func(app *App) []string {
		var out []string
		for _, item := range app.paletteItems() {
			out = append(out, item.Label)
		}
		return out
	}
	app := testApp()
	app.openRequest(grpcRequest())
	got := strings.Join(labels(app), "|")
	for _, want := range []string{"Refresh gRPC methods", "Insert gRPC message template", "Export as grpcurl"} {
		if !strings.Contains(got, want) {
			tt.Errorf("a gRPC request's palette lacks %q: %s", want, got)
		}
	}

	app = testApp()
	got = strings.Join(labels(app), "|")
	if strings.Contains(got, "gRPC") || !strings.Contains(got, "Export as curl") {
		tt.Errorf("an HTTP request's palette: %s", got)
	}
}

func TestGRPCInsertTemplateCommandWithoutAMethodWarns(tt *testing.T) {
	app := testApp()
	app.openRequest(grpcRequest())
	for _, item := range app.paletteItems() {
		if item.Label == "Insert gRPC message template" {
			item.Action()
		}
	}
	if got := app.toast.Peek(); got.message != "Choose a method from the list first" {
		tt.Fatalf("toast = %+v", got)
	}
}

func TestCopyGRPCAsGrpcurl(tt *testing.T) {
	app := testApp()
	req := grpcRequest()
	req.Payload = model.GRPC{Method: "posting.example.v1.Greeter/SayHello", Message: `{"name": "Ada"}`}
	app.openRequest(req)
	command, err := app.curlCommand(true)
	if err != nil {
		tt.Fatal(err)
	}
	for _, want := range []string{"grpcurl", "-plaintext", "-H 'x-tenant: core'", "-H 'authorization: Bearer dev-token-123'", `-d '{"name": "Ada"}'`, "localhost:50051 posting.example.v1.Greeter/SayHello"} {
		if !strings.Contains(command, want) {
			tt.Errorf("grpcurl command lacks %q:\n%s", want, command)
		}
	}
	app.openCurlExport()
	if sc := newScreen(app, snapW, snapH); !sc.shows("Export as grpcurl") {
		tt.Fatalf("the export dialog should name grpcurl:\n%s", sc.renderer.ScreenText())
	}
}

func TestSendingGRPCShowsItsStatusAndTrailers(tt *testing.T) {
	app := testApp()
	req := grpcRequest()
	req.Payload = model.GRPC{Method: "posting.example.v1.Greeter/FindMissing", Message: `{"name": "Ada"}`}
	app.openRequest(req)
	s := app.current()
	s.responseTab.Set("cookies")
	updates := make(chan func(), 32)
	s.dispatch = func(fn func()) { updates <- fn }
	app.send()
	for s.phase.Peek() == exchangeSending {
		(<-updates)()
	}
	if s.err.Peek() != nil {
		tt.Fatal(s.err.Peek())
	}
	if got := s.responseStatus; got != (model.Status{Code: "NOT_FOUND", Text: "no such thing", Class: model.StatusClassError}) {
		tt.Fatalf("status = %+v", got)
	}
	var tabs []string
	for _, tab := range s.responseTabs(s.response.Peek()).Tabs {
		tabs = append(tabs, tab.Label+tab.Badge)
	}
	if want := []string{"Body", "Headers2", "Trailers1", "Trace"}; !reflect.DeepEqual(tabs, want) {
		tt.Fatalf("response tabs = %v, want %v", tabs, want)
	}
	if s.responseTab.Peek() != "body" {
		tt.Fatalf("a gRPC response has no cookies tab, yet %q is shown", s.responseTab.Peek())
	}
	history := app.history.Peek()
	if len(history) != 1 || history[0].Status.Code != "NOT_FOUND" || history[0].Request.Kind() != model.GRPCKind {
		tt.Fatalf("history = %+v", history)
	}

	s.responseTab.Set("trailers")
	sc := newScreen(app, snapW, snapH)
	if !sc.shows("NOT_FOUND no such thing") || !sc.shows("grpc-status") {
		tt.Fatalf("the status and trailers should be on screen:\n%s", sc.renderer.ScreenText())
	}
	var jumps []string
	for _, target := range app.jumpTargets() {
		if strings.HasPrefix(target.ID, responseTabsID) {
			jumps = append(jumps, target.Key+"="+target.ID)
		}
	}
	if want := []string{"a=" + responseTabsID, "s=" + tabID(responseTabsID, "headers"), "d=" + tabID(responseTabsID, "trailers"), "f=" + tabID(responseTabsID, "trace")}; !reflect.DeepEqual(jumps, want) {
		tt.Fatalf("response jumps = %v, want %v", jumps, want)
	}
}

func TestGRPCColourDiffersFromGraphQLs(tt *testing.T) {
	for _, name := range t.ThemeNames() {
		theme, _ := t.GetTheme(name)
		if grpc, gql := requestColor(theme, model.GRPCKind.New()), requestColor(theme, model.GraphQLKind.New()); grpc == gql {
			tt.Errorf("%s: gRPC and GraphQL are both %s", name, grpc.Hex())
		}
	}
}

// grpcSnapshotApp has a gRPC request from the collection open, with its
// methods discovered.
func grpcSnapshotApp(tt *testing.T) *App {
	tt.Helper()
	collection := model.SampleCollection()
	req := grpcRequest()
	req.Name = "Say hello"
	req.File = "users/say-hello.posting.yaml"
	req.Payload = model.GRPC{Method: "posting.example.v1.Greeter/SayHello", Message: "{\n  \"name\": \"${USER_NAME}\"\n}\n"}
	for _, folder := range collection.Children {
		if folder.Name == "users" {
			folder.Requests = append(folder.Requests, req)
		}
	}
	collection.Sort()
	app := New(Config{
		Version:      "3.0.0-dev",
		Collection:   collection,
		Environments: StaticEnvironments(model.SampleEnvironments()),
		Environment:  []string{"local.env"},
		UserHost:     "user@host",
	})
	app.openRequest(req)
	s := app.current()
	updates := make(chan func(), 1)
	s.dispatch = func(fn func()) { updates <- fn }
	newScreen(app, snapW, snapH)
	(<-updates)()
	return app
}

func TestSnapshotGRPC(tt *testing.T) {
	app := grpcSnapshotApp(tt)
	t.AssertSnapshotNamed(tt, "GRPC_message", app, snapW, snapH, "A gRPC request from the collection: RPC badges, the Message tab with the method, its signature under it and the JSON message")

	s := app.current()
	e := s.payloads[model.KindGRPC].(*grpcEditor)
	e.files.SetText("protos/greeter/v1/greeter.proto\nprotos/common.protoset")
	e.imports.SetText("protos")
	s.requestTab.Set("grpc-proto")
	t.AssertSnapshotNamed(tt, "GRPC_proto", app, snapW, snapH, "The gRPC Proto tab: two proto files and an import path")

	app = grpcSnapshotApp(tt)
	s = app.current()
	resp, err := client.Fake{}.Send(context.Background(), client.Call{Request: s.Snapshot(), Variables: map[string]string{"USER_NAME": "Ada"}})
	if err != nil {
		tt.Fatal(err)
	}
	resp.Trace = fixedResponse().Trace
	resp.Elapsed = fixedResponse().Elapsed
	s.sent = s.Snapshot()
	s.showResponse(resp, nil)
	s.phase.Set(exchangeDone)
	s.responseTab.Set("trailers")
	t.AssertSnapshotNamed(tt, "GRPC_response_trailers", app, snapW, snapH, "A gRPC response: OK status chip, Trailers tab in place of Cookies, with grpc-status")
}

package ui

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
)

const (
	grpcMethodID  = "req-grpc-method"
	grpcMessageID = "req-grpc-message"
	grpcFilesID   = "req-grpc-files"
	grpcImportsID = "req-grpc-imports"
)

// grpcEditor edits a gRPC payload: the method, its message and where the
// schema comes from. It keeps the schema it last discovered, so the method
// field can offer what the server has.
type grpcEditor struct {
	session       *Session
	method        *t.TextInputState
	methods       *t.AutocompleteState
	message       *t.TextAreaState
	messageScroll *t.ScrollState
	messageVars   *completion
	files         *t.TextAreaState
	filesScroll   *t.ScrollState
	imports       *t.TextAreaState
	importsScroll *t.ScrollState

	catalog t.AnySignal[catalog]
	// cancel stops the discovery under way, if any.
	cancel context.CancelFunc
	// template is the last template inserted into the message. Picking
	// another method replaces the message only while it is still this, or
	// blank, so a message the user wrote is never lost.
	template string
}

func newGRPCEditor(s *Session) payloadEditor {
	return &grpcEditor{
		session:       s,
		method:        t.NewTextInputState(""),
		methods:       t.NewAutocompleteState(),
		message:       t.NewTextAreaState(""),
		messageScroll: t.NewScrollState(),
		messageVars:   newCompletion(),
		files:         t.NewTextAreaState(""),
		filesScroll:   t.NewScrollState(),
		imports:       t.NewTextAreaState(""),
		importsScroll: t.NewScrollState(),
		catalog:       t.NewAnySignal(catalog{}),
	}
}

func (e *grpcEditor) load(req model.Request) {
	g, _ := req.Payload.(model.GRPC)
	e.method.SetText(g.Method)
	e.message.SetText(g.Message)
	e.files.SetText(strings.Join(g.Protos.Files, "\n"))
	e.imports.SetText(strings.Join(g.Protos.ImportPaths, "\n"))
	for _, area := range []*t.TextAreaState{e.message, e.files, e.imports} {
		area.ClearSelection()
		area.CursorIndex.Set(0)
	}
	e.messageScroll.SetOffset(0)
	e.template = ""
	e.offerMethods()
}

func (e *grpcEditor) payload() model.Payload {
	return model.GRPC{
		Method:  e.method.GetText(),
		Message: e.message.GetText(),
		Protos:  model.ProtoSet{Files: lines(e.files.GetText()), ImportPaths: lines(e.imports.GetText())},
	}
}

// lines are text's non-blank lines, trimmed. None is nil.
func lines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func (e *grpcEditor) tabs() []requestTab {
	return []requestTab{
		{key: "grpc-message", label: "Message", jump: "i", marked: func() bool { return hasText(e.message) }},
		{key: "grpc-proto", label: "Proto", jump: "o", marked: func() bool { return hasText(e.files) }},
	}
}

func (e *grpcEditor) view(tab string, a *App) t.Widget {
	if tab == "grpc-proto" {
		return grpcProtoView{app: a, editor: e}
	}
	return grpcMessageView{app: a, editor: e}
}

func (e *grpcEditor) focusID(tab string) string {
	switch tab {
	case "grpc-message":
		return grpcMethodID
	case "grpc-proto":
		return grpcFilesID
	}
	return ""
}

// catalogPhase is where discovering a schema is up to.
type catalogPhase uint8

const (
	// catalogIdle is before anything asked for the methods.
	catalogIdle catalogPhase = iota
	// catalogNoSource is a request with neither an address nor proto files.
	catalogNoSource
	catalogLoading
	catalogReady
	catalogFailed
)

// catalog is the schema the editor discovered. key is what it was
// discovered from: the proto set, or the server with the metadata and auth
// it was asked with. A result for a key the request has since moved on from
// is dropped.
type catalog struct {
	phase  catalogPhase
	key    string
	source schemaSource
	schema client.Schema
	err    error
}

// schemaSource is where a catalog's schema comes from, for its status line.
type schemaSource struct {
	address string
	tls     bool
	// files is the number of proto files. None means reflection.
	files int
}

func (s schemaSource) String() string {
	if s.files > 0 {
		return pluralize(s.files, "proto file")
	}
	return s.address
}

// describe discovers the request's methods. Events call it, such as moving
// to the method field, and never a Build, so opening a request never
// contacts its server. The server is asked again when force is set, when
// the request would ask it differently, or when asking it failed; proto
// files are read every time, which is cheap, so an edited file is seen.
// The work happens on a goroutine; its result lands on the UI goroutine.
func (e *grpcEditor) describe(a *App, force bool) {
	s := e.session
	req := s.Snapshot()
	if req.Kind() != model.GRPCKind {
		return
	}
	variables, variablesErr := a.requestVariableValues(req)
	key, source, err := catalogKey(req, variables)
	if err == nil && source.files == 0 {
		err = variablesErr
	}
	current := e.catalog.Peek()
	if !force && key != "" && source.files == 0 && current.key == key && current.phase != catalogFailed {
		return
	}
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
	switch {
	case key == "":
		e.setCatalog(catalog{phase: catalogNoSource})
		return
	case err != nil:
		e.setCatalog(catalog{phase: catalogFailed, key: key, source: source, err: err})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	// Reading the same proto files again keeps their methods on screen.
	if force || current.key != key || current.phase != catalogReady {
		e.catalog.Set(catalog{phase: catalogLoading, key: key, source: source, schema: current.schema})
	}
	describer, dispatch := a.describer, s.dispatch
	go func() {
		schema, err := describer.Describe(ctx, client.Call{Request: req, Variables: variables})
		dispatch(func() {
			if ctx.Err() != nil || e.catalog.Peek().key != key {
				return
			}
			cancel()
			e.cancel = nil
			if err != nil {
				e.setCatalog(catalog{phase: catalogFailed, key: key, source: source, err: err})
				return
			}
			e.setCatalog(catalog{phase: catalogReady, key: key, source: source, schema: schema})
			// Discovery usually starts as the method field takes focus, when
			// there is nothing to list yet, so the list opens once there is.
			// It stays hidden while the field doesn't have focus.
			e.methods.Show()
		})
	}()
}

// listProtoMethods discovers s's methods when they come from proto files,
// which never contacts the server, so it runs as soon as a request loads.
func (a *App) listProtoMethods(s *Session) {
	if e := s.payloads[model.KindGRPC].(*grpcEditor); hasText(e.files) {
		e.describe(a, false)
	}
}

func (e *grpcEditor) setCatalog(c catalog) {
	e.catalog.Set(c)
	e.offerMethods()
}

// offerMethods lists the methods matching the whole of the method field:
// the one it names, or those it fuzzily matches. The list itself filters
// only by the text before the cursor, which sits at the start of a loaded
// method, so on its own it would offer every method and enter would swap
// the method for the first.
func (e *grpcEditor) offerMethods() {
	schema, text := e.catalog.Peek().schema, e.method.GetText()
	var methods []client.Method
	if m, ok := e.chosen(schema, text); ok {
		methods = append(methods, m)
	} else {
		for _, m := range schema.Methods {
			if t.MatchString(m.Name, text, t.FilterOptions{Mode: t.FilterFuzzy}).Matched {
				methods = append(methods, m)
			}
		}
	}
	e.methods.SetSuggestions(methodSuggestions(methods))
}

// catalogKey names the source req's schema comes from: its proto set, or the
// server its address resolves to with a fingerprint of the metadata and
// auth it would be asked with, so fixing a token asks again. It is "" when
// there is no source yet.
func catalogKey(req model.Request, variables map[string]string) (string, schemaSource, error) {
	g := req.Payload.(model.GRPC)
	if !g.Protos.Reflection() {
		key := "protos\x00" + strings.Join(g.Protos.Files, "\x00") + "\x01" + strings.Join(g.Protos.ImportPaths, "\x00")
		return key, schemaSource{files: len(g.Protos.Files)}, nil
	}
	if strings.TrimSpace(req.URL) == "" {
		return "", schemaSource{}, nil
	}
	resolved, err := model.Resolve(req, model.MapLookup(variables))
	if err != nil {
		return "unresolved\x00" + req.URL, schemaSource{address: req.URL}, err
	}
	target, err := model.ParseGRPCTarget(resolved.URL)
	if err != nil {
		return "invalid\x00" + resolved.URL, schemaSource{address: resolved.URL}, err
	}
	credentials := sha256.New()
	for _, h := range resolved.Headers {
		if h.Enabled {
			fmt.Fprintf(credentials, "%s\x00%s\x00", strings.ToLower(strings.TrimSpace(h.Name)), h.Value)
		}
	}
	auth := resolved.Auth
	fmt.Fprintf(credentials, "%s\x00%s\x00%s\x00%s", auth.Type, auth.Username, auth.Password, auth.Token)
	key := fmt.Sprintf("server\x00%s\x00%v\x00%v\x00%x", target.Authority, target.TLS, req.Options.VerifySSL, credentials.Sum(nil))
	return key, schemaSource{address: target.Authority, tls: target.TLS}, nil
}

// pick chooses method m, filling in its message template when the message
// is blank or still the last template.
func (e *grpcEditor) pick(m client.Method) {
	e.method.SetText(m.Name)
	e.method.CursorEnd()
	if current := e.message.GetText(); strings.TrimSpace(current) == "" || current == e.template {
		e.setMessage(m.Template)
	}
	e.session.touch()
}

// insertTemplate replaces the message with the chosen method's template.
// It reports false when the method isn't one the catalog knows.
func (e *grpcEditor) insertTemplate() bool {
	m, ok := e.chosen(e.catalog.Peek().schema, e.method.GetText())
	if !ok {
		return false
	}
	e.setMessage(m.Template)
	e.session.touch()
	return true
}

func (e *grpcEditor) setMessage(text string) {
	e.message.SetText(text)
	e.message.ClearSelection()
	e.message.CursorIndex.Set(0)
	e.messageScroll.SetOffset(0)
	e.template = text
}

// chosen is the schema's method named by text, written either as
// "pkg.Service/Method" or as "pkg.Service.Method".
func (e *grpcEditor) chosen(schema client.Schema, text string) (client.Method, bool) {
	name := strings.TrimPrefix(strings.TrimSpace(text), "/")
	if !strings.Contains(name, "/") {
		if i := strings.LastIndexByte(name, '.'); i >= 0 {
			name = name[:i] + "/" + name[i+1:]
		}
	}
	for _, m := range schema.Methods {
		if m.Name == name {
			return m, true
		}
	}
	return client.Method{}, false
}

// methodSuggestions offer methods, each described by its shape and types:
// "unary · GetUserRequest → User".
func methodSuggestions(methods []client.Method) []t.Suggestion {
	out := make([]t.Suggestion, len(methods))
	for i, m := range methods {
		out[i] = t.Suggestion{Label: m.Name, Value: m.Name, Description: signature(m), Data: m}
	}
	return out
}

func signature(m client.Method) string {
	return m.Streaming.String() + " · " + shortName(m.Input) + " → " + shortName(m.Output)
}

// shortName is a message's name without its package.
func shortName(full string) string {
	return full[strings.LastIndexByte(full, '.')+1:]
}

// grpcMessageView is the Message tab: the method, a line about the schema,
// then the message as JSON.
type grpcMessageView struct {
	fillParent
	app    *App
	editor *grpcEditor
}

func (v grpcMessageView) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	e, s, a := v.editor, v.editor.session, v.app
	substitute := s.substitute.Checked.Get()
	var area t.Widget = t.TextArea{
		ID:            grpcMessageID,
		State:         e.message,
		ScrollState:   e.messageScroll,
		Placeholder:   "Choose a method to fill in its message, or write it as JSON…",
		Highlighter:   a.bodyHighlighter(theme, "json", substitute),
		Style:         t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
		OnChange:      func(string) { s.touch() },
		ExtraKeybinds: a.externalKeybinds(e.message, "json", s.touch),
	}
	if substitute {
		area = e.messageVars.wrap(theme, area, a.variableChoices(), t.Flex(1))
	}
	method := input{
		ID:            grpcMethodID,
		State:         e.method,
		Placeholder:   "package.Service/Method",
		OnChange:      func(string) { e.offerMethods(); s.touch() },
		Suggestions:   e.methods,
		SuggestAlways: true,
		SuggestWidth:  t.Cells(96),
		OnSuggestion: func(choice t.Suggestion) {
			if m, ok := choice.Data.(client.Method); ok {
				e.pick(m)
			}
		},
		Keybinds: []t.Keybind{{Key: "ctrl+r", Name: "Refresh methods", Action: func() { e.describe(a, true) }}},
	}
	return t.Column{
		Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Spacing: a.gap(),
		Children: []t.Widget{
			t.Column{Style: t.Style{Width: t.Flex(1)}, Children: []t.Widget{
				formRow(ctx, "Method", "", method),
				grpcCatalogLine{editor: e},
			}},
			scrollingArea(grpcMessageID, e.messageScroll, theme.Surface, area),
		},
	}
}

// grpcCatalogLine sits under the method: what discovery found, the chosen
// method's shape, or why discovery failed. It rebuilds as the method is
// typed, so the view above it doesn't have to.
type grpcCatalogLine struct {
	fillWidth
	editor *grpcEditor
}

func (l grpcCatalogLine) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	e := l.editor
	c := e.catalog.Get()
	method := textOf(e.method)
	color, text := theme.TextMuted, ""
	switch c.phase {
	case catalogIdle:
		text = "Move to this field to list the server's methods"
	case catalogNoSource:
		text = "Enter the server's address to list its methods, or add proto files on the Proto tab"
	case catalogLoading:
		text = "Loading methods from " + c.source.String() + "…"
	case catalogFailed:
		color, text = theme.ErrorText, c.err.Error()+". Press ctrl+r to try again"
	case catalogReady:
		if m, ok := e.chosen(c.schema, method); ok {
			text = signature(m)
			break
		}
		text = pluralize(len(c.schema.Methods), "method")
		if c.source.files > 0 {
			text += " from " + c.source.String()
		} else {
			text += " via reflection · " + map[bool]string{true: "TLS", false: "plaintext"}[c.source.tls]
		}
		if strings.TrimSpace(method) != "" {
			color, text = theme.WarningText, "Not one of the "+text
		}
	}
	return t.Text{
		Content: text,
		Wrap:    t.WrapSoft,
		Style:   t.Style{Width: t.Flex(1), ForegroundColor: color, Padding: t.EdgeInsets{Left: formLabelWidth + 1}},
	}
}

// grpcProtoView is the Proto tab: the schema's files and import paths, one
// per line.
type grpcProtoView struct {
	fillParent
	app    *App
	editor *grpcEditor
}

func (v grpcProtoView) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	e, s := v.editor, v.editor.session
	paths := func(id string, state *t.TextAreaState, scroll *t.ScrollState, label, placeholder string) t.Widget {
		return t.Row{
			Style:      t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			CrossAlign: t.CrossAxisStretch,
			Children: []t.Widget{
				formLabel([]t.Span{{Text: label, Style: t.SpanStyle{Foreground: theme.Text, Bold: true}}}),
				scrollingArea(id, scroll, theme.Surface, t.TextArea{
					ID:          id,
					State:       state,
					ScrollState: scroll,
					Placeholder: placeholder,
					Style:       t.Style{Width: t.Flex(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)},
					OnChange:    func(string) { s.touch() },
				}),
			},
		}
	}
	return t.Column{
		Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Spacing: v.app.gap(),
		Children: []t.Widget{
			t.Text{
				Content: "Leave these empty to ask the server for its methods with reflection. Paths are relative to the collection, one per line; .protoset, .binpb and .pb files are descriptor sets.",
				Wrap:    t.WrapSoft,
				Style:   t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted, Padding: inset},
			},
			paths(grpcFilesID, e.files, e.filesScroll, "Files", "protos/users/v1/users.proto"),
			paths(grpcImportsID, e.imports, e.importsScroll, "Import paths", "protos (default: the collection)"),
		},
	}
}

// grpcCommands are the palette's commands for a gRPC request.
func grpcCommands(a *App, s *Session) []t.CommandPaletteItem {
	e := s.payloads[model.KindGRPC].(*grpcEditor)
	return []t.CommandPaletteItem{
		{Label: "Refresh gRPC methods", Description: "Ask the server or proto files for the methods again", Action: a.run(func() {
			e.describe(a, true)
		})},
		{Label: "Insert gRPC message template", Description: "Replace the message with one for the chosen method", Action: a.run(func() {
			if e.insertTemplate() {
				s.requestTab.Set("grpc-message")
				return
			}
			a.notify("Choose a method from the list first", toastWarning)
		})},
	}
}

// Package ui is Posting's terminal interface, built with Terma.
//
// The UI never performs I/O itself. Requests are sent through the
// client.Sender passed in Config; collections and environments arrive as
// model values. Swapping the fake sender for a real HTTP client requires no
// changes in this package.
package ui

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"strings"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
)

// Config is everything the UI needs from the outside world.
type Config struct {
	Version      string
	Sender       client.Sender
	Collection   *model.Collection
	Environments []model.Environment
	// Theme is a Terma theme name. Empty uses galaxy.
	Theme string
	// UserHost is shown in the header. Empty uses the current user and host.
	UserHost string
}

// layoutMode arranges the request and response panels.
type layoutMode string

const (
	layoutVertical   layoutMode = "vertical"
	layoutHorizontal layoutMode = "horizontal"
)

// App is the root widget.
type App struct {
	version string
	sender  client.Sender
	host    string

	collection t.AnySignal[*model.Collection]
	tree       *t.TreeState[treeItem]
	treeScroll *t.ScrollState

	history       t.AnySignal[[]model.HistoryEntry]
	historyList   *t.ListState[model.HistoryEntry]
	historyScroll *t.ScrollState
	nextHistoryID int64

	environments []model.Environment
	activeEnv    t.Signal[int] // index into environments, -1 for none
	sessionVars  t.AnySignal[map[string]string]

	sessions      t.AnySignal[[]*Session]
	active        t.Signal[int] // Session.id of the visible session
	nextSessionID int

	sidebarVisible t.Signal[bool]
	sidebarTab     t.Signal[string]
	sidebarSplit   *t.SplitPaneState
	panelSplit     *t.SplitPaneState // between the request and response
	layout         t.Signal[layoutMode]
	expanded       t.Signal[string] // "", "request" or "response"
	// compact drops the blank rows between parts of the layout when the
	// terminal is too short to spare them (see heightProbe).
	compact t.Signal[bool]

	jump               *t.JumpState
	palette            *t.CommandPaletteState
	requestSearch      *t.CommandPaletteState
	themeBeforePreview string
	helpScroll         *t.ScrollState
	highlighters       map[string]*syntaxHighlighter
	methodMenu         *t.MenuState
	menuOpen           t.Signal[bool]
	overlay            t.Signal[string] // "", "help", "variables", "save", "confirm"
	confirm            t.AnySignal[confirmation]
	save               *saveForm
	variables          *variablesForm
	toast              t.AnySignal[toast]
	toastSeq           int
}

// New creates the root widget.
func New(cfg Config) *App {
	if cfg.Sender == nil {
		cfg.Sender = client.Fake{}
	}
	if cfg.Collection == nil {
		cfg.Collection = &model.Collection{Name: "collection"}
	}
	if cfg.UserHost == "" {
		cfg.UserHost = userHost()
	}
	a := &App{
		version:        cfg.Version,
		sender:         cfg.Sender,
		host:           cfg.UserHost,
		collection:     t.NewAnySignal(cfg.Collection),
		treeScroll:     t.NewScrollState(),
		history:        t.NewAnySignal[[]model.HistoryEntry](nil),
		historyList:    t.NewListState[model.HistoryEntry](nil),
		historyScroll:  t.NewScrollState(),
		environments:   cfg.Environments,
		activeEnv:      t.NewSignal(-1),
		sessionVars:    t.NewAnySignal(map[string]string{}),
		sessions:       t.NewAnySignal[[]*Session](nil),
		active:         t.NewSignal(0),
		sidebarVisible: t.NewSignal(true),
		sidebarTab:     t.NewSignal("requests"),
		sidebarSplit:   t.NewSplitPaneState(0.28),
		panelSplit:     t.NewSplitPaneState(0.5),
		layout:         t.NewSignal(layoutVertical),
		expanded:       t.NewSignal(""),
		compact:        t.NewSignal(false),
		helpScroll:     t.NewScrollState(),
		menuOpen:       t.NewSignal(false),
		overlay:        t.NewSignal(""),
		confirm:        t.NewAnySignal(confirmation{}),
		toast:          t.NewAnySignal(toast{}),
	}
	if len(cfg.Environments) > 0 {
		a.activeEnv.Set(0)
	}
	a.tree = t.NewTreeState(buildTree(cfg.Collection))
	a.jump = t.NewJumpState()
	a.palette = t.NewCommandPaletteState("Commands", nil)
	a.requestSearch = t.NewCommandPaletteState("Go to request", nil)
	a.methodMenu = t.NewMenuState(a.methodMenuItems())
	a.save = newSaveForm()
	a.variables = newVariablesForm()
	a.openSession(model.NewRequest())

	theme := cfg.Theme
	if theme == "" {
		theme = t.ThemeNameGalaxy
	}
	t.SetTheme(theme)
	t.RequestFocus(urlInputID)
	return a
}

// Run starts the UI and blocks until it exits.
func Run(cfg Config) error {
	return t.Run(New(cfg))
}

func userHost() string {
	name := "posting"
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	host, _ := os.Hostname()
	host, _, _ = strings.Cut(host, ".")
	if host == "" {
		return name
	}
	return name + "@" + host
}

// ---------------------------------------------------------------------------
// Sessions (open request tabs)

// session returns the visible session, subscribing to changes.
func (a *App) session() *Session {
	id := a.active.Get()
	for _, s := range a.sessions.Get() {
		if s.id == id {
			return s
		}
	}
	return nil
}

// current returns the visible session without subscribing.
func (a *App) current() *Session {
	id := a.active.Peek()
	for _, s := range a.sessions.Peek() {
		if s.id == id {
			return s
		}
	}
	return nil
}

func (a *App) openSession(req model.Request) *Session {
	a.nextSessionID++
	s := newSession(a.nextSessionID, req)
	a.sessions.Set(append(append([]*Session(nil), a.sessions.Peek()...), s))
	a.active.Set(s.id)
	return s
}

// openRequest shows req, reusing a tab that already holds the same file or
// replacing an untouched blank tab.
func (a *App) openRequest(req model.Request) {
	for _, s := range a.sessions.Peek() {
		if req.File != "" && s.file.Peek() == req.File {
			a.active.Set(s.id)
			return
		}
	}
	if s := a.current(); s != nil && s.isPristine() {
		s.Load(req)
		return
	}
	a.openSession(req)
}

func (s *Session) isPristine() bool {
	return !s.dirty.Peek() && s.file.Peek() == "" && s.url.GetText() == "" && s.response.Peek() == nil
}

func (a *App) newTab() {
	a.openSession(model.NewRequest())
	t.RequestFocus(urlInputID)
}

func (a *App) closeSession(id int) {
	sessions := a.sessions.Peek()
	index := -1
	for i, s := range sessions {
		if s.id == id {
			index = i
			s.Cancel()
		}
	}
	if index < 0 {
		return
	}
	next := append(append([]*Session(nil), sessions[:index]...), sessions[index+1:]...)
	if len(next) == 0 {
		a.sessions.Set(nil)
		a.openSession(model.NewRequest())
		return
	}
	a.sessions.Set(next)
	if a.active.Peek() == id {
		a.active.Set(next[min(index, len(next)-1)].id)
	}
}

func (a *App) cycleSession(delta int) {
	sessions := a.sessions.Peek()
	for i, s := range sessions {
		if s.id == a.active.Peek() {
			a.active.Set(sessions[(i+delta+len(sessions))%len(sessions)].id)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Variables and environments

// variableValues merges the active environment with session overrides.
// Reading it in Build subscribes to both.
func (a *App) variableValues() map[string]string {
	values := map[string]string{}
	if env := a.activeEnv.Get(); env >= 0 && env < len(a.environments) {
		for _, v := range a.environments[env].Variables {
			values[v.Name] = v.Value
		}
	}
	for name, value := range a.sessionVars.Get() {
		values[name] = value
	}
	return values
}

func sortVariables(vars []model.Variable) {
	sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
}

func (a *App) resolver() func(string) bool {
	values := a.variableValues()
	return func(name string) bool { _, ok := values[name]; return ok }
}

func (a *App) envName() string {
	env := a.activeEnv.Get()
	if env < 0 || env >= len(a.environments) {
		return ""
	}
	return a.environments[env].Name
}

func (a *App) switchEnvironment(index int) {
	a.activeEnv.Set(index)
	if index < 0 {
		a.notify("Environment cleared", toastInfo)
		return
	}
	a.notify("Switched to "+a.environments[index].Name, toastInfo)
}

// ---------------------------------------------------------------------------
// Actions

func (a *App) send() {
	s := a.current()
	if s == nil {
		return
	}
	if strings.TrimSpace(s.url.GetText()) == "" {
		a.notify("Enter a URL before sending", toastWarning)
		t.RequestFocus(urlInputID)
		return
	}
	s.Send(a.sender, a.variableValuesPeek(), func(req model.Request, resp *model.Response) {
		a.nextHistoryID++
		entry := historyEntry(a.nextHistoryID, req, resp)
		history := append([]model.HistoryEntry{entry}, a.history.Peek()...)
		if len(history) > 100 {
			history = history[:100]
		}
		a.history.Set(history)
		a.historyList.SetItems(history)
	})
}

// variableValuesPeek is variableValues without subscribing (for actions).
func (a *App) variableValuesPeek() map[string]string {
	values := map[string]string{}
	if env := a.activeEnv.Peek(); env >= 0 && env < len(a.environments) {
		for _, v := range a.environments[env].Variables {
			values[v.Name] = v.Value
		}
	}
	for name, value := range a.sessionVars.Peek() {
		values[name] = value
	}
	return values
}

func (a *App) cancelSend() {
	if s := a.current(); s != nil && s.phase.Peek() == exchangeSending {
		s.Cancel()
		a.notify("Request cancelled", toastWarning)
	}
}

func (a *App) setMethod(m model.Method) {
	if s := a.current(); s != nil && s.method.Peek() != m {
		s.method.Set(m)
		s.touch()
	}
}

func (a *App) openMethodMenu() {
	a.menuOpen.Set(true)
	t.RequestFocus(methodMenuID)
}

func (a *App) closeMethodMenu() {
	a.menuOpen.Set(false)
	t.RequestFocus(methodSelectorID)
}

func (a *App) methodMenuItems() []t.MenuItem {
	items := make([]t.MenuItem, 0, len(model.Methods))
	for _, m := range model.Methods {
		items = append(items, t.MenuItem{Label: string(m), Shortcut: methodHotkeys[m]})
	}
	return items
}

// saveRequest writes the current request into the collection, asking for a
// name and folder the first time.
func (a *App) saveRequest() {
	s := a.current()
	if s == nil {
		return
	}
	if s.file.Peek() == "" {
		a.save.prefill(s.Snapshot(), a.cursorFolder())
		a.overlay.Set("save")
		return
	}
	req := s.Snapshot()
	a.storeRequest(req)
	s.dirty.Set(false)
	a.notify("Saved "+req.File, toastSuccess)
}

// storeRequest upserts req into the in-memory collection by its File.
func (a *App) storeRequest(req model.Request) {
	root := a.collection.Peek()
	folderPath, _ := splitFile(req.File)
	folder := ensureFolder(root, folderPath)
	replaced := false
	for i := range folder.Requests {
		if folder.Requests[i].File == req.File {
			folder.Requests[i] = req.Clone()
			replaced = true
		}
	}
	if !replaced {
		folder.Requests = append(folder.Requests, req.Clone())
	}
	root.Sort()
	a.refreshTree()
}

func (a *App) deleteRequest(file string) {
	root := a.collection.Peek()
	folderPath, _ := splitFile(file)
	folder := findFolder(root, folderPath)
	if folder == nil {
		return
	}
	kept := folder.Requests[:0]
	for _, r := range folder.Requests {
		if r.File != file {
			kept = append(kept, r)
		}
	}
	folder.Requests = kept
	for _, s := range a.sessions.Peek() {
		if s.file.Peek() == file {
			s.file.Set("")
			s.dirty.Set(true)
		}
	}
	a.refreshTree()
	a.notify("Deleted "+file, toastInfo)
}

func (a *App) refreshTree() {
	a.collection.Set(a.collection.Peek())
	a.tree.Nodes.Set(buildTree(a.collection.Peek()))
}

// toggleExpand maximises a panel, or restores both if one is maximised.
// The panels bind the same key so the focused one is the one expanded.
func (a *App) toggleExpand(panel string) {
	if a.expanded.Peek() != "" {
		a.expanded.Set("")
		return
	}
	a.expanded.Set(panel)
}

func (a *App) toggleLayout() {
	if a.layout.Peek() == layoutVertical {
		a.layout.Set(layoutHorizontal)
	} else {
		a.layout.Set(layoutVertical)
	}
}

func (a *App) toggleSidebar() {
	a.sidebarVisible.Set(!a.sidebarVisible.Peek())
}

func (a *App) copyAsCurl() {
	s := a.current()
	if s == nil {
		return
	}
	lookup := a.variableValuesPeek()
	command := model.Curl(s.Snapshot(), func(name string) (string, bool) { v, ok := lookup[name]; return v, ok })
	t.SetClipboard('c', command)
	a.notify("Copied curl command to clipboard", toastSuccess)
}

func (a *App) focusRequestTab(key string) {
	if s := a.current(); s != nil {
		s.requestTabs().selectKey(key)
		t.RequestFocus(requestTabsID)
	}
}

// ---------------------------------------------------------------------------
// Root widget

// Keybinds are the global shortcuts. Widgets see keys first, so text inputs
// keep their editing keys and these only fire when nothing else claims them.
func (a *App) Keybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "ctrl+j", Name: "Send", Action: a.send},
		{Key: "alt+enter", Name: "Send", Action: a.send, Hidden: true},
		{Key: "escape", Name: "Cancel", Action: a.cancelSend, Hidden: true},
		{Key: "ctrl+t", Name: "Method", Action: a.openMethodMenu, Hidden: true},
		{Key: "ctrl+l", Name: "Focus URL", Action: func() { t.RequestFocus(urlInputID) }, Hidden: true},
		{Key: "ctrl+s", Name: "Save", Action: a.saveRequest},
		{Key: "ctrl+n", Name: "New tab", Action: a.newTab},
		{Key: "alt+w", Name: "Close tab", Action: func() { a.closeSession(a.active.Peek()) }, Hidden: true},
		{Key: "alt+right", Name: "Next tab", Action: func() { a.cycleSession(1) }, Hidden: true},
		{Key: "alt+left", Name: "Prev tab", Action: func() { a.cycleSession(-1) }, Hidden: true},
		{Key: "ctrl+h", Name: "Sidebar", Action: a.toggleSidebar, Hidden: true},
		{Key: "alt+z", Name: "Expand", Action: func() { a.toggleExpand("request") }, Hidden: true},
		{Key: "ctrl+g", Name: "Go to request", Action: a.openRequestSearch, Hidden: true},
		{Key: "ctrl+shift+v", Name: "Variables", Action: a.openVariables, Hidden: true},
		{Key: "ctrl+p", Name: "Commands", Action: a.openPalette},
		{Key: "f1", Name: "Help", Action: func() { a.overlay.Set("help") }},
	}
}

func (a *App) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	var main t.Widget = t.Dock{
		Style:  t.Style{BackgroundColor: theme.Background},
		Top:    []t.Widget{header{app: a}},
		Bottom: []t.Widget{footer{app: a}},
		Body: t.Stack{
			Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			Children: []t.Widget{
				heightProbe{app: a},
				t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1), Padding: t.EdgeInsets{Right: 1}}, Children: []t.Widget{workspace{app: a}}},
			},
		},
	}
	// The sidebar runs the full height of the app, beside the header and
	// footer rather than between them.
	if a.sidebarVisible.Get() {
		main = t.SplitPane{
			ID:           sidebarSplitID,
			State:        a.sidebarSplit,
			First:        sidebar{app: a},
			Second:       main,
			MinPaneSize:  24,
			DisableFocus: true,
		}
	}

	return t.Jumper{
		State:   a.jump,
		Targets: a.jumpTargets(),
		Dynamic: true,
		Child: t.Column{
			Style:    t.Style{Width: t.Flex(1), Height: t.Flex(1), BackgroundColor: theme.Background},
			Children: []t.Widget{main, overlays{app: a}},
		},
	}
}

// jumpTargets is Posting's jump map: fixed keys for the landmarks of the
// layout, the same as Posting 2's so they stay in muscle memory. Everything
// else on screen (collection rows, history entries, request tabs, fields)
// gets a dynamic hint from the letters left over.
func (a *App) jumpTargets() []t.JumpTarget {
	targets := []t.JumpTarget{
		{Key: "1", ID: methodSelectorID},
		{Key: "2", ID: urlInputID},
	}
	targets = append(targets, tabJumps(sidebarTabsID, "34", []string{"requests", "history"}, a.sidebarTab.Set)...)
	targets = append(targets, tabJumps(requestTabsID, "qwertyui", []string{"headers", "body", "path", "query", "auth", "info", "scripts", "options"}, func(key string) {
		if s := a.current(); s != nil {
			s.requestTabs().selectKey(key)
		}
	})...)
	targets = append(targets, tabJumps(responseTabsID, "asdfg", []string{"body", "headers", "cookies", "scripts", "trace"}, func(key string) {
		if s := a.current(); s != nil && s.response.Peek() != nil {
			s.responseTabs(s.response.Peek()).selectKey(key)
		}
	})...)
	return targets
}

// tabJumps maps keys to the tabs of a strip: each jump selects its tab and
// focuses the strip. The first tab is labelled through the strip itself,
// which starts at the same cell; that claims the strip, so it doesn't get a
// dynamic hint of its own on top of the tabs'.
func tabJumps(stripID, keys string, tabs []string, selectTab func(string)) []t.JumpTarget {
	targets := make([]t.JumpTarget, 0, len(tabs))
	for i, tab := range tabs {
		id := tabID(stripID, tab)
		if i == 0 {
			id = stripID
		}
		targets = append(targets, t.JumpTarget{
			Key: keys[i : i+1],
			ID:  id,
			Action: func() {
				selectTab(tab)
				t.RequestFocus(stripID)
			},
		})
	}
	return targets
}

// compactHeight is the height of the area between the header and footer
// below which the layout drops its blank separator rows. Below it, stacked
// request and response panels would otherwise show only a line or two each.
const compactHeight = 28

// heightProbe fills the area between the header and footer and switches the
// app to compact spacing when it is short. Terma doesn't tell widgets the
// screen size while they build, so layout reports it instead.
type heightProbe struct{ app *App }

func (p heightProbe) GetContentDimensions() (t.Dimension, t.Dimension) { return t.Flex(1), t.Flex(1) }
func (p heightProbe) Build(t.BuildContext) t.Widget                    { return p }
func (p heightProbe) Render(*t.RenderContext)                          {}

// OnLayout switches spacing for the next frame. Setting the signal here
// directly would be lost: the frame being laid out has already built the
// widgets that read it.
func (p heightProbe) OnLayout(_ t.BuildContext, metrics t.LayoutMetrics) {
	if compact := metrics.Box().Height < compactHeight; compact != p.app.compact.Peek() {
		t.Dispatch(func() { p.app.compact.Set(compact) })
	}
}

// gap is the number of blank rows between parts of the layout: one, or none
// in compact mode.
func (a *App) gap() int {
	if a.compact.Get() {
		return 0
	}
	return 1
}

// footer shows the keybinds available for the focused widget, or how to use
// jump mode while it is active.
type footer struct {
	fillWidth
	app *App
}

func (f footer) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	style := t.Style{ForegroundColor: theme.TextMuted}
	var hints t.Widget = t.KeybindBar{Style: style, FormatKey: t.FormatKeyCaret}
	if f.app.jump.IsActive() {
		text := t.ParseMarkupToText("[b $AccentText]Jump[/]  Type a label to move there  [b $Text]esc[/] cancel", theme)
		text.Style = style
		hints = text
	}
	return t.Row{
		Style: t.Style{Width: t.Flex(1), Height: t.Cells(1), Padding: t.EdgeInsetsXY(2, 0), BackgroundColor: theme.Background},
		Children: []t.Widget{
			hints,
			t.Spacer{},
			t.Text{Content: "Posting " + f.app.version, Style: t.Style{ForegroundColor: theme.TextDisabled}},
		},
	}
}

// header is the one-line title bar.
type header struct {
	fillWidth
	app *App
}

func (h header) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a := h.app
	env := a.envName()
	envLabel := "no environment"
	envColor := theme.TextMuted
	if env != "" {
		envLabel = env
		envColor = theme.AccentText
	}
	return t.Row{
		Style: t.Style{Width: t.Flex(1), Height: t.Cells(1), Padding: t.EdgeInsetsXY(2, 0), BackgroundColor: theme.Background},
		Children: []t.Widget{
			t.Spacer{},
			t.Text{
				Spans: []t.Span{
					{Text: "◆ ", Style: t.SpanStyle{Foreground: envColor}},
					{Text: envLabel, Style: t.SpanStyle{Foreground: envColor, Bold: env != ""}},
				},
				Click: func(t.MouseEvent) { a.openEnvironmentPicker() },
			},
			t.Text{Content: "   " + a.host, Style: t.Style{ForegroundColor: theme.TextMuted}},
		},
	}
}

// workspace holds the open-request tabs, URL bar and request/response panels.
type workspace struct {
	fillParent
	app *App
}

const panelSplitID = "panel-split"

func (w workspace) Build(ctx t.BuildContext) t.Widget {
	a := w.app
	s := a.session()
	if s == nil {
		return t.EmptyWidget{}
	}
	expanded := a.expanded.Get()
	request := requestPanel{app: a, session: s}
	response := responsePanel{app: a, session: s}

	var panels t.Widget
	switch expanded {
	case "request":
		panels = request
	case "response":
		panels = response
	default:
		split := t.SplitPane{
			ID:           panelSplitID,
			State:        a.panelSplit,
			First:        request,
			Second:       response,
			Orientation:  t.SplitVertical,
			MinPaneSize:  4,
			DisableFocus: true,
		}
		if a.layout.Get() == layoutHorizontal {
			split.Orientation = t.SplitHorizontal
			split.MinPaneSize = 30
		}
		panels = split
	}
	return t.Column{
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
		Children: []t.Widget{
			// The panels start a cell further left than the tabs and URL
			// bar, so their headings and tab labels line up with the
			// highlighted boxes above rather than the text inside them.
			t.Column{
				Style:    t.Style{Width: t.Flex(1), Padding: inset},
				Children: []t.Widget{sessionTabs{app: a}, urlBar{app: a, session: s}},
			},
			panels,
		},
	}
}

// notify shows a transient message in the corner.
func (a *App) notify(message string, kind toastKind) {
	a.toastSeq++
	seq := a.toastSeq
	a.toast.Set(toast{message: message, kind: kind, seq: seq})
	time.AfterFunc(3*time.Second, func() {
		t.Dispatch(func() {
			if a.toast.Peek().seq == seq {
				a.toast.Set(toast{})
			}
		})
	})
}

func splitFile(file string) (folder, name string) {
	if i := strings.LastIndexByte(file, '/'); i >= 0 {
		return file[:i], file[i+1:]
	}
	return "", file
}

func findFolder(root *model.Collection, path string) *model.Collection {
	if path == "" {
		return root
	}
	folder := root
	for _, part := range strings.Split(path, "/") {
		var next *model.Collection
		for _, child := range folder.Children {
			if child.Name == part {
				next = child
			}
		}
		if next == nil {
			return nil
		}
		folder = next
	}
	return folder
}

func ensureFolder(root *model.Collection, path string) *model.Collection {
	if path == "" {
		return root
	}
	folder := root
	built := ""
	for _, part := range strings.Split(path, "/") {
		if built == "" {
			built = part
		} else {
			built += "/" + part
		}
		var next *model.Collection
		for _, child := range folder.Children {
			if child.Name == part {
				next = child
			}
		}
		if next == nil {
			next = &model.Collection{Name: part, Path: built}
			folder.Children = append(folder.Children, next)
		}
		folder = next
	}
	return folder
}

func pluralize(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

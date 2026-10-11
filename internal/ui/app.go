// Package ui is Posting's terminal interface, built with Terma.
//
// The UI never performs I/O itself. Requests are sent through the
// client.Sender passed in Config; collections and environments arrive as
// model values. Swapping the fake sender for a real HTTP client requires no
// changes in this package.
package ui

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"slices"
	"strings"
	"sync"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/config"
	"github.com/darrenburns/posting/internal/model"
	"github.com/darrenburns/posting/internal/themes"
)

// Config is everything the UI needs from the outside world.
type Config struct {
	Version string
	Sender  client.Sender
	// Describer lists the methods gRPC requests can call. Nil uses
	// client.Fake, as a nil Sender does.
	Describer  client.Describer
	Collection *model.Collection
	// Store saves and deletes requests. Nil keeps changes in memory only.
	Store collection.Store
	// Watch, when set, is polled for changes to the collection on disk.
	Watch CollectionWatcher
	// Reload, when set, rereads the collection on request.
	Reload CollectionWatcher
	// OpenURL opens a web page in the user's browser, returning once the
	// browser has the page or has failed to open it (see terma.OpenURL).
	OpenURL func(url string) error
	// Environments finds and loads environment files.
	Environments EnvironmentSource
	// Environment is the files of the environment active at startup.
	Environment []string
	// RememberEnvironment records the environment the user switches to, so
	// the next launch can start in it. Switching to none passes no files.
	RememberEnvironment func(files []string)
	// HostVariables are available to every request, below the environment.
	HostVariables []model.Variable
	// WatchEnvironment reloads the active environment when its files change.
	WatchEnvironment bool
	// Settings is the user's configuration. Nil uses the defaults.
	Settings *config.Settings
	// UserHost is shown in the header. Empty uses the current user and host.
	UserHost string
	// UserThemes are themes from the theme directory.
	UserThemes []themes.Theme
	// History keeps sent requests between runs. Nil keeps them for the
	// session only.
	History HistoryStore
	// NerdFonts draws icons from a Nerd Font.
	NerdFonts bool
	// StartupMessages are problems found while loading, shown once the app
	// starts.
	StartupMessages []string
}

// layoutMode arranges the request and response panels.
type layoutMode string

const (
	layoutVertical   layoutMode = "vertical"
	layoutHorizontal layoutMode = "horizontal"
)

// App is the root widget.
type App struct {
	version    string
	settings   config.Settings
	spacing    t.Signal[string]
	openURL    func(string) error
	watcher    CollectionWatcher
	envFile    *envFileForm
	userThemes []string
	icons      iconSet
	sender     client.Sender
	describer  client.Describer
	store      collection.Store
	host       string // markup

	// runExternal runs the editor or pager; tests replace it.
	runExternal func(*exec.Cmd) error
	// after runs fn on the UI goroutine once d has passed; tests replace it.
	after func(d time.Duration, fn func())
	// dispatch runs fn on the UI goroutine; tests replace it.
	dispatch func(fn func())

	// running is closed once the app is running, when remote commands can
	// be dispatched to it (see remote.go).
	running     chan struct{}
	runningOnce sync.Once
	// remoteTab is the tab remote commands open requests that aren't saved
	// in.
	remoteTab int

	collection t.AnySignal[*model.Collection]
	tree       *t.TreeState[treeItem]
	treeScroll *t.ScrollState
	// treeSearch is the search box above the tree, and treeFilter the
	// search the tree is filtered by (see tree_search.go).
	treeSearch *t.TextInputState
	treeFilter *t.FilterState
	summary    summaryState

	history       t.AnySignal[[]model.HistoryEntry]
	historyList   *t.ListState[model.HistoryEntry]
	historyScroll *t.ScrollState
	nextHistoryID int64
	historyStore  *historySaver

	env         *environments
	sessionVars t.AnySignal[map[string]string]

	sessions      t.AnySignal[[]*Session]
	active        t.Signal[int] // Session.id of the visible session
	nextSessionID int
	sessionView   *sessionTabsView // where the strip of open tabs is scrolled to

	sidebarVisible t.Signal[bool]
	sidebarTab     t.Signal[string]
	sidebarSplit   *t.SplitPaneState
	panelSplit     *t.SplitPaneState // between the request and response
	layout         t.Signal[layoutMode]
	expanded       t.Signal[string] // "", "request" or "response"
	// compact drops the blank rows between parts of the layout when the
	// terminal is too short to spare them (see heightProbe).
	compact t.Signal[bool]
	// focus is where focus was when focusProbe last reported it.
	focus focusAt

	jump               *t.JumpState
	palette            *t.CommandPaletteState
	tabSearch          *t.CommandPaletteState
	themeBeforePreview string
	helpScroll         *t.ScrollState
	highlighters       map[string]*syntaxHighlighter
	methodMenu         *t.MenuState
	menuOpen           t.Signal[bool]
	logoMenu           *t.MenuState
	logoMenuOpen       t.Signal[bool]
	logoReturnFocus    string           // focused when the logo menu opened
	overlay            t.Signal[string] // "", "help", "variables", "save", "confirm"
	confirm            t.AnySignal[confirmation]
	save               *saveForm
	variables          *variablesForm
	curlDialog         *curlForm
	toast              t.AnySignal[toast]
	toastSeq           int
}

// New creates the root widget.
func New(cfg Config) *App {
	if cfg.Sender == nil {
		cfg.Sender = client.Fake{}
	}
	if cfg.Describer == nil {
		cfg.Describer = client.Fake{}
	}
	if cfg.Environments == nil {
		cfg.Environments = StaticEnvironments(nil)
	}
	if cfg.Store == nil {
		cfg.Store = collection.Memory{}
	}
	if cfg.Collection == nil {
		cfg.Collection = &model.Collection{Name: "collection"}
	}
	settings := config.Defaults()
	if cfg.Settings != nil {
		settings = *cfg.Settings
	}
	if cfg.UserHost == "" {
		cfg.UserHost = userHost()
	}
	host := escapeMarkup(cfg.UserHost)
	if settings.Heading.Hostname != "" {
		// Like Posting 2, the configured hostname may contain markup.
		host = settings.Heading.Hostname
	}
	sidebarRatio := 0.28
	if settings.CollectionBrowser.Position == "right" {
		sidebarRatio = 1 - sidebarRatio
	}
	a := &App{
		version:        cfg.Version,
		settings:       settings,
		spacing:        t.NewSignal(settings.Spacing),
		icons:          iconsFor(cfg.NerdFonts),
		openURL:        cfg.OpenURL,
		runExternal:    t.RunExternal,
		after:          dispatchAfter,
		dispatch:       t.Dispatch,
		running:        make(chan struct{}),
		watcher:        cfg.Reload,
		envFile:        newEnvFileForm(),
		sender:         cfg.Sender,
		describer:      cfg.Describer,
		store:          cfg.Store,
		host:           host,
		collection:     t.NewAnySignal(cfg.Collection),
		treeScroll:     t.NewScrollState(),
		treeSearch:     t.NewTextInputState(""),
		treeFilter:     t.NewFilterState(),
		summary:        newSummaryState(),
		history:        t.NewAnySignal[[]model.HistoryEntry](nil),
		historyList:    t.NewListState[model.HistoryEntry](nil),
		historyScroll:  t.NewScrollState(),
		env:            &environments{source: cfg.Environments, active: t.NewAnySignal(model.Environment{}), host: cfg.HostVariables, remember: cfg.RememberEnvironment},
		sessionVars:    t.NewAnySignal(map[string]string{}),
		sessions:       t.NewAnySignal[[]*Session](nil),
		active:         t.NewSignal(0),
		sessionView:    newSessionTabsView(),
		sidebarVisible: t.NewSignal(settings.CollectionBrowser.ShowOnStartup),
		sidebarTab:     t.NewSignal("requests"),
		sidebarSplit:   t.NewSplitPaneState(sidebarRatio),
		panelSplit:     t.NewSplitPaneState(0.5),
		layout:         t.NewSignal(layoutMode(settings.Layout)),
		expanded:       t.NewSignal(""),
		compact:        t.NewSignal(false),
		helpScroll:     t.NewScrollState(),
		menuOpen:       t.NewSignal(false),
		logoMenuOpen:   t.NewSignal(false),
		overlay:        t.NewSignal(""),
		confirm:        t.NewAnySignal(confirmation{}),
		toast:          t.NewAnySignal(toast{}),
	}
	a.tree = t.NewTreeState(buildTree(cfg.Collection))
	if cfg.History != nil {
		a.historyStore = newHistorySaver(cfg.History)
		if entries, err := cfg.History.Load(); err != nil {
			cfg.StartupMessages = append(cfg.StartupMessages, "Couldn't load history: "+err.Error())
		} else {
			for i, e := range entries {
				a.nextHistoryID = max(a.nextHistoryID, e.ID)
				entries[i].Status = model.StatusOf(e.Request, e.Response)
			}
			a.history.Set(entries)
			a.historyList.SetItems(entries)
		}
	}
	a.jump = t.NewJumpState()
	a.palette = t.NewCommandPaletteState("Commands", nil)
	// No title: the tab search opens straight onto its input, without a
	// breadcrumb row above it.
	a.tabSearch = t.NewCommandPaletteState("", nil)
	a.methodMenu = t.NewMenuState(a.methodMenuItems())
	a.logoMenu = t.NewMenuState(a.logoMenuItems())
	a.save = newSaveForm()
	a.variables = newVariablesForm()
	a.curlDialog = newCurlForm()
	a.openSession(model.NewRequest())

	messages := append([]string(nil), cfg.StartupMessages...)
	a.userThemes = registerUserThemes(cfg.UserThemes)
	theme := themeAlias(settings.Theme)
	if _, ok := t.GetTheme(theme); !ok {
		messages = append(messages, fmt.Sprintf("Unknown theme %q; using galaxy", settings.Theme))
		theme = t.ThemeNameGalaxy
	}
	t.SetTheme(theme)
	t.SetCursorBlink(settings.TextInput.BlinkingCursor)
	switch settings.Focus.OnStartup {
	case "method":
		t.RequestFocus(methodSelectorID)
	case "collection":
		if a.sidebarVisible.Peek() {
			t.RequestFocus(treeID)
		} else {
			t.RequestFocus(urlInputID)
		}
	default:
		t.RequestFocus(urlInputID)
	}
	if len(cfg.Environment) > 0 {
		a.activateEnvironment(cfg.Environment)
	}
	if cfg.WatchEnvironment {
		a.watchEnvironment(time.Second)
	}
	if cfg.Watch != nil {
		a.watchCollection(cfg.Watch, time.Second)
	}
	if n := len(messages); n > 0 {
		message := messages[0]
		if n > 1 {
			message += fmt.Sprintf(" (and %d more)", n-1)
		}
		a.notify(message, toastError)
	}
	return a
}

// Run starts the UI and blocks until it exits.
func Run(cfg Config) error {
	return t.Run(New(cfg))
}

// themeAlias maps Posting 2 theme names to their Terma equivalents.
func themeAlias(name string) string {
	switch name {
	case "":
		return t.ThemeNameGalaxy
	case "catppuccin-mocha", "catppuccin-macchiato", "catppuccin-frappe":
		return t.ThemeNameCatppuccin
	case "textual-dark", "posting":
		return t.ThemeNameGalaxy
	case "textual-light":
		return t.ThemeNameCatppuccinLatte
	}
	return name
}

func userHost() string {
	name := "posting"
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	host, _ := os.Hostname()
	if net.ParseIP(host) == nil {
		host, _, _ = strings.Cut(host, ".")
	}
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
	s := a.createSession(req)
	a.sessions.Set(append(append([]*Session(nil), a.sessions.Peek()...), s))
	a.active.Set(s.id)
	return s
}

// createSession makes a session for req without adding it to the tabs.
func (a *App) createSession(req model.Request) *Session {
	a.nextSessionID++
	s := newSession(a.nextSessionID, req)
	s.dispatch = a.dispatch
	s.prettifyJSON = a.settings.Response.PrettifyJSON
	a.listProtoMethods(s)
	return s
}

// openRequest shows req from the collection. A tab that already holds the
// same file is brought into view; otherwise req opens in the preview tab.
func (a *App) openRequest(req model.Request) {
	if s := a.sessionForFile(req.File); s != nil {
		a.showSession(s.id)
		return
	}
	a.openPreview(req)
	a.focusOpenedRequest()
}

// openRequests opens each of reqs in a tab of its own, kept rather than
// previewed since they were opened together, and shows the last. A request
// already in a tab keeps that tab.
func (a *App) openRequests(reqs []model.Request) {
	var last *Session
	for _, req := range reqs {
		if last = a.sessionForFile(req.File); last != nil {
			a.keepSession(last.id)
		} else {
			last = a.openSession(req)
		}
	}
	if last != nil {
		a.showSession(last.id)
		a.focusOpenedRequest()
	}
}

// sessionForFile is the tab holding the request saved in file, if any.
func (a *App) sessionForFile(file string) *Session {
	if file == "" {
		return nil
	}
	for _, s := range a.sessions.Peek() {
		if s.file.Peek() == file {
			return s
		}
	}
	return nil
}

// previewSession is the preview tab, if there is one.
func (a *App) previewSession() *Session {
	for _, s := range a.sessions.Peek() {
		if s.preview.Peek() {
			return s
		}
	}
	return nil
}

// openPreview shows req in the preview tab, as VS Code does for files opened
// from its explorer, so browsing the sidebar doesn't leave a trail of tabs.
// There is at most one preview tab. An untouched blank tab in view becomes
// it, closing the old one; otherwise req replaces the preview tab in place
// or, without one, opens a new tab at the end. Replacing loses nothing: a
// tab stops being the preview as soon as it is edited, sent or saved.
func (a *App) openPreview(req model.Request) *Session {
	old := a.previewSession()
	if s := a.current(); s != nil && s.isPristine() {
		s.Load(req)
		a.listProtoMethods(s)
		s.preview.Set(true)
		if old != nil && old != s {
			a.closeSession(old.id)
		}
		return s
	}
	s := a.createSession(req)
	s.preview.Set(true)
	sessions := append([]*Session(nil), a.sessions.Peek()...)
	if i := slices.Index(sessions, old); i >= 0 {
		// A fresh session rather than reloading the old one, so none of its
		// response or view state carries over to the new request.
		old.Cancel()
		sessions[i] = s
	} else {
		sessions = append(sessions, s)
	}
	a.sessions.Set(sessions)
	a.active.Set(s.id)
	return s
}

// keepSession makes the preview tab id an ordinary tab, which opening other
// requests leaves alone.
func (a *App) keepSession(id int) {
	for _, s := range a.sessions.Peek() {
		if s.id == id {
			s.preview.Set(false)
		}
	}
}

// keepFile keeps the preview tab if it holds file, as double-clicking the
// request in the collection does.
func (a *App) keepFile(file string) {
	if s := a.sessionForFile(file); s != nil {
		a.keepSession(s.id)
	}
}

// focusOpenedRequest moves focus as the focus.on_request_open setting asks,
// into the first field of a request tab, or to the URL or method.
func (a *App) focusOpenedRequest() {
	s := a.current()
	target := a.settings.Focus.OnRequestOpen
	if s == nil || target == "" {
		return
	}
	switch target {
	case "url":
		t.RequestFocus(urlInputID)
		return
	case "method":
		t.RequestFocus(methodSelectorID)
		return
	}
	t.RequestFocus(s.contentFocusID(s.selectRequestTab(target)))
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
// Actions

// send sends the current request, or while it has a stream open, sends the
// message being edited through it.
func (a *App) send() {
	s := a.current()
	if s == nil {
		return
	}
	// A stream stays open when the request becomes another kind, until the
	// next send cancels it.
	if s.streaming.Peek() == streamOpen && s.kind.Peek() == model.KindGRPC {
		message := s.payloads[model.KindGRPC].(*grpcEditor).message.GetText()
		if err := s.stream.Send(message); err != nil {
			a.notify(err.Error(), toastWarning)
		}
		return
	}
	a.start(s, nil)
}

// toggleStream opens a stream for the current gRPC request, or ends the
// sending side of the one open.
func (a *App) toggleStream() {
	s := a.current()
	switch {
	case s == nil:
	case s.kind.Peek() != model.KindGRPC:
		a.notify("Only gRPC requests can open a stream", toastWarning)
	case s.streaming.Peek() == streamStarting:
		a.notify("The stream hasn't started yet", toastWarning)
	case s.streaming.Peek() == streamOpen:
		s.stream.Close()
		s.streaming.Set(streamEnded)
	default:
		a.start(s, client.NewStream())
	}
}

// streamCommand is the palette's command to open a stream for s, or end
// the one open. It lives outside grpcCommands, which kindViews holds, since
// opening a stream reads kindViews.
func (a *App) streamCommand(s *Session) t.CommandPaletteItem {
	if s.streaming.Peek() == streamOpen {
		return t.CommandPaletteItem{Label: "End gRPC stream", Description: "Stop sending, and wait for the server to end the call", Action: a.run(a.toggleStream)}
	}
	return t.CommandPaletteItem{Label: "Open gRPC stream", Description: "Call the method and keep sending messages until you end the stream", Action: a.run(a.toggleStream)}
}

// start sends s's request, opening stream when it isn't nil.
func (a *App) start(s *Session, stream *client.Stream) {
	if strings.TrimSpace(s.url.GetText()) == "" {
		a.notify("Enter a URL before sending", toastWarning)
		t.RequestFocus(urlInputID)
		return
	}
	// Sending commits to the tab, so its response isn't lost to the next
	// request opened.
	s.preview.Set(false)
	variables, err := a.requestVariableValues(s.Snapshot())
	if err != nil {
		s.Cancel()
		s.err.Set(err)
		s.trace.Set(nil)
		s.phase.Set(exchangeFailed)
		a.notify(err.Error(), toastWarning)
		return
	}
	s.Send(a.sender, variables, stream, func(req model.Request, resp *model.Response, status model.Status) {
		if req.Kind() == model.GRPCKind {
			// The call has just reached the server, so asking it for its
			// methods contacts no one new.
			s.payloads[model.KindGRPC].(*grpcEditor).describe(a, true)
		}
		a.nextHistoryID++
		entry := historyEntry(a.nextHistoryID, req, resp, status)
		a.setHistory(append([]model.HistoryEntry{entry}, a.history.Peek()...))
		// A response in a background tab must not interrupt the request the
		// user is working on. Switching back before it arrives still applies
		// the configured response focus behavior.
		if a.current() != s {
			return
		}
		switch a.settings.Focus.OnResponse {
		case "body":
			s.responseTab.Set("body")
			if len(resp.Body) > 0 {
				t.RequestFocus("resp-body")
			} else {
				t.RequestFocus(responseTabsID)
			}
		case "tabs":
			t.RequestFocus(responseTabsID)
		}
	})
}

func (a *App) cancelSend() {
	if s := a.current(); s != nil && s.phase.Peek() == exchangeSending {
		s.Stop()
		a.notify("Request cancelled", toastWarning)
	}
}

// setMethod makes the current request an HTTP request with method m.
func (a *App) setMethod(m model.Method) {
	s := a.current()
	if s == nil || (s.method.Peek() == m && s.kind.Peek() == model.KindHTTP) {
		return
	}
	s.method.Set(m)
	s.setKind(model.KindHTTP)
	s.touch()
}

// setKind changes the kind of the current request. The URL, headers, auth
// and options are shared by every kind, so they carry across.
func (a *App) setKind(id model.KindID) {
	if s := a.current(); s != nil && s.kind.Peek() != id {
		s.setKind(id)
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

// methodChoice is a choice in the method selector: an HTTP method, or a kind
// of request other than HTTP.
type methodChoice struct {
	label, hotkey string
	choose        func()
}

// methodChoices are the HTTP methods, then the other kinds of request. The
// menu lists them, and the selector and the open menu bind their hotkeys.
func (a *App) methodChoices() []methodChoice {
	choices := make([]methodChoice, 0, len(model.Methods)+len(model.Kinds))
	for _, m := range model.Methods {
		choices = append(choices, methodChoice{label: string(m), hotkey: methodHotkeys[m], choose: func() { a.setMethod(m) }})
	}
	for _, kind := range model.Kinds {
		if kind != model.HTTPKind {
			choices = append(choices, methodChoice{label: kind.Label, hotkey: kindViews[kind.ID].hotkey, choose: func() { a.setKind(kind.ID) }})
		}
	}
	return choices
}

// methodMenuItems are the method choices, with a separator before the kinds.
func (a *App) methodMenuItems() []t.MenuItem {
	var items []t.MenuItem
	for i, choice := range a.methodChoices() {
		if i == len(model.Methods) {
			items = append(items, t.MenuItem{})
		}
		items = append(items, t.MenuItem{Label: choice.label, Shortcut: choice.hotkey, Action: choice.choose})
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
	if !a.storeRequest(req) {
		return
	}
	s.dirty.Set(false)
	s.preview.Set(false)
	a.notify("Saved "+req.File, toastSuccess)
}

// storeRequest writes req to the collection on disk and upserts it into the
// in-memory collection by its File. It reports whether the save worked; a
// failure has already been shown to the user.
func (a *App) storeRequest(req model.Request) bool {
	if err := a.writeRequest(req); err != nil {
		a.notify("Couldn't save "+req.File+": "+err.Error(), toastError)
		return false
	}
	return true
}

// writeRequest is storeRequest, returning the error rather than showing it.
func (a *App) writeRequest(req model.Request) error {
	if err := a.store.Save(req); err != nil {
		return err
	}
	root := a.collection.Peek()
	folderPath, _ := splitFile(req.File)
	folder := collection.EnsureFolder(root, folderPath)
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
	return nil
}

// deleteRequests deletes files from the store and the collection, stopping
// at the first that can't be deleted. A tab holding a deleted request keeps
// it as an unsaved edit. The tree's selection named what was deleted, so it
// goes too.
func (a *App) deleteRequests(files []string) {
	deleted := 0
	for _, file := range files {
		if err := a.store.Delete(file); err != nil {
			a.notify("Couldn't delete "+file+": "+err.Error(), toastError)
			break
		}
		folderPath, _ := splitFile(file)
		if folder := findFolder(a.collection.Peek(), folderPath); folder != nil {
			folder.Requests = slices.DeleteFunc(folder.Requests, func(r model.Request) bool { return r.File == file })
		}
		for _, s := range a.sessions.Peek() {
			if s.file.Peek() == file {
				s.file.Set("")
				s.markEdited()
			}
		}
		deleted++
	}
	a.tree.ClearSelection()
	a.refreshTree()
	switch {
	case deleted < len(files):
		return
	case deleted == 1:
		a.notify("Deleted "+files[0], toastInfo)
	default:
		a.notify(fmt.Sprintf("Deleted %d requests", deleted), toastInfo)
	}
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

func (a *App) focusRequestTab(key string) {
	if s := a.current(); s != nil {
		s.selectRequestTab(key)
		t.RequestFocus(requestTabsID)
	}
}

// ---------------------------------------------------------------------------
// Root widget

func (a *App) Build(ctx t.BuildContext) t.Widget {
	a.markRunning()
	theme := ctx.Theme()
	// Without the collection on the left, the workspace needs the margin
	// the split pane's divider would otherwise give it.
	sidebarLeft := a.sidebarVisible.Get() && a.settings.CollectionBrowser.Position != "right"
	margin := t.EdgeInsets{Left: 1}
	if sidebarLeft {
		margin = t.EdgeInsets{}
	}
	var top []t.Widget
	if a.settings.Heading.Visible {
		top = []t.Widget{header{app: a}}
	}
	var main t.Widget = t.Dock{
		Style: t.Style{BackgroundColor: theme.Background, Padding: margin},
		Top:   top,
		Body: t.Stack{
			Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			Children: []t.Widget{
				heightProbe{app: a},
				focusProbe{app: a},
				t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1), Padding: t.EdgeInsets{Right: 1}}, Children: []t.Widget{workspace{app: a}}},
			},
		},
	}
	// The sidebar sits beside the header rather than below it. The footer
	// runs the full width of the app, below both.
	if a.sidebarVisible.Get() {
		split := t.SplitPane{
			ID:           sidebarSplitID,
			State:        a.sidebarSplit,
			First:        sidebar{app: a},
			Second:       main,
			MinPaneSize:  24,
			DisableFocus: true,
		}
		if a.settings.CollectionBrowser.Position == "right" {
			split.First, split.Second = split.Second, split.First
		}
		main = split
	}

	return t.Jumper{
		Key:     a.jumpKey(),
		State:   a.jump,
		Targets: a.jumpTargets(),
		Dynamic: true,
		Child: t.Column{
			Style:    t.Style{Width: t.Flex(1), Height: t.Flex(1), BackgroundColor: theme.Background},
			Children: []t.Widget{main, footer{app: a}, overlays{app: a}},
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
	// The sidebar jumps go straight to the list, which is what you want to
	// move through, rather than to its tab strip.
	for i, tab := range []string{"requests", "history"} {
		tab := tab
		id := tabID(sidebarTabsID, tab)
		if i == 0 {
			id = sidebarTabsID
		}
		targets = append(targets, t.JumpTarget{Key: string("34"[i]), ID: id, Action: func() {
			a.sidebarTab.Set(tab)
			a.focusSidebarList()
		}})
	}
	if s := a.current(); s != nil {
		var keys string
		var tabs []string
		for _, tab := range s.requestTabList() {
			keys += tab.jump
			tabs = append(tabs, tab.key)
		}
		targets = append(targets, tabJumps(requestTabsID, keys, tabs, func(key string) {
			if s := a.current(); s != nil {
				s.selectRequestTab(key)
			}
		})...)
	}
	// Which tabs a response has depends on the request it answers.
	if s := a.current(); s != nil && s.response.Get() != nil {
		targets = append(targets, tabJumps(responseTabsID, "asdf", responseTabKeys(s.sent), func(key string) {
			if s := a.current(); s != nil && s.response.Peek() != nil {
				s.responseTabs(s.response.Peek()).selectKey(key)
			}
		})...)
	}
	return targets
}

// focusBelowSidebarTabs moves down from the sidebar's tabs: to the search box
// above the request tree, or the history list.
func (a *App) focusBelowSidebarTabs() {
	if a.sidebarTab.Peek() == "requests" && len(a.tree.Nodes.Peek()) > 0 {
		t.RequestFocus(treeSearchID)
		return
	}
	a.focusSidebarList()
}

// focusSidebarList focuses the list in the visible sidebar tab.
func (a *App) focusSidebarList() {
	if a.sidebarTab.Peek() == "history" {
		if len(a.history.Peek()) > 0 {
			t.RequestFocus(historyID)
			return
		}
	} else if len(a.tree.Nodes.Peek()) > 0 {
		t.RequestFocus(treeID)
		return
	}
	t.RequestFocus(sidebarTabsID)
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

// focusAt is a focused widget: its ID, in the tab it belongs to. Tabs reuse
// IDs, so switching tabs can move focus without changing the ID.
type focusAt struct {
	session int
	id      string
}

// focusProbe reports focus moving, which Terma has no callback for. It reads
// the focused widget while it is laid out, so its layout reruns whenever
// focus moves, and reports the move on the next frame, like heightProbe.
type focusProbe struct{ app *App }

func (p focusProbe) GetContentDimensions() (t.Dimension, t.Dimension) { return t.Flex(1), t.Flex(1) }
func (p focusProbe) Build(t.BuildContext) t.Widget                    { return p }
func (p focusProbe) Render(*t.RenderContext)                          {}

func (p focusProbe) OnLayout(ctx t.BuildContext, _ t.LayoutMetrics) {
	signal := ctx.FocusedSignal()
	if !signal.IsValid() {
		return
	}
	at := focusAt{session: p.app.active.Get(), id: t.SelectAny(signal, focusedIDOf)}
	if at != p.app.focus {
		t.Dispatch(func() { p.app.focusMoved(at) })
	}
}

// focusMoved acts on focus arriving somewhere. Moving to the gRPC method
// field asks for the methods it lists.
func (a *App) focusMoved(at focusAt) {
	if at == a.focus {
		return
	}
	a.focus = at
	if s := a.current(); s != nil && at.id == grpcMethodID {
		s.payloads[model.KindGRPC].(*grpcEditor).describe(a, false)
	}
}

// gap is the number of blank rows between parts of the layout: one, or none
// in compact mode.
func (a *App) gap() int {
	if a.spacing.Get() == "compact" || a.compact.Get() {
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
	// The sidebar's hints count what is selected (see collectionView), the
	// stream action is named for the stream's phase, and the keybind bar
	// only rebuilds by itself when focus moves.
	f.app.tree.Selection.Get()
	f.app.historyList.Selection.Get()
	if s := f.app.current(); s != nil {
		s.kind.Get()
		s.streaming.Get()
	}
	style := t.Style{ForegroundColor: theme.TextMuted}
	var hints t.Widget = t.KeybindBar{Style: style, FormatKey: t.FormatKeyCaret, Width: t.Auto}
	if f.app.jump.IsActive() {
		text := t.ParseMarkupToText("[b $AccentText]Jump[/]  Type a label to move there  [b $Text]esc[/] cancel", theme)
		text.Style = style
		hints = text
	}
	padding := t.EdgeInsetsXY(2, 0)
	if f.app.settings.Heading.ShowVersion {
		// The logo's glow runs from the left edge and pads the logo itself.
		padding.Left = 0
	}
	return t.Row{
		Style: t.Style{Width: t.Flex(1), Height: t.Cells(1), Padding: padding, BackgroundColor: theme.Background},
		Children: []t.Widget{
			t.ShowWhen(f.app.settings.Heading.ShowVersion, logoGlow(theme, footerLogo{app: f.app})),
			// The hints sit at the right end of the space the version leaves,
			// and leave out those that don't fit.
			t.Row{Style: t.Style{Width: t.Flex(1)}, MainAlign: t.MainAxisEnd, Children: []t.Widget{hints}},
		},
	}
}

// logoGlow sits the footer logo on a faint wash of the theme's secondary
// colour that fades out from the left edge.
func logoGlow(theme t.ThemeData, logo t.Widget) t.Widget {
	bg := theme.Background
	tint := bg.Blend(theme.Secondary, 0.18)
	return t.Row{
		Style: t.Style{
			Height:          t.Cells(1),
			Padding:         t.EdgeInsets{Left: 2, Right: 4},
			BackgroundColor: t.NewGradient(tint, bg.Blend(tint, 0.3), bg).WithAngle(90),
		},
		Children: []t.Widget{logo},
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
					{Text: a.icons.environment, Style: t.SpanStyle{Foreground: envColor}},
					{Text: envLabel, Style: t.SpanStyle{Foreground: envColor, Bold: env != ""}},
				},
				Click: func(t.MouseEvent) { a.openEnvironmentPicker() },
			},
			t.ShowWhen(a.settings.Heading.ShowHost, t.ParseMarkupToText("   [$TextMuted]"+escapeMarkup(a.icons.host)+a.host+"[/]", theme)),
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
	duration := 3 * time.Second
	if kind == toastError {
		duration = 6 * time.Second
	}
	time.AfterFunc(duration, func() {
		t.Dispatch(func() {
			if a.toast.Peek().seq == seq {
				a.toast.Set(toast{})
			}
		})
	})
}

// dispatchAfter runs fn on the UI goroutine once d has passed.
func dispatchAfter(d time.Duration, fn func()) {
	time.AfterFunc(d, func() { t.Dispatch(fn) })
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

func pluralize(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

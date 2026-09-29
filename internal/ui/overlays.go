package ui

import (
	"regexp"
	"strings"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

const (
	paletteID       = "palette"
	requestSearchID = "request-search"
	themesTitle     = "Themes"
)

// overlays hosts every floating layer. Floating widgets register themselves
// with the renderer, so where this sits in the tree doesn't matter.
type overlays struct{ app *App }

func (o overlays) Build(ctx t.BuildContext) t.Widget {
	a := o.app
	overlay := a.overlay.Get()
	return t.Column{Children: []t.Widget{
		t.CommandPalette{
			ID:             paletteID,
			State:          a.palette,
			Placeholder:    "Type a command…",
			Position:       t.FloatPositionTopCenter,
			Offset:         t.Offset{Y: 3},
			OnCursorChange: a.previewTheme,
			OnDismiss:      a.restoreTheme,
		},
		t.CommandPalette{
			ID:          requestSearchID,
			State:       a.requestSearch,
			Placeholder: "Search for a request…",
			Position:    t.FloatPositionTopCenter,
			Offset:      t.Offset{Y: 3},
			RenderItem: func(item t.CommandPaletteItem, active bool, match t.MatchResult) t.Widget {
				return renderRequestSearchItem(ctx.Theme(), item, active, match)
			},
		},
		helpOverlay{app: a, visible: overlay == "help"},
		saveOverlay{app: a, visible: overlay == "save"},
		variablesOverlay{app: a, visible: overlay == "variables"},
		confirmOverlay{app: a, visible: overlay == "confirm"},
		toastOverlay{app: a},
	}}
}

func (a *App) closeOverlay() {
	a.overlay.Set("")
}

// ---------------------------------------------------------------------------
// Command palette

func (a *App) openPalette() {
	a.palette.SetItems(a.paletteItems())
	a.palette.Open()
}

// run wraps an action so it closes the palette first.
func (a *App) run(action func()) func() {
	return func() {
		a.palette.Close(false)
		action()
	}
}

func (a *App) paletteItems() []t.CommandPaletteItem {
	layoutLabel := "Layout: side by side"
	if a.layout.Peek() == layoutHorizontal {
		layoutLabel = "Layout: stacked"
	}
	sidebarLabel := "View: hide collection"
	if !a.sidebarVisible.Peek() {
		sidebarLabel = "View: show collection"
	}
	items := []t.CommandPaletteItem{
		{Label: "Send request", Hint: "ctrl+j", Action: a.run(a.send)},
		{Label: "New request tab", Hint: "ctrl+n", Action: a.run(a.newTab)},
		{Label: "Save request", Hint: "ctrl+s", Action: a.run(a.saveRequest)},
		{Label: "Close request tab", Hint: "alt+w", Action: a.run(func() { a.closeSession(a.active.Peek()) })},
		{Label: "Go to request…", Hint: "ctrl+g", Action: a.run(a.openRequestSearch)},
		{Label: "Jump mode", Hint: "ctrl+o", Action: a.run(a.jump.Activate)},
		{Divider: "Environment"},
		{Label: "Switch environment…", ChildrenTitle: "Environments", Children: a.environmentItems},
		{Label: "Variables", Hint: "ctrl+shift+v", Action: func() {
			// Closing the palette restores focus, which would otherwise
			// replace the table focus that openVariables asks for.
			a.palette.SetNextFocusIDOnClose("vars-table")
			a.run(a.openVariables)()
		}},
		{Divider: "Export"},
		{Label: "Copy as curl", Action: a.run(a.copyAsCurl)},
		{Divider: "View"},
		{Label: layoutLabel, Action: a.run(a.toggleLayout)},
		{Label: sidebarLabel, Hint: "ctrl+h", Action: a.run(a.toggleSidebar)},
		{Label: "View: expand request", Action: a.run(func() { a.expanded.Set("request") })},
		{Label: "View: expand response", Action: a.run(func() { a.expanded.Set("response") })},
	}
	if a.expanded.Peek() != "" {
		items = append(items, t.CommandPaletteItem{Label: "View: restore panels", Hint: "alt+z", Action: a.run(func() { a.expanded.Set("") })})
	}
	items = append(items,
		t.CommandPaletteItem{Label: "Theme…", ChildrenTitle: themesTitle, Children: a.themeItems},
		t.CommandPaletteItem{Divider: "App"},
		t.CommandPaletteItem{Label: "Clear history", Action: a.run(a.clearHistory)},
		t.CommandPaletteItem{Label: "Keyboard shortcuts", Hint: "f1", Action: a.run(func() { a.overlay.Set("help") })},
		t.CommandPaletteItem{Label: "Quit Posting", Hint: "ctrl+c", Action: t.Quit},
	)
	return items
}

func (a *App) environmentItems() []t.CommandPaletteItem {
	items := make([]t.CommandPaletteItem, 0, len(a.environments)+1)
	active := a.activeEnv.Peek()
	for i, env := range a.environments {
		index := i
		items = append(items, t.CommandPaletteItem{
			Label:       env.Name,
			Hint:        strings.Join(env.Files, " + "),
			Description: pluralize(len(env.Variables), "variable"),
			Current:     i == active,
			Action:      a.run(func() { a.switchEnvironment(index) }),
		})
	}
	items = append(items, t.CommandPaletteItem{
		Label:   "No environment",
		Hint:    "session variables only",
		Current: active < 0,
		Action:  a.run(func() { a.switchEnvironment(-1) }),
	})
	return items
}

func (a *App) openEnvironmentPicker() {
	a.palette.SetItems(a.paletteItems())
	a.palette.Open()
	a.palette.PushLevel("Environments", a.environmentItems())
}

func (a *App) themeItems() []t.CommandPaletteItem {
	current := t.CurrentThemeName()
	if a.themeBeforePreview != "" {
		current = a.themeBeforePreview
	}
	var items []t.CommandPaletteItem
	add := func(names []string) {
		for _, name := range names {
			themeName := name
			items = append(items, t.CommandPaletteItem{
				Label:   themeName,
				Current: themeName == current,
				Data:    themeName,
				Action: a.run(func() {
					a.themeBeforePreview = ""
					t.SetTheme(themeName)
				}),
			})
		}
	}
	items = append(items, t.CommandPaletteItem{Divider: "Dark"})
	add(t.DarkThemeNames())
	items = append(items, t.CommandPaletteItem{Divider: "Light"})
	add(t.LightThemeNames())
	return items
}

// previewTheme applies the theme under the cursor while browsing themes.
func (a *App) previewTheme(item t.CommandPaletteItem) {
	level := a.palette.CurrentLevel()
	themeName, ok := item.Data.(string)
	if level == nil || level.Title != themesTitle || !ok {
		a.restoreTheme()
		return
	}
	if a.themeBeforePreview == "" {
		a.themeBeforePreview = t.CurrentThemeName()
	}
	t.SetTheme(themeName)
}

func (a *App) restoreTheme() {
	if a.themeBeforePreview != "" {
		t.SetTheme(a.themeBeforePreview)
		a.themeBeforePreview = ""
	}
}

// openRequestSearch lists every request in the collection for fuzzy search.
func (a *App) openRequestSearch() {
	var items []t.CommandPaletteItem
	a.collection.Peek().Walk(func(_ *model.Collection, r model.Request) {
		req := r
		items = append(items, t.CommandPaletteItem{
			Label: req.DisplayName(),
			Hint:  req.File,
			Data:  req.Method,
			Action: func() {
				a.requestSearch.SetNextFocusIDOnClose(urlInputID)
				a.requestSearch.Close(false)
				a.openRequest(req)
			},
		})
	})
	a.requestSearch.SetItems(items)
	a.requestSearch.Open()
}

func renderRequestSearchItem(theme t.ThemeData, item t.CommandPaletteItem, active bool, match t.MatchResult) t.Widget {
	method, _ := item.Data.(model.Method)
	style := t.Style{Width: t.Flex(1), Padding: t.EdgeInsetsXY(1, 0)}
	label := t.Style{ForegroundColor: theme.Text, Width: t.Flex(1)}
	hint := t.Style{ForegroundColor: theme.TextMuted}
	methodFg := methodColor(theme, method)
	if active {
		style.BackgroundColor = theme.ActiveCursor
		label.ForegroundColor = theme.SelectionText
		hint.ForegroundColor = theme.SelectionText
		methodFg = theme.SelectionText
	}
	name := t.Text{Content: item.Label, Style: label}
	if match.Matched {
		name.Content = ""
		name.Spans = t.HighlightSpans(item.Label, match.Ranges, t.MatchHighlightStyle(theme))
	}
	return t.Row{
		Style: style,
		Children: []t.Widget{
			t.Text{Content: padRight(string(method), 8), Style: t.Style{ForegroundColor: methodFg, Bold: true}},
			name,
			t.Text{Content: item.Hint, Style: hint},
		},
	}
}

// ---------------------------------------------------------------------------
// Modal frame

// modal is a centred box with a title, used by every dialog.
type modal struct {
	Visible   bool
	Title     string
	Width     t.Dimension
	Height    t.Dimension
	Child     t.Widget
	OnDismiss func()
}

func (m modal) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	return t.Floating{
		Visible: m.Visible,
		Config: t.FloatConfig{
			Position:  t.FloatPositionCenter,
			Modal:     true,
			OnDismiss: m.OnDismiss,
		},
		Child: t.Column{
			Style: t.Style{
				Width:           m.Width,
				Height:          m.Height,
				BackgroundColor: theme.Background,
				Border:          t.RoundedBorder(theme.Primary, t.BorderTitleMarkup(" [b $Text]"+m.Title+"[/] "), t.BorderSubtitleRightMarkup(" [$TextMuted]esc to close[/] ")),
				Padding:         t.EdgeInsetsXY(2, 1),
			},
			Children: []t.Widget{m.Child},
		},
	}
}

// ---------------------------------------------------------------------------
// Help

type helpOverlay struct {
	app     *App
	visible bool
}

var helpSections = []struct {
	title string
	keys  [][2]string
}{
	{"Global", [][2]string{
		{"ctrl+j", "Send the request (alt+enter also works)"},
		{"esc", "Cancel an in-flight request"},
		{"ctrl+o", "Jump mode: move focus by typing a label"},
		{"ctrl+p", "Command palette"},
		{"ctrl+g", "Go to a request in the collection"},
		{"ctrl+s", "Save the request to the collection"},
		{"ctrl+n", "Open a new request tab"},
		{"alt+w", "Close the request tab"},
		{"alt+← / alt+→", "Switch request tab"},
		{"ctrl+l", "Focus the URL bar"},
		{"ctrl+t", "Choose the HTTP method"},
		{"ctrl+h", "Show or hide the collection"},
		{"alt+z", "Expand the focused panel"},
		{"ctrl+shift+v", "Variables"},
		{"f1", "This help"},
		{"ctrl+c", "Quit"},
	}},
	{"Jump mode", [][2]string{
		{"1 / 2", "Method selector / URL bar"},
		{"3 / 4", "Collection / History"},
		{"q w e r t y u", "Request tabs, Headers to Options"},
		{"a s d f", "Response tabs, Body to Trace"},
		{"other labels", "Requests, history entries, open tabs and fields"},
		{"esc", "Leave jump mode"},
	}},
	{"Method selector", [][2]string{
		{"g p u a d h o", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS"},
		{"enter", "Open the method menu"},
	}},
	{"Tabs", [][2]string{{"h / l, ← / →", "Previous / next tab"}}},
	{"Headers, query, path, form", [][2]string{
		{"type in the last row", "Add a new row"},
		{"↑ / ↓", "Move between rows"},
		{"ctrl+space", "Enable or disable the row"},
		{"ctrl+x", "Delete the row"},
	}},
	{"Collection", [][2]string{
		{"enter", "Open request / toggle folder"},
		{"space", "Expand or collapse"},
		{"/", "Search requests"},
		{"d", "Duplicate request"},
		{"backspace", "Delete request"},
	}},
	{"Response body", [][2]string{
		{"w", "Toggle soft wrap"},
		{"y", "Copy the body"},
	}},
}

func (h helpOverlay) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	rows := []t.Widget{}
	for i, section := range helpSections {
		title := t.Text{Content: section.title, Style: t.Style{ForegroundColor: theme.AccentText, Bold: true}}
		if i > 0 {
			title.Style.Margin = t.EdgeInsetsTRBL(1, 0, 0, 0)
		}
		rows = append(rows, title)
		for _, key := range section.keys {
			rows = append(rows, t.Text{Spans: []t.Span{
				{Text: padRight(key[0], 22), Style: t.SpanStyle{Foreground: theme.Text, Bold: true}},
				{Text: key[1], Style: t.SpanStyle{Foreground: theme.TextMuted}},
			}})
		}
	}
	return modal{
		Visible:   h.visible,
		Title:     "Keyboard shortcuts",
		Width:     t.Cells(76),
		Height:    t.Cells(30),
		OnDismiss: h.app.closeOverlay,
		Child: t.Scrollable{
			ID:        "help-scroll",
			State:     h.app.helpScroll,
			Focusable: true,
			Style:     t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			// Keep the descriptions clear of the scrollbar.
			Child: t.Column{Style: t.Style{Padding: t.EdgeInsetsTRBL(0, 2, 0, 0)}, Children: rows},
		},
	}
}

// ---------------------------------------------------------------------------
// Save

type saveForm struct {
	name        *t.TextInputState
	file        *t.TextInputState
	folder      *t.TextInputState
	description *t.TextAreaState
	err         t.Signal[string]
	scroll      *formScroll
}

func newSaveForm() *saveForm {
	return &saveForm{
		name:        t.NewTextInputState(""),
		file:        t.NewTextInputState(""),
		folder:      t.NewTextInputState(""),
		description: t.NewTextAreaState(""),
		err:         t.NewSignal(""),
		scroll:      newFormScroll(),
	}
}

func (f *saveForm) prefill(req model.Request, folder string) {
	f.name.SetText(req.Name)
	f.file.SetText("")
	f.folder.SetText(folder)
	f.description.SetText(req.Description)
	f.err.Set("")
	t.RequestFocus("save-name")
}

var slugUnsafe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	return strings.Trim(slugUnsafe.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func (a *App) submitSave() {
	f := a.save
	name := strings.TrimSpace(f.name.GetText())
	if name == "" {
		f.err.Set("Give the request a name")
		return
	}
	folder := strings.Trim(strings.TrimSpace(f.folder.GetText()), "/")
	if strings.Contains(folder, "..") || strings.Contains(folder, ":") {
		f.err.Set("The folder must be a path inside the collection")
		return
	}
	file := slugify(strings.TrimSuffix(strings.TrimSpace(f.file.GetText()), ".posting.yaml"))
	if file == "" {
		file = slugify(name)
	}
	if file == "" {
		f.err.Set("Choose a file name")
		return
	}
	path := file + ".posting.yaml"
	if folder != "" {
		path = folder + "/" + path
	}
	if a.fileExists(path) {
		f.err.Set("A request already exists at " + path)
		return
	}
	s := a.current()
	if s == nil {
		return
	}
	s.syncing = true
	s.name.SetText(name)
	s.description.SetText(f.description.GetText())
	s.file.Set(path)
	s.syncing = false
	req := s.Snapshot()
	a.storeRequest(req)
	s.title.Set(req.DisplayName())
	s.dirty.Set(false)
	a.closeOverlay()
	a.notify("Saved "+path, toastSuccess)
}

type saveOverlay struct {
	app     *App
	visible bool
}

func (o saveOverlay) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a := o.app
	f := a.save
	submit := func(string) { a.submitSave() }
	filePlaceholder := slugify(textOf(f.name))
	if filePlaceholder == "" {
		filePlaceholder = "derived from the name"
	}
	rows := []formField{
		field(formRow(ctx, "Name", "", input{ID: "save-name", State: f.name, Placeholder: "e.g. List users", OnSubmit: submit}), "save-name"),
		field(formRow(ctx, "File name", "", t.Row{Style: t.Style{Width: t.Flex(1)}, Children: []t.Widget{
			input{ID: "save-file", State: f.file, Placeholder: filePlaceholder, OnSubmit: submit},
			t.Text{Content: ".posting.yaml", Style: t.Style{ForegroundColor: theme.TextMuted}},
		}}), "save-file"),
		field(formRow(ctx, "Folder", "", input{ID: "save-folder", State: f.folder, Placeholder: "collection root", OnSubmit: submit}), "save-folder"),
		field(t.Row{Style: t.Style{Width: t.Flex(1), Height: t.Cells(4)}, Children: []t.Widget{
			formLabel([]t.Span{{Text: "Description", Style: t.SpanStyle{Foreground: theme.Text, Bold: true}}}),
			t.TextArea{ID: "save-description", State: f.description, Placeholder: "optional", Style: t.Style{Width: t.Flex(1), Height: t.Flex(1), BackgroundColor: theme.Surface, Padding: t.EdgeInsetsXY(1, 0)}},
		}}, "save-description"),
	}
	if errText := f.err.Get(); errText != "" {
		rows = append(rows, field(t.Text{Content: errText, Style: t.Style{ForegroundColor: theme.ErrorText}}))
	}
	// The buttons scroll with the fields: pinning them would need a fixed
	// height, and the dialog should otherwise be as tall as its content.
	rows = append(rows, field(t.Row{Spacing: 2, MainAlign: t.MainAxisEnd, Style: t.Style{Width: t.Flex(1)}, Children: []t.Widget{
		t.Button{ID: "save-cancel", Label: "Cancel", OnPress: a.closeOverlay, Click: func(t.MouseEvent) { a.closeOverlay() }},
		t.Button{ID: "save-submit", Label: "Save", Variant: t.ButtonSuccess, OnPress: a.submitSave, Click: func(t.MouseEvent) { a.submitSave() }},
	}}, "save-cancel", "save-submit"))
	return modal{
		Visible:   o.visible,
		Title:     "Save request",
		Width:     t.Cells(72),
		OnDismiss: a.closeOverlay,
		Child:     scrollForm{State: f.scroll, Spacing: 1, Rows: rows, Fit: true},
	}
}

// ---------------------------------------------------------------------------
// Variables

// addRowKey marks the trailing add row as the one being edited. It can't
// collide with a real variable because names are identifiers.
const addRowKey = "+"

type variablesForm struct {
	filter *t.TextInputState
	table  *t.TableState[model.Variable]
	// editing is the name of the row being edited in place, addRowKey for
	// the add row, or "" while browsing.
	editing t.Signal[string]
	// err explains why the edit row couldn't be saved. It is shown in the
	// overlay rather than as a toast, which would sit above the overlay and
	// swallow escape.
	err         t.Signal[string]
	name        *t.TextInputState
	value       *t.TextInputState
	showSecrets t.Signal[bool]
}

func newVariablesForm() *variablesForm {
	return &variablesForm{
		filter:      t.NewTextInputState(""),
		table:       t.NewTableState[model.Variable](nil),
		editing:     t.NewSignal(""),
		err:         t.NewSignal(""),
		name:        t.NewTextInputState(""),
		value:       t.NewTextInputState(""),
		showSecrets: t.NewSignal(false),
	}
}

// edit starts editing the named row, or stops editing with "".
func (f *variablesForm) edit(name string) {
	f.editing.Set(name)
	f.err.Set("")
}

// isAddRow reports whether v is the trailing row that creates a variable.
func isAddRow(v model.Variable) bool { return v.Name == "" }

func (a *App) openVariables() {
	a.variables.edit("")
	a.refreshVariableRows()
	a.overlay.Set("variables")
	t.RequestFocus("vars-table")
}

// refreshVariableRows recomputes the table rows from the filter. The add row
// is always last so there is somewhere to create a variable.
func (a *App) refreshVariableRows() {
	query := strings.ToLower(a.variables.filter.GetText())
	var rows []model.Variable
	for _, v := range a.variableList() {
		if query == "" || strings.Contains(strings.ToLower(v.Name), query) || strings.Contains(strings.ToLower(v.Value), query) {
			rows = append(rows, v)
		}
	}
	a.variables.table.SetRows(append(rows, model.Variable{}))
}

func (a *App) variableList() []model.Variable {
	byName := map[string]model.Variable{}
	if env := a.activeEnv.Peek(); env >= 0 && env < len(a.environments) {
		for _, v := range a.environments[env].Variables {
			byName[v.Name] = v
		}
	}
	for name, value := range a.sessionVars.Peek() {
		byName[name] = model.Variable{Name: name, Value: value, Source: "session"}
	}
	out := make([]model.Variable, 0, len(byName))
	for _, v := range byName {
		out = append(out, v)
	}
	sortVariables(out)
	return out
}

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// editVariable turns the row's value cell into an input. Double-clicking
// another row mid-edit saves the current edit first.
func (a *App) editVariable(v model.Variable) {
	if !a.commitVariable() {
		return
	}
	if isAddRow(v) {
		a.addVariable()
		return
	}
	f := a.variables
	f.name.SetText(v.Name)
	f.value.SetText(v.Value)
	f.value.CursorEnd()
	f.edit(v.Name)
	t.RequestFocus("vars-edit-value")
}

// addVariable moves to the add row and starts editing its name.
func (a *App) addVariable() {
	f := a.variables
	f.name.SetText("")
	f.value.SetText("")
	f.table.SelectLast()
	f.edit(addRowKey)
	t.RequestFocus("vars-edit-name")
}

// commitVariable stores the row being edited as a session value. It reports
// whether the edit finished; an invalid name keeps the row open.
func (a *App) commitVariable() bool {
	f := a.variables
	editing := f.editing.Peek()
	if editing == "" {
		return true
	}
	name := strings.TrimSpace(f.name.GetText())
	value := f.value.GetText()
	if editing == addRowKey && name == "" && value == "" {
		a.cancelVariableEdit()
		return true
	}
	if !identifier.MatchString(name) {
		f.err.Set("Variable names must be letters, digits and underscores, and can't start with a digit.")
		t.RequestFocus("vars-edit-name")
		return false
	}
	if current, ok := a.variableValuesPeek()[name]; !ok || current != value {
		vars := map[string]string{}
		for k, v := range a.sessionVars.Peek() {
			vars[k] = v
		}
		vars[name] = value
		a.sessionVars.Set(vars)
	}
	f.edit("")
	a.refreshVariableRows()
	for i, row := range f.table.GetRows() {
		if row.Name == name {
			f.table.SelectIndex(i)
		}
	}
	t.RequestFocus("vars-table")
	return true
}

// commitAndMove finishes the edit, then moves the cursor as the arrow keys
// would in the table itself.
func (a *App) commitAndMove(delta int) {
	if !a.commitVariable() {
		return
	}
	table := a.variables.table
	table.SelectIndex(table.CursorIndex.Peek() + delta)
}

func (a *App) cancelVariableEdit() {
	a.variables.edit("")
	t.RequestFocus("vars-table")
}

// dismissVariables backs out of an edit before closing the overlay.
func (a *App) dismissVariables() {
	if a.variables.editing.Peek() != "" {
		a.cancelVariableEdit()
		return
	}
	a.closeOverlay()
}

func (a *App) revertVariable() {
	v, ok := a.variables.table.SelectedRow()
	if !ok || v.Source != "session" {
		return
	}
	vars := map[string]string{}
	for k, value := range a.sessionVars.Peek() {
		if k != v.Name {
			vars[k] = value
		}
	}
	a.sessionVars.Set(vars)
	a.refreshVariableRows()
}

type variablesOverlay struct {
	app     *App
	visible bool
}

func (o variablesOverlay) Keybinds() []t.Keybind {
	a := o.app
	return []t.Keybind{
		{Key: "ctrl+r", Name: "Reveal secrets", Action: func() { a.variables.showSecrets.Set(!a.variables.showSecrets.Peek()) }},
	}
}

func (o variablesOverlay) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	a := o.app
	f := a.variables
	reveal := f.showSecrets.Get()
	env := a.envName()
	source := "No environment is active"
	if env != "" {
		source = "Environment: " + env
	}
	focused := isFocusedID(ctx, "vars-table")
	// Leaving an edit row with the arrows commits it, like moving between
	// cells in a spreadsheet.
	editKeys := []t.Keybind{
		{Key: "up", Name: "Row above", Action: func() { a.commitAndMove(-1) }, Hidden: true},
		{Key: "down", Name: "Row below", Action: func() { a.commitAndMove(1) }, Hidden: true},
	}
	// While a row is being edited, only its inputs can take focus, so Tab
	// can't strand the edit by moving to the filter or the table.
	editingRow := f.editing.Get() != ""
	valueInput := input{ID: "vars-edit-value", State: f.value, Placeholder: "value", OnSubmit: func(string) { a.commitVariable() }, Keybinds: editKeys}
	table := t.Table[model.Variable]{
		ID:            "vars-table",
		DisableFocus:  editingRow,
		State:         f.table,
		SelectionMode: t.TableSelectionRow,
		Columns: []t.TableColumn{
			{Width: t.Cells(24), Header: tableHeader(theme, "Variable")},
			{Width: t.Flex(1), Header: tableHeader(theme, "Value")},
			{Width: t.Cells(14), Header: tableHeader(theme, "Source")},
		},
		RenderCell: func(v model.Variable, row, col int, active, selected bool) t.Widget {
			editing := f.editing.Get()
			if isAddRow(v) {
				switch {
				case editing == addRowKey && col == 0:
					return input{ID: "vars-edit-name", State: f.name, Placeholder: "NAME", OnSubmit: func(string) { t.RequestFocus("vars-edit-value") }, Keybinds: editKeys}
				case editing == addRowKey && col == 1:
					return valueInput
				case editing == addRowKey:
					return mutedCell(theme, "session")
				case col == 0:
					return addRowCell(theme, active, focused)
				}
				return tableCell(theme, active, focused, false, "")
			}
			if editing == v.Name && col == 1 {
				return valueInput
			}
			value := v.Value
			if !reveal && model.IsSensitiveName(v.Name) {
				value = "••••••••••"
			}
			return tableCell(theme, active, focused, col == 0, []string{v.Name, value, v.Source}[col])
		},
		OnSelect: a.editVariable,
		Style:    t.Style{Width: t.Flex(1), Height: t.Flex(1)},
	}
	return modal{
		Visible:   o.visible,
		Title:     "Variables",
		Width:     t.Cells(96),
		Height:    t.Cells(26),
		OnDismiss: a.dismissVariables,
		Child: t.Column{
			Style:   t.Style{Width: t.Flex(1), Height: t.Flex(1)},
			Spacing: 1,
			Children: []t.Widget{
				t.Row{Style: t.Style{Width: t.Flex(1)}, Spacing: 2, Children: []t.Widget{
					input{
						ID: "vars-filter", State: f.filter, Placeholder: "Filter variables…", DisableFocus: editingRow,
						OnChange: func(string) { a.refreshVariableRows() },
						OnSubmit: func(string) { t.RequestFocus("vars-table") },
						Keybinds: []t.Keybind{{Key: "down", Name: "Variables", Action: func() { t.RequestFocus("vars-table") }, Hidden: true}},
					},
					t.Text{Content: source, Style: t.Style{ForegroundColor: theme.TextMuted}},
				}},
				variableTableKeys{app: a, child: table},
				variablesHint{form: f},
			},
		},
	}
}

// variablesHint explains session values, or why the edit row can't be saved.
type variablesHint struct {
	fillWidth
	form *variablesForm
}

func (h variablesHint) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	if err := h.form.err.Get(); err != "" {
		return t.Text{Content: err, Style: t.Style{ForegroundColor: theme.ErrorText}}
	}
	return t.Text{Content: "Edits are session values: they override the environment and are not written to disk.", Style: t.Style{ForegroundColor: theme.TextMuted}}
}

// addRowCell is the add row's prompt in the name column.
func addRowCell(theme t.ThemeData, active, focused bool) t.Widget {
	cell := tableCell(theme, active, focused, false, "+ Add variable").(t.Text)
	if !(active && focused) {
		cell.Style.ForegroundColor = theme.TextMuted
	}
	return cell
}

func mutedCell(theme t.ThemeData, content string) t.Widget {
	return t.Text{Content: content, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.TextMuted, Padding: t.EdgeInsetsXY(1, 0)}}
}

// variableTableKeys adds row actions to the variables table.
type variableTableKeys struct {
	fillParent
	app   *App
	child t.Widget
}

func (k variableTableKeys) Keybinds() []t.Keybind {
	if k.app.variables.editing.Peek() != "" {
		// The edit inputs handle these; listing them here puts them in the
		// footer in place of the table's actions.
		return []t.Keybind{
			{Key: "enter", Name: "Save", Action: func() { k.app.commitVariable() }},
			{Key: "escape", Name: "Cancel", Action: k.app.cancelVariableEdit},
		}
	}
	return []t.Keybind{
		{Key: "enter", Name: "Edit", Action: func() {
			if v, ok := k.app.variables.table.SelectedRow(); ok {
				k.app.editVariable(v)
			}
		}},
		{Key: "a", Name: "Add", Action: k.app.addVariable},
		{Key: "d", Name: "Revert override", Action: k.app.revertVariable},
		{Key: "/", Name: "Filter", Action: func() { t.RequestFocus("vars-filter") }},
	}
}

func (k variableTableKeys) Build(ctx t.BuildContext) t.Widget {
	return t.Column{Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}, Children: []t.Widget{k.child}}
}

// ---------------------------------------------------------------------------
// Confirmation

type confirmation struct {
	title   string
	message string
	confirm string
	danger  bool
	onYes   func()
}

func (a *App) askConfirm(c confirmation) {
	a.confirm.Set(c)
	a.overlay.Set("confirm")
}

type confirmOverlay struct {
	app     *App
	visible bool
}

func (o confirmOverlay) Build(ctx t.BuildContext) t.Widget {
	a := o.app
	c := a.confirm.Get()
	variant := t.ButtonPrimary
	if c.danger {
		variant = t.ButtonError
	}
	yes := func() {
		a.closeOverlay()
		if c.onYes != nil {
			c.onYes()
		}
	}
	message := t.ParseMarkupToText(c.message, ctx.Theme())
	message.Wrap = t.WrapSoft
	return t.Dialog{
		ID:      "confirm-dialog",
		Visible: o.visible,
		Title:   c.title,
		Content: message,
		Buttons: []t.Button{
			{ID: "confirm-no", Label: "Cancel", OnPress: a.closeOverlay},
			{ID: "confirm-yes", Label: orDefault(c.confirm, "Yes"), Variant: variant, OnPress: yes},
		},
		OnDismiss: a.closeOverlay,
	}
}

// ---------------------------------------------------------------------------
// Toasts

type toastKind int

const (
	toastInfo toastKind = iota
	toastSuccess
	toastWarning
	toastError
)

type toast struct {
	message string
	kind    toastKind
	seq     int
}

type toastOverlay struct{ app *App }

func (o toastOverlay) Build(ctx t.BuildContext) t.Widget {
	theme := ctx.Theme()
	current := o.app.toast.Get()
	accent := theme.Info
	switch current.kind {
	case toastSuccess:
		accent = theme.Success
	case toastWarning:
		accent = theme.Warning
	case toastError:
		accent = theme.Error
	}
	return t.Floating{
		Visible: current.message != "",
		Config: t.FloatConfig{
			Position:              t.FloatPositionBottomRight,
			Offset:                t.Offset{X: -2, Y: -2},
			DismissOnEsc:          t.BoolPtr(false),
			DismissOnClickOutside: t.BoolPtr(false),
		},
		Child: t.Row{
			Style: t.Style{BackgroundColor: theme.Surface2},
			Children: []t.Widget{
				t.Text{Content: "▌", Style: t.Style{ForegroundColor: accent}},
				t.Text{Content: current.message, Style: t.Style{ForegroundColor: theme.Text, Padding: t.EdgeInsetsXY(1, 0)}},
			},
		},
	}
}

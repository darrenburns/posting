package ui

import (
	"fmt"
	"math"
	"slices"
	"sync"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/curl"
	"github.com/darrenburns/posting/internal/grpcurl"
	"github.com/darrenburns/posting/internal/model"
)

// kindView is what the UI knows about a kind of request: the parts that
// need Terma. Toolkit-free facts (label, badge, fields) are on model.Kind.
type kindView struct {
	// hotkey picks the kind from the method selector, beside the method
	// letters. HTTP has none: its methods are the choices.
	hotkey string
	// color is the kind's badge colour. HTTP is coloured by method instead.
	color          func(theme t.ThemeData) t.Color
	urlPlaceholder string
	// rename relabels shared request tabs by key.
	rename map[string]string
	// newEditor makes a session's editor for the kind's payload. Nil for
	// HTTP, whose editing state is the session's own fields.
	newEditor func(s *Session) payloadEditor
	// responseTabs are the response panel's tabs, in order.
	responseTabs []string
	// export writes a request as a command for another tool.
	export exporter
	// commands are the palette's commands for the kind's requests.
	commands func(a *App, s *Session) []t.CommandPaletteItem
}

// exporter writes a request as a command line.
type exporter struct {
	tool   string // "curl"
	format func(a *App, req model.Request) (string, error)
}

var (
	httpResponseTabs = []string{"body", "headers", "cookies", "trace"}

	curlExporter = exporter{tool: "curl", format: func(a *App, req model.Request) (string, error) {
		wire, ok := model.Lower(req)
		if !ok {
			return "", fmt.Errorf("%s requests can't be copied as curl", req.Kind().Label)
		}
		return curl.Format(wire, curl.FormatOptions{ExtraArgs: a.settings.CurlExportExtraArgs, Multiline: true}), nil
	}}
)

var kindViews = map[model.KindID]kindView{
	model.KindHTTP: {
		urlPlaceholder: "Enter a URL or paste a curl command…",
		responseTabs:   httpResponseTabs,
		export:         curlExporter,
	},
	model.KindGraphQL: {
		hotkey:         "q",
		color:          graphQLColor,
		urlPlaceholder: "Enter a GraphQL endpoint, e.g. ${BASE_URL}/graphql",
		// "Query" is the GraphQL document, so the URL's query string
		// parameters are "Params".
		rename:       map[string]string{"query": "Params"},
		newEditor:    newGraphQLEditor,
		responseTabs: httpResponseTabs,
		export:       curlExporter,
	},
	model.KindGRPC: {
		hotkey:         "r",
		color:          grpcColor,
		urlPlaceholder: "Enter a server address, e.g. localhost:50051 or grpcs://api.example.com",
		// gRPC sends headers as metadata, and calls them that.
		rename:    map[string]string{"headers": "Metadata"},
		newEditor: newGRPCEditor,
		// A gRPC server sets no cookies, and ends every call with trailers.
		responseTabs: []string{"body", "headers", "trailers", "trace"},
		export: exporter{tool: "grpcurl", format: func(a *App, req model.Request) (string, error) {
			// Proto paths are relative to the collection; the command
			// should run from anywhere.
			var root string
			if dir, ok := a.store.(collection.Dir); ok {
				root = dir.Root
			}
			return grpcurl.Format(req, grpcurl.FormatOptions{Multiline: true, Root: root}), nil
		}},
		commands: grpcCommands,
	},
}

func graphQLColor(theme t.ThemeData) t.Color {
	return unlikeMethods(theme, nil, theme.AccentText, theme.SecondaryText, theme.Link)
}

// grpcColor stays apart from GraphQL's colour as well as the methods'.
func grpcColor(theme t.ThemeData) t.Color {
	return unlikeMethods(theme, []t.Color{graphQLColor(theme)}, theme.SecondaryText, theme.Link, theme.AccentText, theme.Secondary, theme.Accent)
}

// payloadEditor holds the editing state of one kind's payload. A session
// keeps one per kind for its whole life, so switching from GraphQL to HTTP
// and back gives the query back as it was.
type payloadEditor interface {
	// load shows req's payload, or empties the editor when req is another kind.
	load(req model.Request)
	payload() model.Payload
	// tabs are the kind's own request tabs, shown before the shared ones.
	tabs() []requestTab
	view(tab string, a *App) t.Widget
	focusID(tab string) string
}

// requestTab is a tab of the request panel. badge and marked are read only
// when the tab strip is drawn, so listing the tabs (for the jump map)
// doesn't subscribe to what's in them. jump is the tab's fixed jump mode
// label: a shared tab has the same one in every kind, and a kind's own tabs
// have letters nothing else uses.
type requestTab struct {
	key, label, jump string
	badge            func() string
	marked           func() bool
}

func (r requestTab) item() tabItem {
	item := tabItem{Key: r.key, Label: r.label}
	if r.badge != nil {
		item.Badge = r.badge()
	}
	if r.marked != nil {
		item.Marked = r.marked()
	}
	return item
}

// sharedTabs are the request tabs for the fields kinds share, each shown
// when the kind uses its field. Info and Options are always shown.
var sharedTabs = []struct {
	needs model.Fields
	tab   func(s *Session) requestTab
}{
	{model.FieldHeaders, func(s *Session) requestTab {
		return requestTab{key: "headers", jump: "q", label: "Headers", badge: func() string { return countBadge(s.headers.Count()) }}
	}},
	{model.FieldBody, func(s *Session) requestTab {
		return requestTab{key: "body", jump: "w", label: "Body", marked: func() bool { return s.bodyType.Get() != model.BodyNone }}
	}},
	{model.FieldPathParams, func(s *Session) requestTab {
		return requestTab{key: "path", jump: "e", label: "Path", badge: func() string { return countBadge(s.pathParams.Count()) }}
	}},
	{model.FieldQuery, func(s *Session) requestTab {
		return requestTab{key: "query", jump: "r", label: "Query", badge: func() string { return countBadge(s.query.Count()) }}
	}},
	{model.FieldAuth, func(s *Session) requestTab {
		return requestTab{key: "auth", jump: "t", label: "Auth", marked: func() bool { return s.authType.Get() != model.AuthNone }}
	}},
	{0, func(*Session) requestTab { return requestTab{key: "info", jump: "y", label: "Info"} }},
	{0, func(*Session) requestTab { return requestTab{key: "options", jump: "u", label: "Options"} }},
}

// requestTabList is the request panel's tabs for the session's kind: the
// kind's own, then the shared tabs for the fields it uses.
func (s *Session) requestTabList() []requestTab {
	kind := s.requestKind()
	var tabs []requestTab
	if e := s.payloads[kind.ID]; e != nil {
		tabs = append(tabs, e.tabs()...)
	}
	view := kindViews[kind.ID]
	for _, shared := range sharedTabs {
		if !kind.Fields.Has(shared.needs) {
			continue
		}
		tab := shared.tab(s)
		if label, ok := view.rename[tab.key]; ok {
			tab.label = label
		}
		tabs = append(tabs, tab)
	}
	return tabs
}

// selectRequestTab shows the request tab key, or the kind's first tab when
// the kind has no such tab, and returns the tab shown. Everything outside the
// tab strip selects request tabs through it, so the panel never shows a tab
// the kind doesn't have.
func (s *Session) selectRequestTab(key string) string {
	tabs := s.requestTabList()
	if !slices.ContainsFunc(tabs, func(tab requestTab) bool { return tab.key == key }) {
		key = tabs[0].key
	}
	s.requestTabs().selectKey(key)
	return key
}

// requestKind is the kind of request the session is editing, read reactively.
func (s *Session) requestKind() *model.Kind {
	kind, _ := model.KindByID(s.kind.Get())
	return kind
}

// badgeRequest is just enough of the session's request to draw its badge
// with model.Request.Badge and requestColor, read reactively.
func (s *Session) badgeRequest() model.Request {
	r := s.requestKind().New()
	r.Method = s.method.Get()
	return r
}

// requestColor is the colour of r's badge: its method's colour for HTTP, the
// kind's colour otherwise.
func requestColor(theme t.ThemeData, r model.Request) t.Color {
	if r.Payload == nil {
		return methodColor(theme, r.Method)
	}
	key := theme.Name + "/" + string(r.Kind().ID)
	if color, ok := kindColors.Load(key); ok {
		return color.(t.Color)
	}
	color := kindViews[r.Kind().ID].color(theme)
	kindColors.Store(key, color)
	return color
}

// kindColors caches kinds' badge colours by theme, since picking one
// compares candidates with every method's colour.
var kindColors sync.Map

// unlikeMethods picks the candidate colour whose hue is furthest from every
// method's and from the colours in taken, or failing that the one furthest
// in RGB. Themes often reuse their accent or secondary colour for a method,
// so no single theme colour stays distinct in every theme. A candidate in
// taken is never picked.
func unlikeMethods(theme t.ThemeData, taken []t.Color, candidates ...t.Color) t.Color {
	avoid := slices.Clone(taken)
	for _, method := range model.Methods {
		avoid = append(avoid, methodColor(theme, method))
	}
	best, bestHue, bestRGB := candidates[0], -1.0, -1
	for _, candidate := range candidates {
		if slices.Contains(taken, candidate) {
			continue
		}
		hue, rgb := 180.0, math.MaxInt
		for _, color := range avoid {
			hue, rgb = min(hue, hueDistance(candidate, color)), min(rgb, rgbDistance(candidate, color))
		}
		if hue > bestHue || (hue == bestHue && rgb > bestRGB) {
			best, bestHue, bestRGB = candidate, hue, rgb
		}
	}
	return best
}

// rgbDistance is the squared distance between two colours in RGB space.
func rgbDistance(a, b t.Color) int {
	ar, ag, ab := a.RGB()
	br, bg, bb := b.RGB()
	dr, dg, db := int(ar)-int(br), int(ag)-int(bg), int(ab)-int(bb)
	return dr*dr + dg*dg + db*db
}

// hueDistance is how far apart two colours' hues are, in degrees. Greys
// have no hue to speak of: two greys are alike, a grey and a colour aren't.
func hueDistance(a, b t.Color) float64 {
	const grey = 0.2
	ha, sa, _ := a.HSL()
	hb, sb, _ := b.HSL()
	switch {
	case sa < grey && sb < grey:
		return 0
	case sa < grey || sb < grey:
		return 180
	}
	d := math.Abs(ha - hb)
	return min(d, 360-d)
}

// requestLabel names what r does where there's room for more than a
// badge: its method for HTTP ("DELETE"), the kind's label otherwise.
func requestLabel(r model.Request) string {
	if r.Payload == nil {
		return string(r.Method)
	}
	return r.Kind().Label
}

package ui

import (
	"sort"
	"strings"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

// variableChoices are the ${VARIABLE} completions available right now. key
// changes whenever the list does, so fields only refresh their suggestions
// when there is something new.
type variableChoices struct {
	list []t.Suggestion
	key  string
}

// variableChoices lists every variable as a completion, with its value (or
// a mask for secrets) alongside. Reading it in Build subscribes to changes.
func (a *App) variableChoices() variableChoices {
	values := a.variableValues()
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	list := make([]t.Suggestion, 0, len(names))
	var key strings.Builder
	for _, name := range names {
		value := values[name]
		description := value
		if model.IsSensitiveName(name) {
			description = "••••••••"
		}
		if len(description) > 40 {
			description = description[:39] + "…"
		}
		list = append(list, t.Suggestion{Label: "${" + name + "}", Value: "${" + name + "}", Description: description})
		key.WriteString(name + "=" + description + "\x00")
	}
	return variableChoices{list: list, key: key.String()}
}

// completion is one field's variable completion popup.
type completion struct {
	state *t.AutocompleteState
	key   string
}

func newCompletion() *completion { return &completion{state: t.NewAutocompleteState()} }

// wrap adds ${VARIABLE} completion to a TextInput or TextArea: typing $
// offers the variables, and choosing one inserts ${NAME}.
func (c *completion) wrap(theme t.ThemeData, child t.Widget, choices variableChoices, width t.Dimension) t.Widget {
	if c == nil || len(choices.list) == 0 {
		return child
	}
	if c.key != choices.key {
		c.key = choices.key
		c.state.SetSuggestions(choices.list)
	}
	return t.Autocomplete{
		State:                 c.state,
		Child:                 child,
		Width:                 width,
		TriggerChars:          []rune{'$'},
		TriggerAnywhere:       true, // Variables sit mid-URL: /users/$ID.
		Insert:                t.InsertFromTrigger,
		MatchMode:             t.FilterFuzzy,
		MaxVisible:            8,
		DismissWhenEmpty:      true,
		DisableKeysWhenHidden: true,
		PopupWidth:            t.Cells(60),
		PopupStyle:            t.Style{BackgroundColor: theme.Surface2},
		RenderSuggestion:      renderSuggestion,
	}
}

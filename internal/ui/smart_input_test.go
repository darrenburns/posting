package ui

import (
	"fmt"
	"strings"
	"testing"

	t "github.com/darrenburns/terma"
	"github.com/rivo/uniseg"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestSmartInput(tt *testing.T) {
	for _, tc := range []struct{ name, language, before, input, after string }{
		{"quote pair", "json", `│`, `"`, `"│"`},
		{"quote overtype", "json", `"Ada│"`, `"`, `"Ada"│`},
		{"not an opening quote", "json", `│"Ada"`, `"`, `"│"Ada"`},
		{"escaped quote", "json", `"Ada\│"`, `"`, `"Ada\"│"`},
		{"even backslashes", "json", `"Ada\\│"`, `"`, `"Ada\\"│`},
		{"do not split word", "json", `"Ad│a"`, `"`, `"Ad"│a"`},
		{"single quote is literal in JSON", "json", `│`, `'`, `'│`},
		{"single quote pair", "javascript", `call(│)`, `'`, `call('│')`},
		{"template quote", "javascript", "│", "`", "`│`"},
		{"apostrophe", "yaml", `it's Darren│`, `'`, `it's Darren'│`},
		{"plain text", "", `│`, `"{[(`, `"{[(│`},
		{"parentheses", "graphql", `user│`, `(`, `user(│)`},
		{"nested brackets", "json", `│`, `{[()]}`, `{[()]}│`},
		{"unmatched closer", "json", `│]`, `]`, `]│]`},
		{"mismatched closer", "json", `[│)`, `)`, `[)│)`},
		{"bracket in string", "json", `"│"`, `{`, `"{│"`},
		{"string bracket cannot match", "json", `"["│]`, `]`, `"["]│]`},
		{"hash comment", "graphql", `# │`, `"{`, `# "{│`},
		{"slash comment", "javascript", `// │`, `"[`, `// "[│`},
		{"block comment", "css", `/* │ */`, `(`, `/* (│ */`},
		{"after block comment", "javascript", `/* " */ │`, `(`, `/* " */ (│)`},
		{"XML comment", "xml", `<!-- │ -->`, `"`, `<!-- "│ -->`},
		{"XML attribute", "xml", `<user name=│>`, `"`, `<user name="│">`},
		{"GraphQL block string", "graphql", `""" text " │"""`, `{`, `""" text " {│"""`},
		{"Unicode prefix", "json", "猫e\u0301 │", `"`, "猫e\u0301 \"│\""},
		{"expand braces", "json", `{│}`, "\n", "{\n  │\n}"},
		{"expand spaced brackets", "json", "  [│  ]", "\n", "  [\n    │\n  ]"},
		{"preserve indentation", "json", "    12,│", "\n", "    12,\n    │"},
		{"no extra indent after quote", "json", `  "name"│`, "\n", "  \"name\"\n  │"},
		{"four space document", "json", "{\n    \"x\": {│}\n}", "\n", "{\n    \"x\": {\n        │\n    }\n}"},
		{"tab document", "json", "{\n\t\"x\": [│]\n}", "\n", "{\n\t\"x\": [\n\t\t│\n\t]\n}"},
		{"inside indentation", "json", "  │  1", "\n", "  \n  │  1"},
		{"blank indented line", "json", "  │", "\n", "  \n  │"},
		{"comment opener is literal", "graphql", "  # {│", "\n", "  # {\n  │"},
		{"YAML mapping", "yaml", "  user:│", "\n", "  user:\n    │"},
		{"dedent closer", "json", "{\n    │", "}", "{\n}│"},
		{"dedent to opening line", "json", "  {\n      │", "}", "  {\n  }│"},
	} {
		tt.Run(tc.name, func(tt *testing.T) {
			before, cursor := markedInput(tc.before)
			state := t.NewTextAreaState(before)
			state.CursorIndex.Set(cursor)
			area := t.TextArea{ID: "edit", State: state, BeforeInput: smartInput(state, tc.language)}
			pilot := t.NewPilot(tt, area, 80, 12)
			pilot.Type(tc.input)
			want, wantCursor := markedInput(tc.after)
			if got := state.GetText(); got != want || state.CursorIndex.Peek() != wantCursor {
				tt.Fatalf("got %q at %d; want %q at %d", got, state.CursorIndex.Peek(), want, wantCursor)
			}
		})
	}
}

func markedInput(text string) (string, int) {
	before, after, _ := strings.Cut(text, "│")
	return before + after, uniseg.GraphemeClusterCount(before)
}

func TestSmartInputSelectionAndHistory(tt *testing.T) {
	for _, reverse := range []bool{false, true} {
		for opener, closer := range map[string]string{`"`: `"`, "[": "]", "{": "}"} {
			tt.Run(fmt.Sprintf("%s/reverse=%t", opener, reverse), func(tt *testing.T) {
				state := t.NewTextAreaState("x猫e\u0301z")
				cursor, anchor := 3, 1
				if reverse {
					cursor, anchor = anchor, cursor
				}
				state.CursorIndex.Set(cursor)
				state.SelectionAnchor.Set(anchor)
				pilot := t.NewPilot(tt, t.TextArea{ID: "edit", State: state, BeforeInput: smartInput(state, "json")}, 50, 5)
				pilot.Type(opener)
				if state.GetSelectedText() != "猫e\u0301" || state.CursorIndex.Peek() != cursor+1 || state.SelectionAnchor.Peek() != anchor+1 {
					tt.Fatalf("wrapping %q changed the selection: %q", opener, state.GetText())
				}
				wrapped := "x" + opener + "猫e\u0301" + closer + "z"
				if state.GetText() != wrapped {
					tt.Fatalf("wrapped text = %q, want %q", state.GetText(), wrapped)
				}
				pilot.Press("ctrl+z")
				if state.GetText() != "x猫e\u0301z" || state.CursorIndex.Peek() != cursor || state.SelectionAnchor.Peek() != anchor {
					tt.Fatal("undo lost the original selection")
				}
				pilot.Press("ctrl+y")
				if state.GetText() != wrapped || state.CursorIndex.Peek() != cursor+1 || state.SelectionAnchor.Peek() != anchor+1 {
					tt.Fatal("redo lost the wrapped selection")
				}
			})
		}
	}
}

func TestSmartInputInRequestEditors(tt *testing.T) {
	for _, id := range []string{"req-body-text", graphQLQueryID, graphQLVariablesID, grpcMessageID, "req-part-text"} {
		tt.Run(id, func(tt *testing.T) {
			app := testApp()
			s := app.current()
			state := s.body
			s.bodyType.Set(model.BodyRaw)
			s.contentType.Set("application/json")
			s.requestTab.Set("body")
			switch id {
			case graphQLQueryID, graphQLVariablesID:
				s.Load(model.GraphQLKind.New())
				e := s.payloads[model.KindGraphQL].(*graphQLEditor)
				state = e.query
				if id == graphQLVariablesID {
					state = e.variables
					s.requestTab.Set("gql-variables")
				}
			case grpcMessageID:
				s.Load(model.GRPCKind.New())
				state = s.payloads[model.KindGRPC].(*grpcEditor).message
			case "req-part-text":
				s.bodyType.Set(model.BodyMultipart)
				s.multipart.load([]model.Part{{Name: "metadata", ContentType: "application/json", Enabled: true}})
				state = s.multipart.list.Items.Peek()[0].text
			}
			state.SetText("")
			pilot := t.NewPilot(tt, app, 150, 46)
			t.RequestFocus(id)
			pilot.Advance(0)
			if pilot.FocusedID() != id {
				tt.Fatalf("focused %q instead of %q", pilot.FocusedID(), id)
			}
			pilot.Type("{\n\"name\": \"Ada\"")
			if want := "{\n  \"name\": \"Ada\"\n}"; state.GetText() != want {
				tt.Fatalf("typed payload = %q, want %q", state.GetText(), want)
			}
			if !s.dirty.Peek() {
				tt.Fatal("typing did not mark the request dirty")
			}
			pilot.Press("ctrl+a")
			literal := "{\n\"pasted\": [1]}"
			pilot.Paste(literal)
			if state.GetText() != literal {
				tt.Fatalf("paste was transformed: %q", state.GetText())
			}
			pilot.Press("ctrl+a", "backspace")
			pilot.Type(`"`)
			pilot.Press("ctrl+z", "ctrl+y")
			if state.GetText() != `""` || state.CursorIndex.Peek() != 1 {
				tt.Fatal("redo did not put the cursor between the quotes")
			}
		})
	}
}

func TestSmartInputWithVariableCompletion(tt *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"${BA", "${BASE_URL}"},
		{"${$uu", "${$uuid}"},
		{`"${BA`, `"${BASE_URL}"`},
	} {
		tt.Run(tc.input, func(tt *testing.T) {
			app := testApp()
			s := app.current()
			s.bodyType.Set(model.BodyRaw)
			s.contentType.Set("application/json")
			s.requestTab.Set("body")
			p := t.NewPilot(tt, app, snapW, snapH)
			t.RequestFocus("req-body-text")
			p.Advance(0)
			p.Type(tc.input)
			p.Press("enter")
			if s.body.GetText() != tc.want {
				tt.Fatalf("completion = %q, want %q", s.body.GetText(), tc.want)
			}
		})
	}
}

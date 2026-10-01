// Package importing describes the result of converting another HTTP client's
// format to Posting requests. Format readers never write collection files.
package importing

import (
	"strings"

	"github.com/darrenburns/posting/internal/model"
)

// Result is a complete conversion, including diagnostics for source features
// Posting cannot represent. Requests' File fields are suggested relative paths;
// the collection writer sanitizes them and avoids overwriting existing files.
//
// Every string uses Posting's placeholder syntax, variable values included:
// ${NAME} (or $NAME) is a reference and $$ is a literal dollar sign, as
// model.FindVariables and model.Substitute read them.
//
// Variables are the base layer that every environment builds on. Each of
// Environments is a named layer on top of it. DeferredVariables preserves
// template evaluation until a request's VariableScope is available.
type Result struct {
	Name         string
	Requests     []model.Request
	Variables    []model.Variable
	Environments []Environment
	Warnings     []string

	DeferredVariables bool
}

// Environment is a named set of variables layered over a Result's Variables.
type Environment struct {
	Name      string
	Variables []model.Variable
}

// BracedOnly rewrites text escaped for model.Substitute, where every literal
// "$" is written "$$", for a field that only substitutes ${NAME}, such as a
// GraphQL query. There a lone "$" not followed by "{" is already literal, so
// the escape is dropped and "$$id" reads as the "$id" it sends. Runs of
// several dollars, and dollars before "{", keep their escapes.
func BracedOnly(escaped string) string {
	var b strings.Builder
	for i := 0; i < len(escaped); {
		if escaped[i] != '$' {
			b.WriteByte(escaped[i])
			i++
			continue
		}
		j := i
		for j < len(escaped) && escaped[j] == '$' {
			j++
		}
		run := escaped[i:j]
		if run == "$$" && (j == len(escaped) || escaped[j] != '{') {
			run = "$"
		}
		b.WriteString(run)
		i = j
	}
	return b.String()
}

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
// Variables are collection defaults; narrower scopes must be resolved by the
// format reader so sibling requests cannot change one another's values.
type Result struct {
	Name      string
	Requests  []model.Request
	Variables []model.Variable
	Warnings  []string
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

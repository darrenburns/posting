// Package importing describes the result of converting another HTTP client's
// format to Posting requests. Format readers never write collection files.
package importing

import "github.com/darrenburns/posting/internal/model"

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

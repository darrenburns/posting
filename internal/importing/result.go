// Package importing describes the result of converting another HTTP client's
// format to Posting requests. Format readers never write collection files.
package importing

import "github.com/darrenburns/posting/internal/model"

// Result is a complete conversion, including diagnostics for source features
// Posting cannot represent. Requests' File fields are suggested relative paths;
// the collection writer sanitizes them and avoids overwriting existing files.
//
// Every string uses Posting's placeholder syntax, variable values included:
// ${NAME} (or $NAME) is a reference and $$ is a literal dollar sign, as
// model.FindVariables and model.Substitute read them.
//
// Variables are the base layer that every environment builds on. Each of
// Environments is a named layer on top of it. Scopes narrower than an
// environment must be resolved by the format reader, so sibling requests
// cannot change one another's values.
type Result struct {
	Name         string
	Requests     []model.Request
	Variables    []model.Variable
	Environments []Environment
	Warnings     []string
}

// Environment is a named set of variables layered over a Result's Variables.
type Environment struct {
	Name      string
	Variables []model.Variable
}

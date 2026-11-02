// Package skills holds Agent Skills that teach coding agents to use
// Posting. They're built into the binary, so `posting remote skill` prints
// the skill for the version installed.
package skills

import _ "embed"

// UsingPosting is using-posting/SKILL.md.
//
//go:embed using-posting/SKILL.md
var UsingPosting string

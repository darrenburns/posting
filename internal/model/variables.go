package model

import (
	"slices"
	"strings"
)

// VariableRef is a $NAME or ${NAME} reference found in a string.
type VariableRef struct {
	Name string
	// Start and End are byte offsets of the whole reference (including $ and braces).
	Start, End int
}

// FindVariables returns every variable reference in s. "$$" is an escaped
// dollar sign and never starts a reference.
func FindVariables(s string) []VariableRef {
	var refs []VariableRef
	for i := 0; i < len(s); i++ {
		if s[i] != '$' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '$' {
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '{' {
			end := strings.IndexByte(s[i+2:], '}')
			if end < 0 {
				continue
			}
			name := s[i+2 : i+2+end]
			if isIdentifier(name) {
				refs = append(refs, VariableRef{Name: name, Start: i, End: i + 3 + end})
			}
			i += 2 + end
			continue
		}
		j := i + 1
		for j < len(s) && isIdentByte(s[j], j == i+1) {
			j++
		}
		if j > i+1 {
			refs = append(refs, VariableRef{Name: s[i+1 : j], Start: i, End: j})
			i = j - 1
		}
	}
	return refs
}

// Substitute replaces variable references with values from lookup. Unknown
// variables are left untouched and "$$" becomes "$".
func Substitute(s string, lookup func(name string) (string, bool)) string {
	return substitute(s, FindVariables(s), lookup)
}

func substitute(s string, refs []VariableRef, lookup func(name string) (string, bool)) string {
	if !strings.Contains(s, "$") {
		return s
	}
	var b strings.Builder
	last := 0
	for _, ref := range refs {
		b.WriteString(strings.ReplaceAll(s[last:ref.Start], "$$", "$"))
		if value, ok := lookup(ref.Name); ok {
			b.WriteString(value)
		} else {
			b.WriteString(s[ref.Start:ref.End])
		}
		last = ref.End
	}
	b.WriteString(strings.ReplaceAll(s[last:], "$$", "$"))
	return b.String()
}

// FindBracedVariables returns the ${NAME} references in s, skipping $NAME
// ones, for text where a bare $name means something else (a GraphQL
// variable).
func FindBracedVariables(s string) []VariableRef {
	var refs []VariableRef
	for _, ref := range FindVariables(s) {
		if s[ref.Start+1] == '{' {
			refs = append(refs, ref)
		}
	}
	return refs
}

// SubstituteBraced is Substitute for ${NAME} references only: $NAME is left
// as written. "$$" still becomes "$".
func SubstituteBraced(s string, lookup func(name string) (string, bool)) string {
	return substitute(s, FindBracedVariables(s), lookup)
}

// IsSensitiveName reports whether a variable name looks like a secret whose
// value should be masked by default.
func IsSensitiveName(name string) bool {
	lower := strings.ToLower(name)
	for _, marker := range []string{"secret", "password", "passwd", "token", "api_key", "apikey"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isIdentByte(s[i], i == 0) {
			return false
		}
	}
	return true
}

func isIdentByte(c byte, first bool) bool {
	switch {
	case c == '_', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return !first
	}
	return false
}

// Merge layers sets of variables in increasing priority: a variable in a
// later layer replaces one of the same name in an earlier layer and records
// the source it replaced. The result is sorted by name.
func Merge(layers ...[]Variable) []Variable {
	index := map[string]int{}
	var out []Variable
	for _, layer := range layers {
		for _, v := range layer {
			i, ok := index[v.Name]
			if !ok {
				index[v.Name] = len(out)
				out = append(out, v)
				continue
			}
			prev := out[i]
			overrides := slices.Clone(v.Overrides)
			if prev.Source != v.Source {
				overrides = append(overrides, prev.Source)
			}
			v.Overrides = append(overrides, prev.Overrides...)
			out[i] = v
		}
	}
	slices.SortFunc(out, func(a, b Variable) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// Values is the name → value map of merged variables.
func Values(vars []Variable) map[string]string {
	values := make(map[string]string, len(vars))
	for _, v := range vars {
		values[v.Name] = v.Value
	}
	return values
}

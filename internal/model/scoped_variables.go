package model

import (
	"fmt"
	"maps"
	"strings"
)

// VariableScope contains request and inherited folder templates. Environment
// templates resolve against this scope when a request is sent.
type VariableScope struct {
	Variables map[string]string `yaml:"variables,omitempty"`
}

func (s *VariableScope) Clone() *VariableScope {
	if s == nil {
		return nil
	}
	return &VariableScope{Variables: maps.Clone(s.Variables)}
}

// ScopedVariables evaluates templates after layering request variables above
// environments and below session overrides. Ordinary values remain literal.
func ScopedVariables(variables []Variable, scope *VariableScope, fallback func(string) (string, bool)) ([]Variable, error) {
	return scopedVariables(variables, scope, fallback, nil)
}

func scopedVariables(variables []Variable, scope *VariableScope, fallback func(string) (string, bool), roots []string) ([]Variable, error) {
	var base, scoped, session []Variable
	for _, v := range variables {
		if v.SessionOverride {
			session = append(session, v)
		} else {
			base = append(base, v)
		}
	}
	if scope != nil {
		for name, template := range scope.Variables {
			scoped = append(scoped, Variable{Name: name, Value: template, Template: &template, Source: "request"})
		}
	}
	out := Merge(base, scoped, session)
	byName := make(map[string]int, len(out))
	for i, v := range out {
		byName[v.Name] = i
		if v.Template != nil {
			out[i].Value = *v.Template
		}
	}
	state := make([]uint8, len(out))
	work, bytes := 0, 0
	var resolve func(string, int) (string, bool, error)
	resolve = func(name string, depth int) (string, bool, error) {
		i, ok := byName[name]
		if !ok {
			if fallback != nil {
				value, found := fallback(name)
				return value, found, nil
			}
			return "", false, nil
		}
		v := &out[i]
		if v.Template == nil || state[i] == 2 {
			return v.Value, true, nil
		}
		if state[i] == 1 {
			return "", false, fmt.Errorf("cyclic variable reference %s", name)
		}
		if depth >= 32 {
			return "", false, fmt.Errorf("variable expansion depth exceeds 32")
		}
		state[i] = 1
		defer func() {
			if state[i] == 1 {
				state[i] = 0
			}
		}()
		template := *v.Template
		var b strings.Builder
		last := 0
		for _, ref := range FindVariables(template) {
			work++
			if work > 10000 {
				return "", false, fmt.Errorf("variable expansion exceeds 10000 references")
			}
			value, found, err := resolve(ref.Name, depth+1)
			if err != nil {
				return "", false, err
			}
			if !found {
				return "", false, UndefinedVariablesError{Names: []string{ref.Name}}
			}
			literal := strings.ReplaceAll(template[last:ref.Start], "$$", "$")
			if b.Len()+len(literal)+len(value) > 16<<20 {
				return "", false, fmt.Errorf("expanded variable exceeds 16 MiB")
			}
			b.WriteString(literal)
			b.WriteString(value)
			last = ref.End
		}
		tail := strings.ReplaceAll(template[last:], "$$", "$")
		if b.Len()+len(tail) > 16<<20 {
			return "", false, fmt.Errorf("expanded variable exceeds 16 MiB")
		}
		b.WriteString(tail)
		bytes += b.Len()
		if bytes > 16<<20 {
			return "", false, fmt.Errorf("aggregate variable expansion exceeds 16 MiB")
		}
		v.Value = b.String()
		state[i] = 2
		return v.Value, true, nil
	}
	if roots == nil {
		for _, v := range out {
			roots = append(roots, v.Name)
		}
	}
	var firstError error
	for _, name := range roots {
		if _, _, err := resolve(name, 0); err != nil && firstError == nil {
			firstError = err
		}
	}
	return out, firstError
}

// VariablesForRequest resolves only the templates used by this snapshot, so
// an unused alias cannot prevent sending an otherwise independent request.
func VariablesForRequest(req Request, variables []Variable, fallback func(string) (string, bool)) ([]Variable, error) {
	roots := make([]string, 0)
	_, _ = Resolve(req, func(name string) (string, bool) {
		roots = append(roots, name)
		return "", true
	})
	return scopedVariables(variables, req.VariableScope, fallback, roots)
}

package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

// EnvironmentSource finds and loads environments. It is the UI's only way to
// reach environment files, so the UI itself never touches the filesystem.
type EnvironmentSource interface {
	// Candidates lists the environments available to switch to. Each is a
	// set of files layered in order.
	Candidates() [][]string
	// Load reads an environment's files afresh.
	Load(files []string) (model.Environment, error)
}

// StaticEnvironments is an EnvironmentSource over environments already in
// memory. Each environment is identified by its Files.
type StaticEnvironments []model.Environment

func (s StaticEnvironments) Candidates() [][]string {
	out := make([][]string, len(s))
	for i, e := range s {
		out[i] = e.Files
	}
	return out
}

func (s StaticEnvironments) Load(files []string) (model.Environment, error) {
	for _, e := range s {
		if slices.Equal(e.Files, files) {
			return e, nil
		}
	}
	return model.Environment{Name: strings.Join(files, " + "), Files: files}, nil
}

// envKey identifies a set of files, for comparing environments.
func envKey(files []string) string { return strings.Join(files, "\x00") }

// environments is the app's environment state: the active environment,
// the host's variables and the sets of files switched to this session.
type environments struct {
	source  EnvironmentSource
	active  t.AnySignal[model.Environment]
	host    []model.Variable
	history [][]string
	// remember records the environment switched to, for the next launch.
	remember func(files []string)

	// watched mirrors the active files for the watcher goroutine.
	mu      sync.Mutex
	watched []string
}

// resolvedVariables layers host variables, the active environment's files
// and session overrides, in increasing priority, keeping where each value
// comes from and what it overrides. Reading it in Build subscribes to
// environment and session changes.
func (a *App) resolvedVariables() []model.Variable {
	return a.layerVariables(a.env.active.Get().Variables, a.sessionVars.Get())
}

// variableList is resolvedVariables without subscribing (for actions).
func (a *App) variableList() []model.Variable {
	return a.layerVariables(a.env.active.Peek().Variables, a.sessionVars.Peek())
}

// variableValues is the value of every variable. Reading it in Build
// subscribes to environment and session changes.
func (a *App) variableValues() map[string]string { return model.Values(a.resolvedVariables()) }

// variableValuesPeek is variableValues without subscribing (for actions).
func (a *App) variableValuesPeek() map[string]string { return model.Values(a.variableList()) }

func (a *App) layerVariables(envVars []model.Variable, session map[string]string) []model.Variable {
	sessionVars := make([]model.Variable, 0, len(session))
	for name, value := range session {
		sessionVars = append(sessionVars, model.Variable{Name: name, Value: value, Source: "session"})
	}
	return model.Merge(a.env.host, envVars, sessionVars)
}

// variableSource describes where a variable's value comes from, and the
// layer beneath it that it overrides, if any.
func variableSource(v model.Variable) string {
	if len(v.Overrides) == 0 {
		return v.Source
	}
	return v.Source + " over " + v.Overrides[0]
}

func (a *App) resolver() func(string) bool {
	values := a.variableValues()
	return func(name string) bool { _, ok := values[name]; return ok }
}

// envName is the active environment's name, or "" when none is active.
func (a *App) envName() string {
	return a.env.active.Get().Name
}

// activateEnvironment loads files and makes them the active environment.
// No files clears the environment. Session variables are kept either way.
func (a *App) activateEnvironment(files []string) bool {
	if len(files) == 0 {
		a.setEnvironment(model.Environment{})
		return true
	}
	loaded, err := a.env.source.Load(files)
	if err != nil {
		a.notify("Couldn't load environment: "+err.Error(), toastError)
		return false
	}
	a.setEnvironment(loaded)
	return true
}

func (a *App) setEnvironment(e model.Environment) {
	a.env.active.Set(e)
	a.env.mu.Lock()
	a.env.watched = e.Files
	a.env.mu.Unlock()
	if len(e.Files) == 0 {
		return
	}
	for _, used := range a.env.history {
		if envKey(used) == envKey(e.Files) {
			return
		}
	}
	a.env.history = append(a.env.history, e.Files)
}

// switchEnvironment makes files the active environment at the user's
// request, and remembers the choice for the next launch.
func (a *App) switchEnvironment(files []string) {
	if !a.activateEnvironment(files) {
		return
	}
	if a.env.remember != nil {
		a.env.remember(files)
	}
	if len(files) == 0 {
		a.notify("Environment cleared", toastInfo)
		return
	}
	a.notify("Switched to "+a.env.active.Peek().Name, toastInfo)
}

// environmentItems lists the environments used this session, then the other
// environment files found, then the option to clear the environment.
func (a *App) environmentItems() []t.CommandPaletteItem {
	groups := append([][]string(nil), a.env.history...)
	seen := map[string]bool{}
	for _, g := range groups {
		seen[envKey(g)] = true
	}
	for _, candidate := range a.env.source.Candidates() {
		if !seen[envKey(candidate)] {
			seen[envKey(candidate)] = true
			groups = append(groups, candidate)
		}
	}
	active := envKey(a.env.active.Peek().Files)
	items := make([]t.CommandPaletteItem, 0, len(groups)+1)
	for _, files := range groups {
		files := files
		item := t.CommandPaletteItem{
			Hint:    shortDir(files),
			Current: envKey(files) == active,
			Action:  a.run(func() { a.switchEnvironment(files) }),
		}
		if loaded, err := a.env.source.Load(files); err != nil {
			item.Label = strings.Join(files, " + ")
			item.Description = err.Error()
		} else {
			item.Label = loaded.Name
			item.Description = pluralize(len(loaded.Variables), "variable")
			if layers := fileNames(files); layers != loaded.Name {
				item.Description += " · " + layers
			}
		}
		items = append(items, item)
	}
	items = append(items, t.CommandPaletteItem{
		Label:   "No environment",
		Hint:    "session variables only",
		Current: active == "",
		Action:  a.run(func() { a.switchEnvironment(nil) }),
	})
	return items
}

// fileNames lists an environment's files by name, in layering order.
func fileNames(files []string) string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = filepath.Base(f)
	}
	return strings.Join(names, " + ")
}

// shortDir is where an environment's files live, abbreviated to fit beside
// its name: the home directory becomes ~ and long paths keep their end.
func shortDir(files []string) string {
	if len(files) == 0 {
		return ""
	}
	dir := filepath.Dir(files[0])
	for _, f := range files[1:] {
		if filepath.Dir(f) != dir {
			return "several folders"
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, dir); err == nil && !strings.HasPrefix(rel, "..") {
			dir = filepath.Join("~", rel)
		}
	}
	const maxWidth = 32
	if len(dir) > maxWidth {
		parts := strings.Split(dir, string(filepath.Separator))
		short := parts[len(parts)-1]
		for i := len(parts) - 2; i >= 0 && len(parts[i])+len(short)+5 <= maxWidth; i-- {
			short = parts[i] + string(filepath.Separator) + short
		}
		dir = "…" + string(filepath.Separator) + short
	}
	return dir
}

func (a *App) openEnvironmentPicker() {
	a.palette.SetItems(a.paletteItems())
	a.palette.Open()
	a.palette.PushLevel("Environments", a.environmentItems())
}

// watchEnvironment reloads the active environment when its files change.
// Polling keeps it simple and portable; env files are tiny.
func (a *App) watchEnvironment(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			a.env.mu.Lock()
			files := a.env.watched
			a.env.mu.Unlock()
			if len(files) == 0 {
				continue
			}
			loaded, err := a.env.source.Load(files)
			if err != nil {
				continue
			}
			t.Dispatch(func() {
				current := a.env.active.Peek()
				if envKey(current.Files) != envKey(files) || reflect.DeepEqual(current.Variables, loaded.Variables) {
					return
				}
				a.env.active.Set(loaded)
				if a.overlay.Peek() == "variables" {
					a.refreshVariableRows()
				}
				a.notify("Reloaded "+loaded.Name, toastInfo)
			})
		}
	}()
}

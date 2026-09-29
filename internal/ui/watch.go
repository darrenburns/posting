package ui

import (
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/collection"
	"github.com/darrenburns/posting/internal/model"
)

// CollectionWatcher reads the collection from disk for the watcher.
type CollectionWatcher interface {
	// Signature changes whenever the collection's files do.
	Signature() string
	Load() (*model.Collection, []collection.LoadError)
}

// watchCollection reloads the collection when its files change on disk, for
// example after a git checkout or an edit in another editor. Polling a
// signature keeps it portable and cheap.
func (a *App) watchCollection(w CollectionWatcher, interval time.Duration) {
	last := w.Signature()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			signature := w.Signature()
			if signature == last {
				continue
			}
			last = signature
			root, _ := w.Load()
			t.Dispatch(func() { a.replaceCollection(root) })
		}
	}()
}

// sameOnDisk reports whether two requests would be saved identically, which
// is what matters: the editor holds the query in the URL as well as the
// table, and files don't.
func sameOnDisk(a, b model.Request) bool {
	x, errX := collection.MarshalRequest(a)
	y, errY := collection.MarshalRequest(b)
	return errX == nil && errY == nil && string(x) == string(y)
}

// replaceCollection swaps in a collection reloaded from disk. Open tabs of
// requests that changed on disk are updated unless they have unsaved edits,
// which are never thrown away.
func (a *App) replaceCollection(root *model.Collection) {
	onDisk := map[string]model.Request{}
	root.Walk(func(_ *model.Collection, r model.Request) { onDisk[r.File] = r })
	updated := 0
	for _, s := range a.sessions.Peek() {
		file := s.file.Peek()
		if file == "" || s.dirty.Peek() {
			continue
		}
		req, ok := onDisk[file]
		if !ok || sameOnDisk(req, s.Snapshot()) {
			continue
		}
		s.Load(req)
		updated++
	}
	a.collection.Set(root)
	a.tree.Nodes.Set(buildTree(root))
	if updated > 0 {
		a.notify("Reloaded "+pluralize(updated, "open request")+" changed on disk", toastInfo)
	}
}

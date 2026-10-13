package ui

import (
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/darrenburns/posting/v3/internal/model"
	"github.com/rivo/uniseg"
)

const (
	maxURLUndoSteps = 500
	maxURLUndoBytes = 2 << 20
)

type urlEditKind uint8

const (
	urlSingleEdit urlEditKind = iota
	urlTyping
	urlBackspace
	urlDelete
)

// A query snapshot includes unnamed and blank rows, which Values omits.
type urlSnapshot struct {
	text           string
	cursor, anchor int
	query          []model.KeyValue
}

func (s urlSnapshot) sameContent(other urlSnapshot) bool {
	return s.text == other.text && slices.Equal(s.query, other.query)
}

func (s urlSnapshot) size() int {
	n := len(s.text) + 64
	for _, row := range s.query {
		n += len(row.Name) + len(row.Value) + 64
	}
	return n
}

type urlEdit struct {
	before, after urlSnapshot
	kind          urlEditKind
	edge          string
	at            time.Time
}

type urlHistory struct {
	current    urlSnapshot
	undo, redo []urlEdit
	generation uint64
	editing    bool
	merge      bool
}

func (s *Session) urlSnapshot() urlSnapshot {
	state := urlSnapshot{
		text: s.url.GetText(), cursor: s.url.CursorIndex.Peek(), anchor: s.url.SelectionAnchor.Peek(),
	}
	for _, row := range s.query.rows.Peek() {
		state.query = append(state.query, model.KeyValue{
			Name: row.key.GetText(), Value: row.value.GetText(), Enabled: row.enabled.Peek(),
		})
	}
	return state
}

func (s *Session) resetURLHistory() {
	h := &s.urlHistory
	h.current = s.urlSnapshot()
	h.undo, h.redo, h.merge = nil, nil, false
	h.generation++
	s.url.ClearHistory()
}

func (s *Session) editURL(kind urlEditKind, edit func()) {
	h := &s.urlHistory
	if h.editing {
		edit()
		return
	}
	before, generation := s.urlSnapshot(), h.generation
	h.editing = true
	defer func() { h.editing = false }()
	edit()
	// A curl paste can load a different request inside this transaction.
	if h.generation != generation {
		return
	}
	h.record(before, s.urlSnapshot(), kind)
	s.url.ClearHistory()
}

func (h *urlHistory) record(before, after urlSnapshot, kind urlEditKind) {
	h.current = after
	if before.sameContent(after) {
		h.merge = false
		return
	}
	first, last := urlEditEdges(kind, before, after)
	if first == "" {
		kind = urlSingleEdit
	}
	now := time.Now()
	if h.merge && len(h.undo) > 0 && kind != urlSingleEdit && (before.anchor < 0 || before.anchor == before.cursor) {
		top := &h.undo[len(h.undo)-1]
		if top.kind == kind && top.after.sameContent(before) &&
			top.after.cursor == before.cursor && top.after.anchor == before.anchor &&
			now.Sub(top.at) <= time.Second && !(isURLSpace(top.edge) && !isURLSpace(first)) {
			top.after, top.edge, top.at = after, last, now
			h.redo = nil
			h.trim()
			return
		}
	}
	h.undo = append(h.undo, urlEdit{before: before, after: after, kind: kind, edge: last, at: now})
	h.redo, h.merge = nil, true
	h.trim()
}

func (h *urlHistory) trim() {
	total := 0
	for _, edit := range h.undo {
		total += edit.before.size() + edit.after.size()
	}
	drop := 0
	for drop < len(h.undo) && (len(h.undo)-drop > maxURLUndoSteps || total > maxURLUndoBytes) {
		total -= h.undo[drop].before.size() + h.undo[drop].after.size()
		drop++
	}
	h.undo = slices.Delete(h.undo, 0, drop)
}

// Edges follow the order characters were typed or deleted, including backwards.
func urlEditEdges(kind urlEditKind, before, after urlSnapshot) (string, string) {
	if kind != urlTyping && before.anchor >= 0 && before.anchor != before.cursor {
		return "", ""
	}
	var text string
	switch kind {
	case urlTyping:
		text = after.text
	case urlBackspace, urlDelete:
		text = before.text
	default:
		return "", ""
	}
	var graphemes []string
	g := uniseg.NewGraphemes(text)
	for g.Next() {
		graphemes = append(graphemes, g.Str())
	}
	start, end := before.cursor, after.cursor
	switch kind {
	case urlTyping:
		if before.anchor >= 0 {
			start = min(start, before.anchor)
		}
	case urlBackspace:
		start, end = after.cursor, before.cursor
	case urlDelete:
		end = start + uniseg.GraphemeClusterCount(before.text) - uniseg.GraphemeClusterCount(after.text)
	}
	if start < 0 || end > len(graphemes) || start >= end {
		return "", ""
	}
	if kind == urlBackspace {
		return graphemes[end-1], graphemes[start]
	}
	return graphemes[start], graphemes[end-1]
}

func isURLSpace(text string) bool {
	return text != "" && strings.IndexFunc(text, func(r rune) bool { return !unicode.IsSpace(r) }) < 0
}

func (s *Session) stepURLHistory(redo bool, notify func(string)) {
	h := &s.urlHistory
	var state urlSnapshot
	if redo {
		if len(h.redo) == 0 {
			return
		}
		edit := h.redo[len(h.redo)-1]
		h.redo = slices.Delete(h.redo, len(h.redo)-1, len(h.redo))
		h.undo = append(h.undo, edit)
		state = edit.after
	} else {
		if len(h.undo) == 0 {
			return
		}
		edit := h.undo[len(h.undo)-1]
		h.undo = slices.Delete(h.undo, len(h.undo)-1, len(h.undo))
		h.redo = append(h.redo, edit)
		state = edit.before
	}
	s.syncing = true
	s.url.SetText(state.text)
	s.url.CursorIndex.Set(state.cursor)
	s.url.SelectionAnchor.Set(state.anchor)
	s.query.Load(state.query)
	s.syncPathParams()
	if notify != nil {
		notify(state.text)
	}
	s.syncing = false
	h.current, h.merge = state, false
	s.touch()
}

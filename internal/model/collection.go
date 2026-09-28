package model

import (
	"sort"
	"time"
)

// Collection is a folder of saved requests. Sub-folders are child collections.
type Collection struct {
	Name     string
	Path     string // relative to the collection root; "" for the root
	Requests []Request
	Children []*Collection
}

// Sort orders children by name and requests by method then name, recursively.
func (c *Collection) Sort() {
	sort.SliceStable(c.Children, func(i, j int) bool { return c.Children[i].Name < c.Children[j].Name })
	sort.SliceStable(c.Requests, func(i, j int) bool {
		a, b := c.Requests[i], c.Requests[j]
		if a.Method.SortRank() != b.Method.SortRank() {
			return a.Method.SortRank() < b.Method.SortRank()
		}
		return a.Name < b.Name
	})
	for _, child := range c.Children {
		child.Sort()
	}
}

// Walk visits every request in the collection tree, depth first.
func (c *Collection) Walk(fn func(folder *Collection, request Request)) {
	for _, child := range c.Children {
		child.Walk(fn)
	}
	for _, r := range c.Requests {
		fn(c, r)
	}
}

// HistoryEntry is a request that was sent and the response it received.
type HistoryEntry struct {
	ID       int64
	Request  Request
	Response *Response
	SentAt   time.Time
}

// Variable is a named value available for ${NAME} substitution.
type Variable struct {
	Name  string
	Value string
	// Source describes where the value came from ("session" or an env file name).
	Source string
}

// Environment is a named set of variables loaded from one or more env files.
type Environment struct {
	Name      string
	Files     []string
	Variables []Variable
}

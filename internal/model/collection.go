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

// Sort orders children by name and requests by method (or kind) then name,
// recursively.
func (c *Collection) Sort() {
	sort.SliceStable(c.Children, func(i, j int) bool { return c.Children[i].Name < c.Children[j].Name })
	sort.SliceStable(c.Requests, func(i, j int) bool {
		a, b := c.Requests[i], c.Requests[j]
		if a.SortRank() != b.SortRank() {
			return a.SortRank() < b.SortRank()
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
	// Status is StatusOf(Request, Response), read once when the entry is
	// made or loaded rather than each time it's drawn, since a GraphQL
	// status parses the whole body. It isn't saved.
	Status Status `json:"-"`
}

// Variable is a named value available for ${NAME} substitution.
type Variable struct {
	// SessionOverride gives an in-app override priority over request scope.
	// Source is a display label and may also be an environment file name.
	SessionOverride bool
	// Template is retained only for deferred imported variables. Value is its
	// evaluated value in the current scope. Ordinary dotenv values are literal.
	Template *string
	Name     string
	Value    string
	// Source describes where the value came from ("session", "host" or an
	// env file name).
	Source string
	// Overrides are the sources of the values this one replaced, nearest
	// first: with posting.env and staging.env layered, a variable set in
	// both comes from staging.env and overrides posting.env.
	Overrides []string
}

// Environment is a named set of variables loaded from one or more env files,
// layered in order.
type Environment struct {
	Name      string
	Files     []string
	Variables []Variable
}

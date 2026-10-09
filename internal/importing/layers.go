package importing

import (
	"container/heap"
	"fmt"
	"strings"

	"github.com/darrenburns/posting/v3/internal/model"
)

// layers is what each environment file must contain, in write order: the
// base first, then one entry per environment.
//
// Posting expands a reference when it loads the line, using what the earlier
// lines and files defined. Postman and Bruno resolve references when a request
// is sent. Two things keep the eager reading equal to the lazy one. A variable
// is written after the variables it references. And an environment file
// repeats each base variable that depends, directly or not, on a name the
// environment defines, so the repeat picks up the environment's value.
func layers(base []model.Variable, environments []Environment) ([][]model.Variable, []string) {
	var warnings []string
	order := func(label string, vars []model.Variable) []model.Variable {
		out, cycle := ordered(vars)
		if len(cycle) > 0 {
			warnings = append(warnings, fmt.Sprintf("%s: variables %s reference each other in a cycle; they are written in source order", label, strings.Join(cycle, ", ")))
		}
		return out
	}
	files := [][]model.Variable{order("base environment", base)}
	for _, e := range environments {
		defined := map[string]bool{}
		for _, v := range e.Variables {
			defined[v.Name] = true
		}
		repeat := make([]bool, len(base))
		for changed := true; changed; {
			changed = false
			for i, v := range base {
				if repeat[i] || defined[v.Name] {
					continue
				}
				for _, ref := range model.FindVariables(v.Value) {
					if defined[ref.Name] {
						repeat[i], changed = true, true
						break
					}
				}
			}
			for i, v := range base {
				if repeat[i] {
					defined[v.Name] = true
				}
			}
		}
		vars := append([]model.Variable{}, e.Variables...)
		for i, v := range base {
			if repeat[i] {
				vars = append(vars, v)
			}
		}
		files = append(files, order("environment "+e.Name, vars))
	}
	return files, warnings
}

// ordered sorts variables so each follows the variables it references,
// keeping source order wherever references allow. A cycle is broken at its
// earliest variable, and the names of variables in cycles are returned in
// source order.
func ordered(vars []model.Variable) ([]model.Variable, []string) {
	byName := map[string][]int{}
	for i, v := range vars {
		byName[v.Name] = append(byName[v.Name], i)
	}
	deps := make([][]int, len(vars))
	dependents := make([][]int, len(vars))
	pending := make([]int, len(vars))
	for i, v := range vars {
		seen := map[int]bool{}
		for _, ref := range model.FindVariables(v.Value) {
			for _, j := range byName[ref.Name] {
				if j != i && !seen[j] {
					seen[j] = true
					deps[i] = append(deps[i], j)
					dependents[j] = append(dependents[j], i)
					pending[i]++
				}
			}
		}
	}
	done := make([]bool, len(vars))
	ready := &indexHeap{}
	for i := range vars {
		if pending[i] == 0 {
			heap.Push(ready, i)
		}
	}
	out := make([]model.Variable, 0, len(vars))
	inCycle := make([]bool, len(vars))
	first := 0
	for len(out) < len(vars) {
		var next int
		if ready.Len() > 0 {
			next = heap.Pop(ready).(int)
		} else {
			for done[first] {
				first++
			}
			members := cycleFrom(first, deps, done)
			next = members[0]
			for _, m := range members {
				inCycle[m] = true
				next = min(next, m)
			}
		}
		if done[next] {
			continue
		}
		done[next] = true
		out = append(out, vars[next])
		for _, d := range dependents[next] {
			if pending[d]--; pending[d] == 0 && !done[d] {
				heap.Push(ready, d)
			}
		}
	}
	var cycle []string
	for i, v := range vars {
		if inCycle[i] {
			cycle = append(cycle, v.Name)
		}
	}
	return out, cycle
}

// cycleFrom follows unwritten references from a variable that is waiting on
// one until a variable repeats, and returns the variables in that loop.
func cycleFrom(start int, deps [][]int, done []bool) []int {
	var path []int
	at := map[int]int{}
	for i := start; ; {
		if n, ok := at[i]; ok {
			return path[n:]
		}
		at[i] = len(path)
		path = append(path, i)
		for _, j := range deps[i] {
			if !done[j] {
				i = j
				break
			}
		}
	}
}

type indexHeap []int

func (h indexHeap) Len() int           { return len(h) }
func (h indexHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h indexHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *indexHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *indexHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

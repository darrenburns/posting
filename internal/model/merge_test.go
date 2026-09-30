package model

import (
	"reflect"
	"testing"
)

func TestMergeRecordsOverrides(t *testing.T) {
	host := []Variable{{Name: "A", Value: "h", Source: "host"}, {Name: "H", Value: "h", Source: "host"}}
	env := []Variable{{Name: "A", Value: "e", Source: "staging.env", Overrides: []string{"posting.env"}}}
	session := []Variable{{Name: "B", Value: "b", Source: "session"}, {Name: "A", Value: "s", Source: "session"}}
	got := Merge(host, env, session)
	want := []Variable{
		{Name: "A", Value: "s", Source: "session", Overrides: []string{"staging.env", "posting.env", "host"}},
		{Name: "B", Value: "b", Source: "session"},
		{Name: "H", Value: "h", Source: "host"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge =\n%+v\nwant\n%+v", got, want)
	}
	if v := Values(got); v["A"] != "s" || v["H"] != "h" || len(v) != 3 {
		t.Fatalf("Values = %v", v)
	}
}

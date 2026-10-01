package history

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/internal/model"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := ForCollection(dir, "/some/collection")
	if entries, err := store.Load(); err != nil || entries != nil {
		t.Fatalf("a new store should be empty: %v %v", entries, err)
	}
	req := model.NewRequest()
	req.URL = "https://api.test/x"
	entries := []model.HistoryEntry{{
		ID:      1,
		Request: req,
		Response: &model.Response{
			StatusCode: 200, Reason: "OK", Body: []byte(`{"ok":true}`),
			Headers: []model.Header{{Name: "A", Value: "b"}},
			Elapsed: 42 * time.Millisecond,
			Trace:   []model.TraceEvent{{Stage: model.TraceConnect, State: model.TraceComplete, Duration: time.Millisecond}},
		},
		SentAt: time.Date(2026, 9, 29, 1, 2, 3, 0, time.UTC),
	}}
	if err := store.Save(entries); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || !reflect.DeepEqual(got, entries) {
		t.Fatalf("loaded %+v, %v", got, err)
	}
	info, err := os.Stat(store.Path())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("history file mode = %v, %v", info.Mode(), err)
	}
	if other := ForCollection(dir, "/another/collection"); other.Path() == store.Path() {
		t.Fatal("collections must not share history")
	}
}

func TestTrim(t *testing.T) {
	var entries []model.HistoryEntry
	for i := 0; i < MaxEntries+20; i++ {
		entries = append(entries, model.HistoryEntry{ID: int64(i)})
	}
	if got := len(Trim(entries)); got != MaxEntries {
		t.Fatalf("kept %d entries", got)
	}
	big := []model.HistoryEntry{
		{ID: 1, Response: &model.Response{Body: make([]byte, MaxBytes/2)}},
		{ID: 2, Response: &model.Response{Body: make([]byte, MaxBytes/2)}},
		{ID: 3, Response: &model.Response{Body: make([]byte, 10)}},
	}
	if got := Trim(big); len(got) != 2 {
		t.Fatalf("kept %d entries over the byte budget", len(got))
	}
}

func TestTrimCountsGraphQLPayloads(t *testing.T) {
	gql := model.GraphQLKind.New()
	gql.Payload = model.GraphQL{Query: strings.Repeat("x", MaxBytes+1)}
	kept := Trim([]model.HistoryEntry{{ID: 1, Request: gql}, {ID: 2, Request: model.NewRequest()}})
	if len(kept) != 1 || kept[0].ID != 2 {
		t.Fatalf("a GraphQL query over the byte budget must be skipped, kept %d entries", len(kept))
	}
}

func TestSaveAndLoadGraphQL(t *testing.T) {
	store := ForCollection(t.TempDir(), "/c")
	entries := []model.HistoryEntry{{ID: 1, Request: model.GraphQLKind.Example(), SentAt: time.Date(2026, 9, 29, 1, 2, 3, 0, time.UTC)}}
	if err := store.Save(entries); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || !reflect.DeepEqual(got, entries) {
		t.Fatalf("loaded %+v, %v", got, err)
	}
}

func TestSaveOversizedExchangePreservesHistory(t *testing.T) {
	store := ForCollection(t.TempDir(), "/some/collection")
	older := []model.HistoryEntry{
		{ID: 2, Request: model.NewRequest(), Response: &model.Response{Body: []byte("second")}},
		{ID: 1, Request: model.NewRequest(), Response: &model.Response{Body: []byte("first")}},
	}
	if err := store.Save(older); err != nil {
		t.Fatal(err)
	}
	entries := append([]model.HistoryEntry{{
		ID: 3, Request: model.NewRequest(), Response: &model.Response{Body: make([]byte, MaxBytes+1)},
	}}, older...)
	if err := store.Save(entries); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, older) {
		t.Fatalf("oversized newest exchange erased existing history: loaded %d entries, want %d", len(got), len(older))
	}
}

func TestTrimSkipsOversizedEntriesWithoutMutatingInput(t *testing.T) {
	largeBody := make([]byte, MaxBytes+1)
	largeRaw := strings.Repeat("x", MaxBytes+1)
	for _, oversized := range []model.HistoryEntry{
		{Request: model.Request{Body: model.Body{Type: model.BodyRaw, Raw: largeRaw}}},
		{Response: &model.Response{Body: largeBody}},
		{Request: model.Request{Body: model.Body{Type: model.BodyRaw, Raw: largeRaw[:MaxBytes/2]}}, Response: &model.Response{Body: largeBody[:MaxBytes/2+1]}},
	} {
		oversized.ID = 99
		entries := []model.HistoryEntry{{ID: 3}, oversized, {ID: 2}, {ID: 1}}
		original := append([]model.HistoryEntry(nil), entries...)
		got := Trim(entries)
		var ids []int64
		for _, entry := range got {
			ids = append(ids, entry.ID)
		}
		if !reflect.DeepEqual(ids, []int64{3, 2, 1}) {
			t.Fatalf("oversized exchange changed retained order: %v", ids)
		}
		if !reflect.DeepEqual(entries, original) {
			t.Fatal("trimming changed the caller's history entries")
		}
	}

	entries := []model.HistoryEntry{{ID: 999, Response: &model.Response{Body: largeBody}}}
	for id := 1; id <= MaxEntries+1; id++ {
		entries = append(entries, model.HistoryEntry{ID: int64(id)})
	}
	got := Trim(entries)
	if len(got) != MaxEntries || got[0].ID != 1 || got[MaxEntries-1].ID != MaxEntries {
		t.Fatalf("oversized exchange consumed an entry slot: kept %d entries", len(got))
	}

	// Normal entries still use a contiguous byte budget; skipping a later
	// entry that would exceed the remaining space must not let older ones in.
	entries = []model.HistoryEntry{
		{ID: 3, Response: &model.Response{Body: largeBody[:MaxBytes]}},
		{ID: 2, Response: &model.Response{Body: []byte("overflow")}},
		{ID: 1},
	}
	if got := Trim(entries); len(got) != 1 || got[0].ID != 3 {
		t.Fatalf("cumulative byte budget retained %d entries, want only newest", len(got))
	}
}

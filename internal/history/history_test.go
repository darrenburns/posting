package history

import (
	"os"
	"reflect"
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

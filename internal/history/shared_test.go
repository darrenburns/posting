package history

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestHistoryProcess(t *testing.T) {
	dir := os.Getenv("POSTING_HISTORY_TEST_DIR")
	if dir == "" {
		return
	}
	store := ForCollection(dir, "/shared")
	action := os.Getenv("POSTING_HISTORY_ACTION")
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	if action == "hold" {
		lock, err := os.OpenFile(store.Path()+".lock", os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if err := lockFile(lock); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(os.Getenv("POSTING_HISTORY_READY"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waitForFile(t, os.Getenv("POSTING_HISTORY_GATE"))
	switch action {
	case "append":
		count, _ := strconv.Atoi(os.Getenv("POSTING_HISTORY_COUNT"))
		for i := 0; i < count; i++ {
			req := model.NewRequest()
			req.Name = fmt.Sprintf("%s-%d", os.Getenv("POSTING_HISTORY_LABEL"), i)
			if _, err := store.Append(model.HistoryEntry{Request: req}); err != nil {
				t.Fatal(err)
			}
		}
	case "delete":
		id, _ := strconv.ParseInt(os.Getenv("POSTING_HISTORY_ID"), 10, 64)
		if _, err := store.Delete([]int64{id}); err != nil {
			t.Fatal(err)
		}
	case "clear":
		if _, err := store.Clear(); err != nil {
			t.Fatal(err)
		}
	case "hold":
	default:
		t.Fatalf("unknown helper action %q", action)
	}
}

type historyProcess struct {
	cmd    *exec.Cmd
	done   chan error
	output bytes.Buffer
}

func startHistoryProcess(t *testing.T, dir, gate, action, label string, count int) *historyProcess {
	t.Helper()
	ready := filepath.Join(t.TempDir(), "ready")
	p := &historyProcess{cmd: exec.Command(os.Args[0], "-test.run=^TestHistoryProcess$"), done: make(chan error, 1)}
	p.cmd.Env = append(os.Environ(),
		"POSTING_HISTORY_TEST_DIR="+dir,
		"POSTING_HISTORY_ACTION="+action,
		"POSTING_HISTORY_LABEL="+label,
		"POSTING_HISTORY_COUNT="+strconv.Itoa(count),
		"POSTING_HISTORY_ID="+strconv.Itoa(count),
		"POSTING_HISTORY_GATE="+gate,
		"POSTING_HISTORY_READY="+ready,
	)
	p.cmd.Stdout, p.cmd.Stderr = &p.output, &p.output
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { p.done <- p.cmd.Wait() }()
	t.Cleanup(func() { _ = p.cmd.Process.Kill() })
	waitForFile(t, ready)
	return p
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func releaseHistoryProcesses(t *testing.T, gate string) {
	t.Helper()
	if err := os.WriteFile(gate, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (p *historyProcess) wait(t *testing.T) {
	t.Helper()
	select {
	case err := <-p.done:
		if err != nil {
			t.Fatalf("history subprocess: %v\n%s", err, p.output.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("history subprocess did not finish")
	}
}

func TestProcessAppendsRetainEverySend(t *testing.T) {
	dir := t.TempDir()
	gate := filepath.Join(dir, "go")
	var processes []*historyProcess
	for i := 0; i < 6; i++ {
		processes = append(processes, startHistoryProcess(t, dir, gate, "append", strconv.Itoa(i), 8))
	}
	releaseHistoryProcesses(t, gate)
	for _, p := range processes {
		p.wait(t)
	}
	entries, err := ForCollection(dir, "/shared").Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 48 {
		t.Fatalf("concurrent processes retained %d sends, want 48", len(entries))
	}
	names, ids := map[string]bool{}, map[int64]bool{}
	for _, entry := range entries {
		if ids[entry.ID] {
			t.Fatalf("duplicate ID %d", entry.ID)
		}
		ids[entry.ID], names[entry.Request.Name] = true, true
	}
	for i := 0; i < 6; i++ {
		for j := 0; j < 8; j++ {
			if !names[fmt.Sprintf("%d-%d", i, j)] {
				t.Fatalf("lost send %d-%d", i, j)
			}
		}
	}
	for id := int64(1); id <= 48; id++ {
		if !ids[id] {
			t.Fatalf("missing allocated ID %d", id)
		}
	}
}

func TestProcessDeleteDoesNotResurrectOrRemoveConcurrentSends(t *testing.T) {
	dir := t.TempDir()
	store := ForCollection(dir, "/shared")
	if _, err := store.Append(model.HistoryEntry{Request: model.NewRequest()}); err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(dir, "go")
	deleting := startHistoryProcess(t, dir, gate, "delete", "", 1)
	appending := startHistoryProcess(t, dir, gate, "append", "new", 12)
	releaseHistoryProcesses(t, gate)
	deleting.wait(t)
	appending.wait(t)
	entries, err := store.Load()
	if err != nil || len(entries) != 12 {
		t.Fatalf("delete and append retained %d entries: %v", len(entries), err)
	}
	for _, entry := range entries {
		if entry.ID == 1 {
			t.Fatal("deleted entry was resurrected")
		}
	}
}

func TestProcessClearErasesLatestEntriesAndStaleDeleteCannotEraseNewID(t *testing.T) {
	dir := t.TempDir()
	store := ForCollection(dir, "/shared")
	if _, err := store.Append(model.HistoryEntry{Request: model.NewRequest()}); err != nil {
		t.Fatal(err)
	}
	clearGate, deleteGate := filepath.Join(dir, "clear"), filepath.Join(dir, "delete")
	clearing := startHistoryProcess(t, dir, clearGate, "clear", "", 0)
	deleting := startHistoryProcess(t, dir, deleteGate, "delete", "", 1)
	if _, err := store.Append(model.HistoryEntry{Request: model.NewRequest()}); err != nil {
		t.Fatal(err)
	}
	releaseHistoryProcesses(t, clearGate)
	clearing.wait(t)
	if entries, err := store.Load(); err != nil || len(entries) != 0 {
		t.Fatalf("clear missed a send committed since loading: %d %v", len(entries), err)
	}
	entries, err := store.Append(model.HistoryEntry{Request: model.NewRequest()})
	if err != nil || len(entries) != 1 || entries[0].ID != 3 {
		t.Fatalf("clear reused an ID: %+v %v", entries, err)
	}
	releaseHistoryProcesses(t, deleteGate)
	deleting.wait(t)
	if entries, err := store.Load(); err != nil || len(entries) != 1 || entries[0].ID != 3 {
		t.Fatalf("stale delete erased a newer send: %+v %v", entries, err)
	}
}

func TestProcessLockReleasedOnExitAndCrash(t *testing.T) {
	for _, crash := range []bool{false, true} {
		t.Run(fmt.Sprintf("crash=%v", crash), func(t *testing.T) {
			dir := t.TempDir()
			store := ForCollection(dir, "/shared")
			gate, appendGate := filepath.Join(dir, "release"), filepath.Join(dir, "append")
			holder := startHistoryProcess(t, dir, gate, "hold", "", 0)
			waiting := startHistoryProcess(t, dir, appendGate, "append", "after-release", 1)
			releaseHistoryProcesses(t, appendGate)
			select {
			case err := <-waiting.done:
				t.Fatalf("append bypassed held lock: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			if crash {
				if err := holder.cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				select {
				case <-holder.done:
				case <-time.After(5 * time.Second):
					t.Fatal("process did not exit")
				}
			} else {
				releaseHistoryProcesses(t, gate)
				holder.wait(t)
			}
			waiting.wait(t)
			if entries, err := store.Load(); err != nil || len(entries) != 1 {
				t.Fatalf("append did not recover lock: %+v %v", entries, err)
			}
			if _, err := os.Stat(store.Path() + ".lock"); err != nil {
				t.Fatalf("stable lockfile disappeared: %v", err)
			}
		})
	}
}

func TestLegacyUnknownEntryKeepsBytesAndSeedsIDs(t *testing.T) {
	store := ForCollection(t.TempDir(), "/shared")
	raw := `{ "ID" : 41, "Request" : { "Kind" : "future", "Payload" : {"value":"<keep>"} } }`
	if err := os.WriteFile(store.Path(), []byte(`{"version":1,"entries":[`+raw+`]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := store.Append(model.HistoryEntry{ID: 999, Request: model.NewRequest()})
	if err != nil || len(entries) != 1 || entries[0].ID != 42 {
		t.Fatalf("unknown entry ID was not reserved: %+v %v", entries, err)
	}
	data, err := os.ReadFile(store.Path())
	if err != nil || !bytes.Contains(data, []byte(raw)) {
		t.Fatalf("unknown entry bytes changed: %s %v", data, err)
	}
	if _, err := store.Delete([]int64{41, 42}); err != nil {
		t.Fatal(err)
	}
	entries, err = ForCollection(filepath.Dir(store.Path()), "/shared").Append(model.HistoryEntry{Request: model.NewRequest()})
	if err != nil || len(entries) != 1 || entries[0].ID != 43 {
		t.Fatalf("deleting all entries reused IDs: %+v %v", entries, err)
	}
	data, _ = os.ReadFile(store.Path())
	if bytes.Contains(data, []byte(raw)) {
		t.Fatal("explicit delete retained unknown entry")
	}
}

func TestIDExhaustionDoesNotChangeHistory(t *testing.T) {
	for _, header := range []string{
		fmt.Sprintf(`{"version":1,"next_id":%d,"entries":[]}`, math.MaxInt64),
		fmt.Sprintf(`{"version":1,"entries":[{"ID":%d,"Request":{"Kind":"future"}}]}`, math.MaxInt64),
	} {
		store := ForCollection(t.TempDir(), "/shared")
		if err := os.WriteFile(store.Path(), []byte(header), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Append(model.HistoryEntry{Request: model.NewRequest()}); err == nil {
			t.Fatal("ID overflow was accepted")
		}
		data, _ := os.ReadFile(store.Path())
		if string(data) != header {
			t.Fatal("ID overflow modified history")
		}
	}
}

func TestUnknownEntriesShareRetentionLimits(t *testing.T) {
	t.Run("entry count", func(t *testing.T) {
		store := ForCollection(t.TempDir(), "/shared")
		raws := make([]string, MaxEntries)
		for i := range raws {
			raws[i] = fmt.Sprintf(`{"ID":%d,"Request":{"Kind":"future"}}`, MaxEntries-i)
		}
		if err := os.WriteFile(store.Path(), []byte(`{"version":1,"entries":[`+strings.Join(raws, ",")+`]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Append(model.HistoryEntry{Request: model.NewRequest()}); err != nil {
			t.Fatal(err)
		}
		f, err := store.read()
		if err != nil || len(f.Entries) != MaxEntries || rawID(f.Entries[0]) != 101 || rawID(f.Entries[99]) != 2 {
			t.Fatalf("unknown entries escaped count limit: %d %v", len(f.Entries), err)
		}
	})
	t.Run("combined byte budget", func(t *testing.T) {
		store := ForCollection(t.TempDir(), "/shared")
		raw := `{"ID":1,"Request":{"Kind":"future"},"data":"` + strings.Repeat("x", MaxBytes/2) + `"}`
		if err := os.WriteFile(store.Path(), []byte(`{"version":1,"entries":[`+raw+`]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Append(model.HistoryEntry{Request: model.NewRequest(), Response: &model.Response{Body: make([]byte, MaxBytes/2)}}); err != nil {
			t.Fatal(err)
		}
		f, err := store.read()
		if err != nil || len(f.Entries) != 1 || rawID(f.Entries[0]) != 2 {
			t.Fatalf("unknown entry escaped combined byte limit: %d %v", len(f.Entries), err)
		}
	})
}

func TestStoreLoadsLatestFileWithoutCachedUnknownEntries(t *testing.T) {
	dir := t.TempDir()
	first, second := ForCollection(dir, "/shared"), ForCollection(dir, "/shared")
	raw, _ := json.Marshal(map[string]any{"ID": 1, "Request": map[string]any{"Kind": "future"}})
	if err := os.WriteFile(first.Path(), []byte(`{"version":1,"entries":[`+string(raw)+`]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Append(model.HistoryEntry{Request: model.NewRequest()}); err != nil {
		t.Fatal(err)
	}
	f, err := first.read()
	if err != nil || len(f.Entries) != 1 || rawID(f.Entries[0]) != 2 {
		t.Fatalf("stale load resurrected an unknown entry: %+v %v", f, err)
	}
}

func TestProcessClearAndAppendSerializeWithoutResurrection(t *testing.T) {
	dir := t.TempDir()
	store := ForCollection(dir, "/shared")
	if _, err := store.Append(model.HistoryEntry{Request: model.NewRequest()}); err != nil {
		t.Fatal(err)
	}
	gate := filepath.Join(dir, "go")
	clearing := startHistoryProcess(t, dir, gate, "clear", "", 0)
	appending := startHistoryProcess(t, dir, gate, "append", "new", 12)
	releaseHistoryProcesses(t, gate)
	clearing.wait(t)
	appending.wait(t)
	entries, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for i, entry := range entries {
		if entry.ID != int64(13-i) || entry.Request.Name != fmt.Sprintf("new-%d", 11-i) {
			t.Fatalf("clear retained an old or reordered send: %+v", entries)
		}
	}
	entries, err = store.Append(model.HistoryEntry{Request: model.NewRequest()})
	if err != nil || entries[0].ID != 14 {
		t.Fatalf("concurrent clear reused an ID: %+v %v", entries, err)
	}
}

func TestMalformedHistoryIsNotReplaced(t *testing.T) {
	store := ForCollection(t.TempDir(), "/shared")
	broken := []byte(`{"version":1,"entries":[`)
	if err := os.WriteFile(store.Path(), broken, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func() ([]model.HistoryEntry, error){
		func() ([]model.HistoryEntry, error) {
			return store.Append(model.HistoryEntry{Request: model.NewRequest()})
		},
		func() ([]model.HistoryEntry, error) { return store.Delete([]int64{1}) },
		store.Clear,
	} {
		if _, err := operation(); err == nil {
			t.Fatal("operation ignored malformed history")
		}
		data, err := os.ReadFile(store.Path())
		if err != nil || !bytes.Equal(data, broken) {
			t.Fatalf("operation replaced malformed history: %q %v", data, err)
		}
	}
}

func TestIndependentStoresRetainBothSends(t *testing.T) {
	dir := t.TempDir()
	first, second := ForCollection(dir, "/shared"), ForCollection(dir, "/shared")
	if _, err := first.Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Append(model.HistoryEntry{Request: model.NewRequest()}); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Append(model.HistoryEntry{Request: model.NewRequest()}); err != nil {
		t.Fatal(err)
	}
	entries, err := first.Load()
	if err != nil || len(entries) != 2 {
		t.Fatalf("two independent sends retained %d entries, want 2: %v", len(entries), err)
	}
}

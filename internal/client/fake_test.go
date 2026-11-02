package client

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/model"
)

func TestFakeEchoesResolvedRequest(t *testing.T) {
	req := model.NewRequest()
	req.Method = model.MethodPost
	req.URL = "${BASE}/users/:id?page=2"
	req.PathParams = []model.KeyValue{{Name: "id", Value: "7", Enabled: true}}
	req.Headers = []model.KeyValue{{Name: "X-Token", Value: "${TOKEN}", Enabled: true}}
	req.Body = model.Body{Type: model.BodyRaw, Raw: `{"a": 1}`}

	var stages []model.TraceEvent
	resp, err := Fake{}.Send(context.Background(), Call{
		Request:   req,
		Variables: map[string]string{"BASE": "http://api.test", "TOKEN": "secret"},
		OnTrace:   func(e model.TraceEvent) { stages = append(stages, e) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var echo struct {
		URL     string            `json:"url"`
		Args    map[string]string `json:"args"`
		Headers map[string]string `json:"headers"`
		JSON    map[string]int    `json:"json"`
	}
	if err := json.Unmarshal(resp.Body, &echo); err != nil {
		t.Fatal(err)
	}
	if echo.URL != "http://api.test/users/7?page=2" || echo.Args["page"] != "2" || echo.Headers["X-Token"] != "secret" || echo.JSON["a"] != 1 {
		t.Fatalf("unexpected echo: %+v", echo)
	}
	if len(resp.Trace) != len(model.TraceStages) || len(stages) == 0 {
		t.Fatalf("expected a trace for every stage, got %d events (%d reported)", len(resp.Trace), len(stages))
	}
}

func TestFakeHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := model.NewRequest()
	req.URL = "http://api.test"
	if _, err := (Fake{StageDelay: time.Hour}).Send(ctx, Call{Request: req}); err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
}

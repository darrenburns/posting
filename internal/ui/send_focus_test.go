package ui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/darrenburns/posting/v3/internal/client"
	"github.com/darrenburns/posting/v3/internal/config"
	"github.com/darrenburns/posting/v3/internal/model"
	t "github.com/darrenburns/terma"
)

// responseFocusProbe observes the same focus selection a rendered app sees.
type responseFocusProbe struct {
	app     *App
	focused string
}

func (p *responseFocusProbe) Build(ctx t.BuildContext) t.Widget {
	if w, ok := ctx.Focused().(t.Identifiable); ok {
		p.focused = w.WidgetID()
	}
	return p.app
}

func TestSendCompletionKeepsBackgroundTabSelection(tt *testing.T) {
	for _, tc := range []struct {
		name, onResponse      string
		switchBack, emptyBody bool
	}{
		{name: "background_body", onResponse: "body"},
		{name: "background_tabs", onResponse: "tabs"},
		{name: "background_keep_focus"},
		{name: "switch_back_body", onResponse: "body", switchBack: true},
		{name: "switch_back_tabs", onResponse: "tabs", switchBack: true},
		{name: "switch_back_empty_body", onResponse: "body", switchBack: true, emptyBody: true},
	} {
		tt.Run(tc.name, func(tt *testing.T) {
			app := settingsApp(tt, func(s *config.Settings) { s.Focus.OnResponse = tc.onResponse })
			app.openRequest(sampleRequest(tt, "List users"))
			sending := app.current()
			sending.responseTab.Set("headers")
			// Standalone Terma dispatches inline. Queue callbacks as the real app
			// does, then drain on this goroutine to finish the whole UI update before
			// inspecting state or rendering, without racing the sender goroutine.
			updates := make(chan func(), 1)
			sending.dispatch = func(fn func()) { updates <- fn }
			release := make(chan struct{})
			response := fixedResponse()
			if tc.emptyBody {
				response.Body = nil
			}
			app.sender = client.SenderFunc(func(context.Context, client.Call) (*model.Response, error) {
				<-release
				return response, nil
			})
			app.send()
			app.openRequest(sampleRequest(tt, "Get user"))
			other := app.current()
			other.showResponse(fixedResponse(), nil)
			other.phase.Set(exchangeDone)
			other.responseTab.Set("headers")
			if tc.switchBack {
				app.cycleSession(-1)
			}
			t.RequestFocus(urlInputID)
			close(release)
			select {
			case update := <-updates:
				update()
			case <-time.After(2 * time.Second):
				tt.Fatal("sender did not finish")
			}

			probe := &responseFocusProbe{app: app}
			buf := t.RenderToBuffer(probe, snapW, snapH)
			if tc.name == "background_body" {
				if path := os.Getenv("POSTING_UI_FOCUS_SVG"); path != "" {
					if err := os.WriteFile(path, []byte(t.BufferToSVG(buf, snapW, snapH, t.DefaultSVGOptions())), 0600); err != nil {
						tt.Fatal(err)
					}
				}
			}
			if got := other.responseTab.Peek(); got != "headers" {
				tt.Errorf("background completion changed other request's response tab to %q; want headers", got)
			}
			wantTab, wantFocus := "headers", urlInputID
			if tc.switchBack {
				if tc.onResponse == "body" {
					wantTab, wantFocus = "body", "resp-body"
					if tc.emptyBody {
						wantFocus = responseTabsID
					}
				} else if tc.onResponse == "tabs" {
					wantFocus = responseTabsID
				}
			}
			if got := sending.responseTab.Peek(); got != wantTab {
				tt.Errorf("sending request response tab = %q; want %q", got, wantTab)
			}
			if probe.focused != wantFocus {
				tt.Errorf("focused widget = %q; want %q", probe.focused, wantFocus)
			}
			wantActive := other
			if tc.switchBack {
				wantActive = sending
			}
			if app.current() != wantActive {
				tt.Error("completion changed the active request")
			}
			if sending.response.Peek() != response || sending.phase.Peek() != exchangeDone {
				tt.Error("sending request did not receive its response")
			}
			if got := app.history.Peek(); len(got) != 1 || got[0].Request.File != sending.file.Peek() {
				tt.Errorf("history did not retain the sending request: %+v", got)
			}
		})
	}
}

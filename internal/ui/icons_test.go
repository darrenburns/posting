package ui

import (
	"testing"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/internal/model"
)

func TestSnapshotNerdFonts(tt *testing.T) {
	app := New(Config{
		Version:      "3.0.0-dev",
		Collection:   model.SampleCollection(),
		Environments: StaticEnvironments(model.SampleEnvironments()),
		Environment:  []string{"local.env"},
		UserHost:     "user@host",
		NerdFonts:    true,
	})
	app.openRequest(sampleRequest(tt, "List users"))
	app.current().showResponse(fixedResponse(), nil)
	app.current().phase.Set(exchangeDone)
	app.notify("Saved users/list-users.posting.yaml", toastSuccess)
	t.AssertSnapshot(tt, app, snapW, snapH, "Nerd Font icons on folders, sidebar tabs, the environment, host, send button, timing and toast")
}

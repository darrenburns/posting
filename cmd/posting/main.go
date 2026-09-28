// Command posting is the Posting 3 terminal HTTP client.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/darrenburns/posting/internal/client"
	"github.com/darrenburns/posting/internal/model"
	"github.com/darrenburns/posting/internal/ui"
)

const version = "3.0.0-dev"

func main() {
	err := ui.Run(ui.Config{
		Version: version,
		// Swap in a real HTTP implementation of client.Sender here.
		Sender:       client.Fake{StageDelay: 60 * time.Millisecond},
		Collection:   model.SampleCollection(),
		Environments: model.SampleEnvironments(),
		Theme:        os.Getenv("POSTING_THEME"),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

//go:build unix

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// closeOnSignal closes server when Posting is told to stop by a signal,
// such as the hangup from its terminal closing, which would otherwise end
// the process without removing its socket. The signal is then raised again,
// so Posting stops as it would have without this.
func closeOnSignal(server interface{ Close() error }) (stop func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-signals:
			server.Close()
			signal.Reset(sig)
			_ = syscall.Kill(os.Getpid(), sig.(syscall.Signal))
		case <-done:
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}

//go:build !unix

package main

// closeOnSignal does nothing where Unix signals aren't how programs are
// stopped. A socket left behind is removed by the next `posting remote`.
func closeOnSignal(interface{ Close() error }) (stop func()) { return func() {} }

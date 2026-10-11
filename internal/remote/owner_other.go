//go:build !unix

package remote

import "os"

// ownedByMe can't be checked here. The socket directory is in the user's
// own temporary directory on Windows.
func ownedByMe(os.FileInfo) bool { return true }

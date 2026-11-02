package model

import (
	"mime"
	"path/filepath"
)

// UploadPath resolves a file reference on the machine running Posting.
// The collection root is used even for unsaved requests.
func UploadPath(root, path string) string { return absPath(root, path) }

// FileContentType infers a file's media type without reading its contents.
func FileContentType(path string) string {
	if typ := mime.TypeByExtension(filepath.Ext(path)); typ != "" {
		return typ
	}
	return "application/octet-stream"
}

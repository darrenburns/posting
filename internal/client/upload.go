package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/darrenburns/posting/internal/model"
)

// preparedBody keeps only framing/text in memory, never file contents. Each
// attempt opens independent readers, including redirects and Digest retries.
type preparedBody struct {
	segments    []bodySegment
	length      int64
	contentType string
}

type bodySegment struct {
	text string
	path string
	info os.FileInfo
}

func prepareBody(body model.Body, root string) (*preparedBody, error) {
	b := &preparedBody{}
	text := func(s string) {
		b.segments = append(b.segments, bodySegment{text: s})
		b.length += int64(len(s))
	}
	file := func(path string) error {
		if path == "" {
			return fmt.Errorf("choose a file first")
		}
		path = model.UploadPath(root, path)
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		b.segments = append(b.segments, bodySegment{path: path, info: info})
		b.length += info.Size()
		return nil
	}
	switch body.Type {
	case model.BodyFile:
		if err := file(body.File); err != nil {
			return nil, fmt.Errorf("body file: %w", err)
		}
		b.contentType = body.ContentType
		if b.contentType == "" {
			b.contentType = model.FileContentType(body.File)
		}
	case model.BodyMultipart:
		var framing bytes.Buffer
		writer := multipart.NewWriter(&framing)
		b.contentType = writer.FormDataContentType()
		for i, part := range body.Multipart {
			if !part.Enabled {
				continue
			}
			label := fmt.Sprintf("multipart part %d (%s)", i+1, part.Name)
			if part.Name == "" {
				return nil, fmt.Errorf("%s: enter a name", label)
			}
			for _, value := range []string{part.Name, part.Filename, part.ContentType} {
				if strings.ContainsAny(value, "\r\n\x00") {
					return nil, fmt.Errorf("%s: header fields cannot contain newlines or NUL", label)
				}
			}
			params := map[string]string{"name": part.Name}
			typ := part.ContentType
			switch part.Type {
			case model.PartFile:
				filename := part.Filename
				if filename == "" {
					filename = filepath.Base(part.File)
				}
				if strings.ContainsAny(filename, "\r\n\x00") {
					return nil, fmt.Errorf("%s: invalid filename", label)
				}
				params["filename"] = filename
				if typ == "" {
					typ = model.FileContentType(part.File)
				}
			case "", model.PartText:
				if part.Filename != "" {
					params["filename"] = part.Filename
				}
			default:
				return nil, fmt.Errorf("%s: unsupported type %q", label, part.Type)
			}
			h := textproto.MIMEHeader{"Content-Disposition": {multipartDisposition(params)}}
			if typ != "" {
				if _, _, err := mime.ParseMediaType(typ); err != nil {
					return nil, fmt.Errorf("%s: invalid content type: %w", label, err)
				}
				h.Set("Content-Type", typ)
			}
			if _, err := writer.CreatePart(h); err != nil {
				return nil, err
			}
			text(framing.String())
			framing.Reset()
			if part.Type == model.PartFile {
				if err := file(part.File); err != nil {
					return nil, fmt.Errorf("%s: %w", label, err)
				}
			} else {
				text(part.Value)
			}
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		text(framing.String())
	case "", model.BodyNone, model.BodyRaw, model.BodyForm:
		data, typ := encodeBody(body)
		text(string(data))
		b.contentType = typ
	default:
		return nil, fmt.Errorf("unsupported body type %q", body.Type)
	}
	if body.Type == model.BodyFile {
		if strings.ContainsAny(b.contentType, "\r\n\x00") {
			return nil, fmt.Errorf("invalid file content type")
		}
		if _, _, err := mime.ParseMediaType(b.contentType); err != nil {
			return nil, fmt.Errorf("invalid file content type: %w", err)
		}
	}
	return b, nil
}

// RFC 7578 forbids filename*. Send UTF-8 in a quoted filename, as browsers
// do, rather than mime.FormatMediaType's RFC 2231 extended parameter.
func multipartDisposition(params map[string]string) string {
	quote := func(s string) string { return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"` }
	s := "form-data; name=" + quote(params["name"])
	if filename, ok := params["filename"]; ok {
		s += "; filename=" + quote(filename)
	}
	return s
}

func (b *preparedBody) open(ctx context.Context, progress func(int64, int64)) (io.ReadCloser, error) {
	r := &uploadReader{ctx: ctx, total: b.length, progress: progress}
	var readers []io.Reader
	for _, segment := range b.segments {
		if err := ctx.Err(); err != nil {
			r.Close()
			return nil, err
		}
		if segment.info == nil {
			readers = append(readers, strings.NewReader(segment.text))
			continue
		}
		f, err := os.Open(segment.path)
		if err != nil {
			r.Close()
			return nil, fmt.Errorf("upload %s: %w", segment.path, err)
		}
		r.files = append(r.files, f)
		info, err := f.Stat()
		if err != nil {
			r.Close()
			return nil, err
		}
		if !os.SameFile(segment.info, info) || info.Size() != segment.info.Size() || !info.ModTime().Equal(segment.info.ModTime()) {
			r.Close()
			return nil, fmt.Errorf("upload file %s changed during this send; send again to use the new file", segment.path)
		}
		readers = append(readers, io.NewSectionReader(f, 0, info.Size()))
	}
	r.reader = io.MultiReader(readers...)
	if progress != nil {
		progress(0, b.length)
	}
	return r, nil
}

type uploadReader struct {
	ctx         context.Context
	reader      io.Reader
	files       []*os.File
	once        sync.Once
	progress    func(int64, int64)
	sent, total int64
	last        time.Time
}

func (r *uploadReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	r.sent += int64(n)
	if r.progress != nil && (r.sent == r.total || time.Since(r.last) >= 50*time.Millisecond) {
		r.last = time.Now()
		r.progress(r.sent, r.total)
	}
	return n, err
}

func (r *uploadReader) Close() error {
	r.once.Do(func() {
		for _, f := range r.files {
			f.Close()
		}
	})
	return nil
}

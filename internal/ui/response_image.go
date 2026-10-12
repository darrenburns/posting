package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"

	t "github.com/darrenburns/terma"
	_ "golang.org/x/image/webp"

	"github.com/darrenburns/posting/internal/model"
)

// Bound both the encoded input and decoded pixels before allocating an image.
// Terma takes its own NRGBA copy; the HTTP body's byte limit alone doesn't
// protect against a small compressed image expanding into a large allocation.
const (
	maxPreviewBytes     = 32 << 20
	maxPreviewPixels    = 16 << 20
	maxPreviewDimension = 16384
)

// imagePreview is transient UI state, never part of a response or its history.
// A nil resource and empty error mean decoding is still in progress.
type imagePreview struct {
	source        *t.ImageResource
	width, height int
	format        string
	err           string
}

func isImageResponse(resp *model.Response) bool {
	if resp == nil || len(resp.Body) == 0 || len(resp.Events) > 0 {
		return false
	}
	ct := resp.ContentType()
	if ct == "" || ct == "application/octet-stream" {
		ct = http.DetectContentType(resp.Body)
	}
	return strings.HasPrefix(ct, "image/")
}

func decodeImagePreview(body []byte) *imagePreview {
	p := &imagePreview{}
	if len(body) > maxPreviewBytes {
		p.err = "Image previews are limited to 32 MiB. Save the response to view it externally."
		return p
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		p.err = "Couldn't read this image. Supported formats: PNG, JPEG, GIF (first frame) and static WebP."
		return p
	}
	p.width, p.height, p.format = config.Width, config.Height, format
	if config.Width <= 0 || config.Height <= 0 || config.Width > maxPreviewDimension || config.Height > maxPreviewDimension || config.Width > maxPreviewPixels/config.Height {
		p.err = "Image previews are limited to 16 megapixels and 16,384 pixels per side. Save the response to view it externally."
		return p
	}
	decoded, _, err := image.Decode(bytes.NewReader(body))
	if err == nil {
		// A GIF's first frame may cover only part of its logical screen.
		// Preserve its offset and transparent margins instead of enlarging
		// that subrectangle to fill the entire preview.
		bounds := image.Rect(0, 0, config.Width, config.Height)
		if decoded.Bounds() != bounds {
			canvas := image.NewNRGBA(bounds)
			draw.Draw(canvas, decoded.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
			decoded = canvas
		}
		p.source, err = t.NewImageResource(decoded)
	}
	if err != nil {
		p.err = "Couldn't decode this image: " + err.Error()
	}
	return p
}

// prepareImagePreview decodes once, off the UI goroutine. Identity guards the
// callback when another response arrives or the tab closes during decoding.
func (s *Session) prepareImagePreview(resp *model.Response) {
	s.responseImage.Set(nil)
	s.responseAsText.Set(false)
	if !isImageResponse(resp) {
		return
	}
	pending := &imagePreview{}
	s.responseImage.Set(pending)
	dispatch := s.dispatch
	go func() {
		preview := decodeImagePreview(resp.Body)
		dispatch(func() {
			if s.responseImage.Peek() == pending {
				s.responseImage.Set(preview)
			}
		})
	}()
}

func (s *Session) toggleImagePreview() {
	if s.responseImage.Peek() == nil {
		return
	}
	asText := !s.responseAsText.Peek()
	text := ""
	if asText {
		text = formatBody(s.response.Peek(), s.prettifyJSON)
	}
	s.responseBody.SetText(text)
	s.responseBody.ClearSelection()
	s.responseBody.CursorIndex.Set(0)
	s.responseBodyScroll.SetOffset(0)
	s.responseVisual.Set(false)
	s.responseAsText.Set(asText)
	t.RequestFocus("resp-body")
}

type responseImageView struct {
	fillParent
	app     *App
	session *Session
}

// Keep the body's focus target across loading, preview, error and text views.
func (v responseImageView) WidgetID() string      { return "resp-body" }
func (v responseImageView) IsFocusable() bool     { return true }
func (v responseImageView) OnKey(t.KeyEvent) bool { return false }
func (v responseImageView) OnMouseDown(t.MouseEvent) {
	t.RequestFocus("resp-body")
}

func (v responseImageView) Keybinds() []t.Keybind {
	return []t.Keybind{
		{Key: "f2", Name: "View as text", Action: v.session.toggleImagePreview},
		{Key: "s", Name: "Save image", Action: func() { v.app.openResponseSave(v.session.response.Peek()) }},
		{Key: "up", Name: "Tabs", Action: func() { t.RequestFocus(responseTabsID) }, Hidden: true},
	}
}

func (v responseImageView) Build(ctx t.BuildContext) t.Widget {
	s, theme := v.session, ctx.Theme()
	p := s.responseImage.Get()
	var content t.Widget = emptyState{Title: "Loading image…"}
	if p.err != "" {
		content = emptyState{Title: "Preview unavailable", Lines: []string{escapeMarkup(p.err)}}
	} else if p.source != nil {
		content = t.Image{ID: "resp-image", Source: p.source, Fit: t.ImageContain, Style: t.Style{Width: t.Flex(1), Height: t.Flex(1)}}
	}
	label := s.response.Peek().ContentType()
	if p.format != "" {
		label = fmt.Sprintf("%s · %d × %d", strings.ToUpper(p.format), p.width, p.height)
		if p.format == "gif" {
			label += " · first frame"
		}
	}
	return t.Column{
		Style: t.Style{Width: t.Flex(1), Height: t.Flex(1), BackgroundColor: theme.Background},
		Children: []t.Widget{
			content,
			t.Row{
				Style: t.Style{Width: t.Flex(1), Height: t.Cells(1), BackgroundColor: theme.Surface, Padding: inset}, Spacing: 2,
				Children: []t.Widget{
					t.Text{Content: label, Style: t.Style{Width: t.Flex(1), ForegroundColor: theme.AccentText}},
					t.Text{Content: "text", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { s.toggleImagePreview() }},
					t.Text{Content: "save", Style: t.Style{ForegroundColor: theme.TextMuted}, Click: func(t.MouseEvent) { v.app.openResponseSave(s.response.Peek()) }},
				},
			},
		},
	}
}

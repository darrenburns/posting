package ui

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	t "github.com/darrenburns/terma"

	"github.com/darrenburns/posting/v3/internal/history"
	"github.com/darrenburns/posting/v3/internal/model"
)

// The asymmetric red/blue image catches missing pixels and wrong orientation.
func imageBody(tt *testing.T, format string) []byte {
	tt.Helper()
	if format == "webp" {
		// Lossless 6×4 fixture, red for x < 4 and blue for x >= 4.
		data, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4TBYAAAAvBcAAAA/wGaLTuJ8oc/5DC4WJ6H8A")
		if err != nil {
			tt.Fatal(err)
		}
		return data
	}
	palette := color.Palette{color.NRGBA{R: 238, G: 51, B: 68, A: 255}, color.NRGBA{R: 34, G: 119, B: 221, A: 255}, color.NRGBA{G: 255, A: 255}}
	img := image.NewPaletted(image.Rect(0, 0, 6, 4), palette)
	for y := 0; y < 4; y++ {
		for x := 4; x < 6; x++ {
			img.SetColorIndex(x, y, 1)
		}
	}
	var out bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&out, img)
	case "jpeg":
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 100})
	case "gif":
		next := image.NewPaletted(img.Bounds(), palette)
		for i := range next.Pix {
			next.Pix[i] = 2
		}
		err = gif.EncodeAll(&out, &gif.GIF{Image: []*image.Paletted{img, next}, Delay: []int{1, 1}})
	default:
		tt.Fatalf("unknown fixture format %q", format)
	}
	if err != nil {
		tt.Fatal(err)
	}
	return out.Bytes()
}

func imageResponse(contentType string, body []byte) *model.Response {
	resp := fixedResponse()
	resp.Headers = []model.Header{{Name: "Content-Type", Value: contentType}}
	resp.URL = "https://example.test/images/sample"
	resp.Body = body
	return resp
}

func showImage(tt *testing.T, app *App, resp *model.Response) {
	tt.Helper()
	s := app.current()
	updates := make(chan func(), 1)
	s.dispatch = func(fn func()) { updates <- fn }
	s.showResponse(resp, nil)
	s.phase.Set(exchangeDone)
	select {
	case fn := <-updates:
		fn()
	case <-time.After(2 * time.Second):
		tt.Fatal("image decoding did not complete")
	}
}

func TestImageResponseFormatsAndDetection(tt *testing.T) {
	for _, tc := range []struct{ name, format, contentType string }{
		{"png", "png", "IMAGE/PNG; charset=binary"},
		{"jpeg", "jpeg", "image/jpeg"},
		{"gif_first_frame", "gif", "image/gif"},
		{"webp", "webp", "image/webp"},
		{"missing_type", "png", ""},
		{"generic_type", "png", "application/octet-stream"},
		{"incorrect_image_type", "png", "image/jpeg"},
	} {
		tt.Run(tc.name, func(tt *testing.T) {
			app := testApp()
			showImage(tt, app, imageResponse(tc.contentType, imageBody(tt, tc.format)))
			p := t.NewPilot(tt, app, snapW, snapH)
			text := p.ScreenText()
			if !strings.Contains(text, strings.ToUpper(tc.format)+" · 6 × 4") || strings.Contains(text, "Preview unavailable") {
				tt.Fatalf("missing image metadata:\n%s", text)
			}
			if tc.format == "gif" && !strings.Contains(text, "first frame") {
				tt.Fatal("GIF should be labelled as its first frame")
			}
			if tc.format != "jpeg" {
				colors := map[string]int{}
				for _, cell := range t.SerializeBuffer(p.Buffer(), snapW, snapH).Cells {
					if cell.Content == "▀" || cell.Content == "▄" {
						colors[strings.ToLower(cell.Fg)]++
					}
				}
				if colors["#ee3344"] <= colors["#2277dd"] || colors["#2277dd"] == 0 || colors["#00ff00"] != 0 {
					tt.Fatalf("wrong preview colours/first frame: %v", colors)
				}
			}
			if got := app.current().responseBody.GetText(); got != "" {
				tt.Fatal("image bytes were also loaded into the text viewer")
			}
		})
	}
}

func TestImageResponseGIFFrameOffset(tt *testing.T) {
	// This GIF has a small first frame at the bottom-right of its logical
	// screen. It must render like a full-size transparent PNG, not a crop.
	frame := image.NewPaletted(image.Rect(4, 1, 6, 3), color.Palette{color.NRGBA{R: 238, G: 51, B: 68, A: 255}})
	var gifBody, pngBody bytes.Buffer
	if err := gif.EncodeAll(&gifBody, &gif.GIF{
		Image: []*image.Paletted{frame}, Delay: []int{0},
		Config: image.Config{Width: 6, Height: 4},
	}); err != nil {
		tt.Fatal(err)
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, 6, 4))
	draw.Draw(canvas, image.Rect(4, 1, 6, 3), image.NewUniform(frame.Palette[0]), image.Point{}, draw.Src)
	if err := png.Encode(&pngBody, canvas); err != nil {
		tt.Fatal(err)
	}
	var want string
	for _, tc := range []struct {
		format string
		body   []byte
	}{{"png", pngBody.Bytes()}, {"gif", gifBody.Bytes()}} {
		app := testApp()
		showImage(tt, app, imageResponse("image/"+tc.format, tc.body))
		var pixels strings.Builder
		for _, cell := range t.SerializeBuffer(t.RenderToBuffer(app, snapW, snapH), snapW, snapH).Cells {
			if cell.Content == "▀" || cell.Content == "▄" {
				fmt.Fprintf(&pixels, "%v\n", cell)
			}
		}
		if pixels.Len() == 0 {
			tt.Fatal("no image pixels rendered")
		}
		if tc.format == "png" {
			want = pixels.String()
		} else if pixels.String() != want {
			tt.Fatal("GIF first frame lost its offset or transparent margins")
		}
	}
}

func TestImageResponseExplicitTextType(tt *testing.T) {
	app := testApp()
	app.current().showResponse(imageResponse("text/plain", imageBody(tt, "png")), nil)
	app.current().phase.Set(exchangeDone)
	p := t.NewPilot(tt, app, snapW, snapH)
	if !strings.Contains(p.ScreenText(), "wrap on") || strings.Contains(p.ScreenText(), "Loading image") {
		tt.Fatalf("explicit text response was sniffed as an image:\n%s", p.ScreenText())
	}
}

func TestImageResponseErrorsKeepOriginalBody(tt *testing.T) {
	// A valid PNG IHDR is enough to check dimensions without allocating pixels.
	header := func(w, h uint32) []byte {
		data := append([]byte(nil), imageBody(tt, "png")[:33]...)
		binary.BigEndian.PutUint32(data[16:20], w)
		binary.BigEndian.PutUint32(data[20:24], h)
		// Truecolour config needs no palette chunk after the IHDR.
		data[24], data[25] = 8, 2
		binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
		return data
	}
	for _, tc := range []struct {
		name, contentType, errorText string
		body                         []byte
	}{
		{"not_an_image", "image/png", "Couldn't read", []byte("server returned an error")},
		{"svg", "image/svg+xml", "Supported formats", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
		{"truncated", "image/png", "Couldn't decode", header(6, 4)},
		{"pixel_limit", "image/png", "limited to 16 megapixels", header(4096, 4097)},
		{"at_pixel_limit", "image/png", "Couldn't decode", header(4096, 4096)},
		{"side_limit", "image/png", "limited to 16 megapixels", header(16385, 1)},
		{"byte_limit", "image/png", "limited to 32 MiB", make([]byte, (32<<20)+1)},
	} {
		tt.Run(tc.name, func(tt *testing.T) {
			app := testApp()
			resp := imageResponse(tc.contentType, tc.body)
			showImage(tt, app, resp)
			sc := newScreen(app, snapW, snapH)
			text := sc.renderer.ScreenText()
			for _, want := range []string{"Preview unavailable", tc.errorText, "200 OK", "save"} {
				if !strings.Contains(text, want) {
					tt.Fatalf("missing %q:\n%s", want, text)
				}
			}
			if !bytes.Equal(app.current().response.Peek().Body, tc.body) {
				tt.Fatal("preview failure changed the original body")
			}
		})
	}
}

func TestImageResponseTextSaveAndHistory(tt *testing.T) {
	app := testApp()
	resp := imageResponse("image/png", imageBody(tt, "png"))
	showImage(tt, app, resp)
	p := t.NewPilot(tt, app, snapW, snapH)
	p.Click("resp-body")
	if p.FocusedID() != "resp-body" {
		tt.Fatalf("preview focus = %q", p.FocusedID())
	}
	p.Press("f2")
	if p.FocusedID() != "resp-body" || !strings.Contains(p.ScreenText(), "wrap on") || app.current().responseBody.GetText() == "" {
		tt.Fatalf("F2 didn't open the text viewer:\n%s", p.ScreenText())
	}
	p.Press("ctrl+f")
	p.Type("PNG")
	if p.FocusedID() != responseSearchID || !strings.Contains(p.ScreenText(), "1/1") {
		tt.Fatalf("image bytes couldn't be searched in text mode:\n%s", p.ScreenText())
	}
	p.Press("escape")
	p.Press("f2")
	if p.FocusedID() != "resp-body" || !strings.Contains(p.ScreenText(), "PNG · 6 × 4") || strings.Contains(p.ScreenText(), "wrap on") {
		tt.Fatalf("F2 didn't restore the preview:\n%s", p.ScreenText())
	}
	p.Press("s")
	if p.FocusedID() != "response-save-path" {
		tt.Fatalf("save dialog focus = %q", p.FocusedID())
	}
	// A newer response must not replace the bytes captured by the save dialog.
	app.current().showResponse(fixedResponse(), nil)
	name := filepath.Join(tt.TempDir(), "original.png")
	p.Press("ctrl+a")
	p.Type(name)
	p.Press("enter")
	data, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(data, resp.Body) {
		tt.Fatalf("saved response differs: %v", err)
	}
	app.openResponseSave(resp)
	app.responseSave.path.SetText(name)
	app.submitResponseSave()
	if !strings.Contains(app.responseSave.err.Peek(), "already exists") {
		tt.Fatal("saving over an existing file wasn't rejected")
	}
	app.closeOverlay()
	data, err = os.ReadFile(name)
	if err != nil || !bytes.Equal(data, resp.Body) {
		tt.Fatal("existing file changed")
	}

	// Persist/reload bytes, then preview through the ordinary history route.
	store := history.ForCollection(tt.TempDir(), "/test-collection")
	entry := sentEntry(1, model.NewRequest(), resp, time.Now())
	if err := store.Save([]model.HistoryEntry{entry}); err != nil {
		tt.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil || len(loaded) != 1 {
		tt.Fatalf("load history: %v", err)
	}
	app.openHistory(loaded[0])
	// openHistory created a new session; Terma's pilot owns UI dispatch here.
	p.WaitUntil(func() bool { return strings.Contains(p.ScreenText(), "PNG · 6 × 4") }, 2*time.Second)
}

func TestImageResponseLateDecodeDoesNotReplaceNewResponse(tt *testing.T) {
	app := testApp()
	s := app.current()
	updates := make(chan func(), 1)
	s.dispatch = func(fn func()) { updates <- fn }
	s.showResponse(imageResponse("image/png", imageBody(tt, "png")), nil)
	s.showResponse(fixedResponse(), nil)
	s.phase.Set(exchangeDone)
	select {
	case fn := <-updates:
		fn()
	case <-time.After(2 * time.Second):
		tt.Fatal("image decoding didn't complete")
	}
	sc := newScreen(app, snapW, snapH)
	if !sc.shows("application/json") || sc.shows("PNG ·") || sc.shows("Loading image") {
		tt.Fatalf("old image replaced JSON:\n%s", sc.renderer.ScreenText())
	}
}

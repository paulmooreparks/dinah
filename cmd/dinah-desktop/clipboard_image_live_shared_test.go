//go:build live && cgo && (darwin || linux)

package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// liveClipboardPictureEnv turns on the clipboard picture tests of macOS and
// Linux. They need the live tag as well, and they replace whatever is on
// the clipboard, which is why they ask for both:
//
//	DINAH_LIVE_CLIPBOARD_PICTURE=1 go test -tags live -run TestLiveClipboardPicture -v ./cmd/dinah-desktop
const liveClipboardPictureEnv = "DINAH_LIVE_CLIPBOARD_PICTURE"

func requireLiveClipboardPicture(t *testing.T) {
	t.Helper()
	if os.Getenv(liveClipboardPictureEnv) != "1" {
		t.Skip("set " + liveClipboardPictureEnv + "=1 to let this test replace the clipboard")
	}
}

// samplePNG is a small opaque picture of six plain colours, as PNG.
func samplePNG(t *testing.T) ([]byte, image.Image) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	colours := []color.NRGBA{
		{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255},
		{255, 255, 255, 255}, {0, 0, 0, 255}, {128, 128, 128, 255},
	}
	for i, c := range colours {
		img.SetNRGBA(i%3, i/3, c)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), img
}

// checkClipboardPicture says whether got is a PNG of the same size and
// pixels as want.
func checkClipboardPicture(t *testing.T, got ClipboardImage, want image.Image) {
	t.Helper()
	if got.Type != "image/png" {
		t.Fatalf("type %q with %d bytes, want image/png", got.Type, len(got.Data))
	}
	img, err := png.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("the data read back is not a PNG: %v", err)
	}
	if img.Bounds().Size() != want.Bounds().Size() {
		t.Fatalf("size %v, want %v", img.Bounds().Size(), want.Bounds().Size())
	}
	for y := 0; y < want.Bounds().Dy(); y++ {
		for x := 0; x < want.Bounds().Dx(); x++ {
			g := color.NRGBAModel.Convert(img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y)).(color.NRGBA)
			w := color.NRGBAModel.Convert(want.At(x, y)).(color.NRGBA)
			if g != w {
				t.Errorf("pixel (%d, %d) is %v, want %v", x, y, g, w)
			}
		}
	}
}

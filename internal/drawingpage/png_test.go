package drawingpage

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestPictureBytesMustMatchDeclaredGeometry(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 20, 10))); err != nil {
		t.Fatal(err)
	}
	if err := validatePNG(Picture{PNG: b.Bytes(), Width: 20, Height: 10}); err != nil {
		t.Fatal(err)
	}
	if err := validatePNG(Picture{PNG: b.Bytes(), Width: 21, Height: 10}); err == nil {
		t.Fatal("accepted wrong declared width")
	}
	if err := validatePNG(Picture{PNG: b.Bytes()[:len(b.Bytes())-8], Width: 20, Height: 10}); err == nil {
		t.Fatal("accepted truncated PNG")
	}
}

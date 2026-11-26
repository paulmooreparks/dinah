package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// dibPixels is the 3 by 2 picture each test bitmap holds, top row first.
var dibPixels = [2][3]color.NRGBA{
	{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}},
	{{R: 10, G: 20, B: 30, A: 255}, {R: 255, G: 255, B: 255, A: 255}, {A: 255}},
}

type dibSpec struct {
	headerSize  int
	bitCount    int
	compression uint32
	topDown     bool
	masks       []uint32 // red, green, blue (and alpha) for BI_BITFIELDS
	alpha       [2][3]uint8
	withAlpha   bool
}

// buildDIB writes a packed DIB of dibPixels as a program puts one on the
// clipboard: the header, any masks after a BITMAPINFOHEADER, then the rows,
// each padded to four bytes.
func buildDIB(s dibSpec) []byte {
	le := binary.LittleEndian
	header := make([]byte, s.headerSize)
	le.PutUint32(header[0:], uint32(s.headerSize))
	le.PutUint32(header[4:], 3)
	h := int32(2)
	if s.topDown {
		h = -2
	}
	le.PutUint32(header[8:], uint32(h))
	le.PutUint16(header[12:], 1)
	le.PutUint16(header[14:], uint16(s.bitCount))
	le.PutUint32(header[16:], s.compression)
	var out bytes.Buffer
	if s.headerSize > dibInfoHeaderSize {
		for i, m := range s.masks {
			le.PutUint32(header[40+4*i:], m)
		}
	}
	out.Write(header)
	if s.headerSize == dibInfoHeaderSize && s.compression == dibBitfields {
		for _, m := range s.masks[:3] {
			binary.Write(&out, le, m)
		}
	}
	stride := ((3*s.bitCount + 31) / 32) * 4
	for i := 0; i < 2; i++ {
		y := 1 - i
		if s.topDown {
			y = i
		}
		row := make([]byte, stride)
		for x, c := range dibPixels[y] {
			switch {
			case s.bitCount == 24:
				copy(row[x*3:], []byte{c.B, c.G, c.R})
			case s.compression == dibRGB:
				// The fourth byte is not used; a non-zero value must not become alpha.
				copy(row[x*4:], []byte{c.B, c.G, c.R, 0x7f})
			default:
				// Masks used here: red 0x00ff0000, green 0x0000ff00, blue 0x000000ff, alpha 0xff000000.
				a := uint32(0x33)
				if s.withAlpha {
					a = uint32(s.alpha[y][x])
				}
				le.PutUint32(row[x*4:], a<<24|uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B))
			}
		}
		out.Write(row)
	}
	return out.Bytes()
}

func decodePNG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 3 || b.Dy() != 2 {
		t.Fatalf("size %v, want 3 by 2", b)
	}
	return img
}

func checkPixels(t *testing.T, img image.Image, alpha *[2][3]uint8) {
	t.Helper()
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			want := dibPixels[y][x]
			if alpha != nil {
				want.A = alpha[y][x]
			}
			got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if want.A == 0 {
				// A fully transparent pixel keeps no colour once encoded.
				if got.A != 0 {
					t.Errorf("pixel (%d,%d) = %v, want transparent", x, y, got)
				}
				continue
			}
			if got != want {
				t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, want)
			}
		}
	}
}

var rgbaMasks = []uint32{0x00ff0000, 0x0000ff00, 0x000000ff, 0xff000000}

func TestDIBToPNG(t *testing.T) {
	alpha := [2][3]uint8{{255, 128, 0}, {64, 255, 200}}
	cases := []struct {
		name  string
		spec  dibSpec
		alpha *[2][3]uint8
	}{
		{"24-bit BI_RGB bottom-up", dibSpec{headerSize: 40, bitCount: 24, compression: dibRGB}, nil},
		{"24-bit BI_RGB top-down", dibSpec{headerSize: 40, bitCount: 24, compression: dibRGB, topDown: true}, nil},
		{"32-bit BI_RGB is opaque", dibSpec{headerSize: 40, bitCount: 32, compression: dibRGB}, nil},
		{"32-bit BI_RGB in a V5 header is opaque", dibSpec{headerSize: 124, bitCount: 32, compression: dibRGB}, nil},
		{"32-bit BI_BITFIELDS after an info header is opaque", dibSpec{headerSize: 40, bitCount: 32, compression: dibBitfields, masks: rgbaMasks[:3]}, nil},
		{"32-bit BI_BITFIELDS in a V4 header is opaque", dibSpec{headerSize: 108, bitCount: 32, compression: dibBitfields, masks: rgbaMasks}, nil},
		{"V5 header with an alpha mask", dibSpec{headerSize: 124, bitCount: 32, compression: dibBitfields, masks: rgbaMasks, alpha: alpha, withAlpha: true}, &alpha},
		{"V5 header with alpha, top-down", dibSpec{headerSize: 124, bitCount: 32, compression: dibBitfields, masks: rgbaMasks, alpha: alpha, withAlpha: true, topDown: true}, &alpha},
		{"V5 header with no alpha mask is opaque", dibSpec{headerSize: 124, bitCount: 32, compression: dibBitfields, masks: rgbaMasks[:3]}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := dibToPNG(buildDIB(c.spec))
			if err != nil {
				t.Fatal(err)
			}
			checkPixels(t, decodePNG(t, data), c.alpha)
		})
	}
}

func TestDIBAlphaAllZeroIsOpaque(t *testing.T) {
	// An alpha mask with every alpha byte zero would be an invisible
	// picture, so it is pasted opaque.
	spec := dibSpec{headerSize: 124, bitCount: 32, compression: dibBitfields, masks: rgbaMasks, withAlpha: true}
	data, err := dibToPNG(buildDIB(spec))
	if err != nil {
		t.Fatal(err)
	}
	checkPixels(t, decodePNG(t, data), nil)
}

func TestDIBAlphaOneNonZeroIsKept(t *testing.T) {
	// One pixel with alpha is enough for the alpha to be taken as read,
	// zeros included.
	alpha := [2][3]uint8{{0, 0, 0}, {0, 1, 0}}
	spec := dibSpec{headerSize: 124, bitCount: 32, compression: dibBitfields, masks: rgbaMasks, alpha: alpha, withAlpha: true}
	data, err := dibToPNG(buildDIB(spec))
	if err != nil {
		t.Fatal(err)
	}
	img := decodePNG(t, data)
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			if got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).A; got != alpha[y][x] {
				t.Errorf("pixel (%d,%d) alpha %d, want %d", x, y, got, alpha[y][x])
			}
		}
	}
}

func TestDIBBitfieldsOtherOrder(t *testing.T) {
	// Masks need not be in the usual places: red in the low byte, blue high.
	spec := dibSpec{headerSize: 40, bitCount: 32, compression: dibBitfields, masks: []uint32{0x000000ff, 0x0000ff00, 0x00ff0000}}
	dib := buildDIB(spec)
	// buildDIB wrote blue low and red high; swap them to match the masks.
	px := dib[40+12:]
	for i := 0; i+4 <= len(px); i += 4 {
		px[i], px[i+2] = px[i+2], px[i]
	}
	data, err := dibToPNG(dib)
	if err != nil {
		t.Fatal(err)
	}
	checkPixels(t, decodePNG(t, data), nil)
}

func TestDIBRefused(t *testing.T) {
	le := binary.LittleEndian
	sixteen := buildDIB(dibSpec{headerSize: 40, bitCount: 24, compression: dibRGB})
	le.PutUint16(sixteen[14:], 16)
	eight := buildDIB(dibSpec{headerSize: 40, bitCount: 24, compression: dibRGB})
	le.PutUint16(eight[14:], 8)
	jpeg := buildDIB(dibSpec{headerSize: 40, bitCount: 24, compression: dibRGB})
	le.PutUint32(jpeg[16:], 4) // BI_JPEG
	short := buildDIB(dibSpec{headerSize: 40, bitCount: 24, compression: dibRGB})
	short = short[:len(short)-4]
	oddHeader := buildDIB(dibSpec{headerSize: 40, bitCount: 24, compression: dibRGB})
	le.PutUint32(oddHeader[0:], 64)
	gappy := buildDIB(dibSpec{headerSize: 40, bitCount: 32, compression: dibBitfields, masks: []uint32{0x00f0f000, 0x0000ff00, 0x000000ff}})
	huge := buildDIB(dibSpec{headerSize: 40, bitCount: 24, compression: dibRGB})
	le.PutUint32(huge[4:], 0x7fffffff)
	cases := map[string]struct {
		dib  []byte
		want string
	}{
		"16-bit":            {sixteen, "16-bit"},
		"8-bit palette":     {eight, "8-bit"},
		"JPEG compression":  {jpeg, "compression 4"},
		"truncated pixels":  {short, "shorter"},
		"undocumented size": {oddHeader, "not a documented size"},
		"gappy mask":        {gappy, "one run of bits"},
		"impossible width":  {huge, "shorter"},
		"empty":             {nil, "too short"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := dibToPNG(c.dib)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want one containing %q", err, c.want)
			}
		})
	}
}

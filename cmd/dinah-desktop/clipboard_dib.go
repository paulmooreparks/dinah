package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/bits"
)

// A device-independent bitmap (DIB) as Windows puts one on the clipboard in
// the standard formats CF_DIB and CF_DIBV5 ("Standard Clipboard Formats",
// Microsoft's Win32 documentation): a BITMAPINFO structure, that is a
// header followed by any colour table, and then the pixels. This file only
// converts bytes, so it builds and is tested on every system.
//
// The header is a BITMAPINFOHEADER (40 bytes), a BITMAPV4HEADER (108 bytes)
// or a BITMAPV5HEADER (124 bytes), each starting with the fields of the one
// before ("BITMAPINFOHEADER structure", "BITMAPV5HEADER structure").
const (
	dibInfoHeaderSize = 40
	dibV4HeaderSize   = 108
	dibV5HeaderSize   = 124

	// biCompression values ("BITMAPINFOHEADER structure").
	dibRGB       = 0
	dibBitfields = 3
)

// dibToPNG converts a packed DIB to a PNG file. It supports the forms
// Microsoft documents for clipboard pictures that need no palette:
//
//   - BI_RGB at 24 bits (blue, green, red bytes) and 32 bits (blue, green,
//     red and a byte BITMAPINFOHEADER says is not used, so the picture is
//     opaque);
//   - BI_BITFIELDS at 32 bits, with the red, green and blue masks that
//     follow a BITMAPINFOHEADER as three DWORDs, or inside a V4 or V5
//     header; alpha is read only from a BITMAPV5HEADER's bV5AlphaMask, and
//     otherwise the picture is opaque, as it is when every pixel's alpha
//     is zero;
//   - rows stored bottom-up (positive biHeight) or top-down (negative
//     biHeight).
//
// Anything else, such as palettes, 16 bits, or JPEG and PNG compression, is
// refused with an error saying what the clipboard held.
func dibToPNG(dib []byte) ([]byte, error) {
	img, err := dibImage(dib)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("the picture could not be converted to PNG: %w", err)
	}
	return out.Bytes(), nil
}

func dibImage(dib []byte) (*image.NRGBA, error) {
	if len(dib) < dibInfoHeaderSize {
		return nil, errors.New("the clipboard's picture is too short to be a bitmap")
	}
	le := binary.LittleEndian
	headerSize := int64(le.Uint32(dib[0:]))
	width := int64(int32(le.Uint32(dib[4:])))
	height := int64(int32(le.Uint32(dib[8:])))
	planes := le.Uint16(dib[12:])
	bitCount := int(le.Uint16(dib[14:]))
	compression := le.Uint32(dib[16:])
	clrUsed := int64(le.Uint32(dib[32:]))

	if headerSize != dibInfoHeaderSize && headerSize != dibV4HeaderSize && headerSize != dibV5HeaderSize {
		return nil, fmt.Errorf("the clipboard's picture has a bitmap header of %d bytes, which is not a documented size", headerSize)
	}
	if int64(len(dib)) < headerSize {
		return nil, errors.New("the clipboard's picture is shorter than its bitmap header")
	}
	if planes != 1 {
		return nil, fmt.Errorf("the clipboard's picture has %d planes; a bitmap has one", planes)
	}
	// A negative height is a top-down bitmap ("BITMAPINFOHEADER structure").
	topDown := height < 0
	if topDown {
		height = -height
	}
	if width <= 0 || height == 0 {
		return nil, errors.New("the clipboard's picture has no pixels")
	}

	var redMask, greenMask, blueMask, alphaMask uint32
	tableEnd := headerSize
	switch {
	case compression == dibRGB && (bitCount == 24 || bitCount == 32):
		// The colour table for these depths is optional, biClrUsed entries
		// long, and only an aid to palette devices; it is skipped.
	case compression == dibBitfields && bitCount == 32:
		if headerSize == dibInfoHeaderSize {
			// "If biCompression equals BI_BITFIELDS, the bitmap uses three
			// DWORD color masks" that follow the header.
			if int64(len(dib)) < headerSize+12 {
				return nil, errors.New("the clipboard's picture is missing its colour masks")
			}
			redMask, greenMask, blueMask = le.Uint32(dib[40:]), le.Uint32(dib[44:]), le.Uint32(dib[48:])
			tableEnd += 12
		} else {
			// V4 and V5 headers hold the masks themselves, at the same
			// offsets: red, green, blue, then alpha.
			redMask, greenMask, blueMask = le.Uint32(dib[40:]), le.Uint32(dib[44:]), le.Uint32(dib[48:])
			if headerSize == dibV5HeaderSize {
				alphaMask = le.Uint32(dib[52:])
			}
		}
		for _, m := range []uint32{redMask, greenMask, blueMask, alphaMask} {
			if m != 0 && !contiguous(m) {
				return nil, errors.New("the clipboard's picture has a colour mask that is not one run of bits")
			}
		}
	default:
		return nil, fmt.Errorf("the clipboard's picture is a %d-bit bitmap with compression %d, which cannot be read here; only 24-bit and 32-bit uncompressed bitmaps are supported", bitCount, compression)
	}
	if clrUsed < 0 || clrUsed > 1<<16 {
		return nil, errors.New("the clipboard's picture has a colour table of an impossible length")
	}
	pixels := tableEnd + clrUsed*4

	// Each row is padded to a multiple of four bytes ("BITMAPINFOHEADER
	// structure", Calculating Surface Stride).
	stride := ((width*int64(bitCount) + 31) / 32) * 4
	if stride <= 0 || height > (int64(len(dib))-pixels)/stride {
		return nil, errors.New("the clipboard's picture is shorter than its size says")
	}

	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	bytesPer := int64(bitCount / 8)
	anyAlpha := false
	for y := int64(0); y < height; y++ {
		row := y
		if !topDown {
			row = height - 1 - y
		}
		src := dib[pixels+row*stride:]
		for x := int64(0); x < width; x++ {
			p := src[x*bytesPer:]
			var c color.NRGBA
			if compression == dibRGB {
				c = color.NRGBA{R: p[2], G: p[1], B: p[0], A: 0xff}
			} else {
				v := le.Uint32(p)
				c = color.NRGBA{R: masked(v, redMask), G: masked(v, greenMask), B: masked(v, blueMask), A: 0xff}
				if alphaMask != 0 {
					c.A = masked(v, alphaMask)
					anyAlpha = anyAlpha || c.A != 0
				}
			}
			img.SetNRGBA(int(x), int(y), c)
		}
	}
	// A picture whose alpha is zero in every pixel would paste as nothing
	// at all, which is never what the person copied; some programs write
	// 32-bit bitmaps with an alpha mask but leave the alpha bytes unused.
	// Such a picture is made opaque. This does not depend on any
	// undocumented behaviour: it only refuses to produce an invisible
	// picture. A picture with any non-zero alpha keeps its alpha as read.
	if alphaMask != 0 && !anyAlpha {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 0xff
		}
	}
	return img, nil
}

// contiguous reports whether a mask's set bits are one run.
func contiguous(m uint32) bool {
	m >>= bits.TrailingZeros32(m)
	return m&(m+1) == 0
}

// masked is a pixel's component under a mask, scaled to eight bits.
func masked(v, mask uint32) uint8 {
	if mask == 0 {
		return 0
	}
	shift := bits.TrailingZeros32(mask)
	width := bits.OnesCount32(mask)
	top := uint64(1)<<width - 1
	return uint8((uint64((v&mask)>>shift)*255 + top/2) / top)
}

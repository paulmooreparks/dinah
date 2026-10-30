package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// Anthropic's vision guide says that when a request has more than 20
// images, any image over 2000 px on a side is refused, and images sent in
// earlier turns count toward the 20
// (https://platform.claude.com/docs/en/build-with-claude/vision). A long
// Claude chat passes 20 pictures sooner or later, so Dinah reduces every
// picture whose longer side is over 2000 px before a Claude chat is given it
// (Paul, 28 October 2026), keeping its proportions and its format. Only the
// standard library's documented PNG, JPEG and GIF decoders and encoders and
// golang.org/x/image's documented scalers are used. WebP has a documented
// decoder in golang.org/x/image/webp and no encoder there or in the standard
// library, so a WebP picture over the size is sent as it is, and the agent
// is told (errCannotRewrite).

// claudeLongestSide is the longest side a picture keeps in a Claude chat.
const claudeLongestSide = 2000

// errCannotRewrite is returned for a picture over the size in a format Dinah
// can read but not write.
var errCannotRewrite = errors.New("this format can be read but not written")

// fittedSize is a w by h picture's size with its longer side at most
// longest, its proportions kept, rounded to the nearest pixel.
func fittedSize(w, h, longest int) (int, int) {
	side := max(w, h)
	if side <= longest {
		return w, h
	}
	nw := max(1, (w*longest+side/2)/side)
	nh := max(1, (h*longest+side/2)/side)
	return nw, nh
}

// fitPicture returns a picture with its longer side at most longest, in its
// own format. changed is false when the picture was small enough, or is of
// a type this does not read, and the data is then returned as it was. A
// WebP picture over the size returns errCannotRewrite with its size.
func fitPicture(mime string, data []byte, longest int) (out []byte, changed bool, w, h int, err error) {
	var cfg image.Config
	switch mime {
	case "image/png":
		cfg, err = png.DecodeConfig(bytes.NewReader(data))
	case "image/jpeg":
		cfg, err = jpeg.DecodeConfig(bytes.NewReader(data))
	case "image/gif":
		cfg, err = gif.DecodeConfig(bytes.NewReader(data))
	case "image/webp":
		cfg, err = webp.DecodeConfig(bytes.NewReader(data))
	default:
		return data, false, 0, 0, nil
	}
	if err != nil {
		return data, false, 0, 0, err
	}
	w, h = cfg.Width, cfg.Height
	nw, nh := fittedSize(w, h, longest)
	if nw == w && nh == h {
		return data, false, w, h, nil
	}
	var buf bytes.Buffer
	switch mime {
	case "image/png":
		src, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return data, false, w, h, err
		}
		err = png.Encode(&buf, scaled(src, nw, nh))
		if err != nil {
			return data, false, w, h, err
		}
	case "image/jpeg":
		src, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return data, false, w, h, err
		}
		if err := jpeg.Encode(&buf, scaled(src, nw, nh), &jpeg.Options{Quality: 90}); err != nil {
			return data, false, w, h, err
		}
	case "image/gif":
		if err := fitGIF(&buf, data, w, h, nw, nh); err != nil {
			return data, false, w, h, err
		}
	default:
		return data, false, w, h, fmt.Errorf("%w (%s)", errCannotRewrite, mime)
	}
	return buf.Bytes(), true, w, h, nil
}

// scaled is src drawn at nw by nh with Catmull-Rom resampling.
func scaled(src image.Image, nw, nh int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst
}

// fitGIF writes a GIF, every frame of it, at nw by nh. Each frame keeps its
// own palette, and is scaled by nearest neighbour so that every pixel stays
// one of the palette's colours, the transparent one included; its place in
// the picture is scaled with it.
func fitGIF(buf *bytes.Buffer, data []byte, w, h, nw, nh int) error {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return err
	}
	whole := image.Rect(0, 0, nw, nh)
	at := func(x, size, nsize int) int { return (x*nsize + size/2) / size }
	for i, frame := range g.Image {
		r := frame.Bounds()
		nr := image.Rect(at(r.Min.X, w, nw), at(r.Min.Y, h, nh), at(r.Max.X, w, nw), at(r.Max.Y, h, nh)).Intersect(whole)
		if nr.Dx() < 1 || nr.Dy() < 1 {
			nr = image.Rect(nr.Min.X, nr.Min.Y, min(nr.Min.X+1, nw), min(nr.Min.Y+1, nh))
			if nr.Empty() {
				nr = image.Rect(nw-1, nh-1, nw, nh)
			}
		}
		dst := image.NewPaletted(nr, frame.Palette)
		xdraw.NearestNeighbor.Scale(dst, nr, frame, r, xdraw.Src, nil)
		g.Image[i] = dst
	}
	g.Config.Width, g.Config.Height = nw, nh
	return gif.EncodeAll(buf, g)
}

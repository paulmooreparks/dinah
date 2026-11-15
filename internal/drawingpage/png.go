package drawingpage

import (
	"bytes"
	"errors"
	"image/png"
)

func validatePNG(p Picture) error {
	if len(p.PNG) > 16<<20 {
		return errors.New("PNG exceeds 16 MiB")
	}
	config, err := png.DecodeConfig(bytes.NewReader(p.PNG))
	if err != nil {
		return err
	}
	if config.Width != p.Width || config.Height != p.Height || p.Width < 1 || p.Height < 1 || p.Width > 8192 || p.Height > 8192 || int64(p.Width)*int64(p.Height) > 16777216 {
		return errors.New("PNG dimensions exceed limits or disagree")
	}
	_, err = png.Decode(bytes.NewReader(p.PNG))
	return err
}

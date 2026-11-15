//go:build !windows

package drawingpage

import (
	"context"
	"errors"
	"io/fs"
)

func Draw(context.Context, fs.FS, Request, Options) (Picture, error) {
	return Picture{}, errors.New("the disposable native drawing page is unavailable on this platform")
}

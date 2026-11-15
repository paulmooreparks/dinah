//go:build !windows

package drawingpage

import (
	"context"
	"io/fs"
)

func Draw(context.Context, fs.FS, Request, Options) (Picture, error) {
	return Picture{}, ErrUnavailable
}

//go:build tui

package main

import (
	"strings"
	"sync"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

// TestNoFrameReachesTheLastColumn is the second clause of
// dinah-603/criteria/44 and holds the first condition of section 16.4 of the
// specification. For every frame the sweep of everyModeRun draws at widths
// 60, 99, 100 and 160, the frame drawn into a cell buffer the way Bubble
// Tea's renderer draws it has an unstyled blank in the last column of every
// row, so the renderer never puts a cell in the lower-right corner and never
// writes autowrap.
func TestNoFrameReachesTheLastColumn(t *testing.T) {
	for _, width := range []int{60, 99, 100, 160} {
		var mu sync.Mutex
		var frames []string
		run := everyModeRun(t, tuiBench(t), width, 30, nil, func(content string) {
			mu.Lock()
			frames = append(frames, content)
			mu.Unlock()
		})
		if run.model == nil {
			t.Fatalf("the run at width %d never finished: %q", width, run.errw)
		}
		mu.Lock()
		drawn := append([]string(nil), frames...)
		mu.Unlock()
		checked := 0
		for i, content := range drawn {
			// Each frame is laid out for the window the model had, which the
			// resize makes narrower for a while, so it is drawn at both
			// widths and checked at whichever its rows fit.
			for _, frameWidth := range []int{width, width - 10} {
				if !fitsWidth(content, frameWidth) {
					continue
				}
				buffer := uv.NewScreenBuffer(frameWidth, 30)
				uv.NewStyledString(content).Draw(buffer, uv.Rect(0, 0, frameWidth, 30))
				for y := 0; y < 30; y++ {
					cell := buffer.CellAt(frameWidth-1, y)
					if cell == nil {
						continue
					}
					if cell.Content != " " || !cell.Style.IsZero() || !cell.Link.IsZero() {
						t.Errorf("at width %d, frame %d, row %d's last column holds %q styled %+v", frameWidth, i, y+1, cell.Content, cell.Style)
					}
				}
				checked++
				break
			}
		}
		if checked != len(drawn) {
			t.Errorf("at width %d, %d of %d frames fit neither the window nor the smaller one", width, len(drawn)-checked, len(drawn))
		}
		t.Logf("width %d: %d frames checked", width, checked)
	}
}

// fitsWidth reports whether every row of a frame, its styling set aside, is
// at most width columns wide by Dinah's own measure.
func fitsWidth(content string, width int) bool {
	for _, row := range strings.Split(content, "\n") {
		if displayWidth(visible(row)) > width {
			return false
		}
	}
	return true
}

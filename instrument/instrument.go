// Package instrument exposes a tiny counter for diagnosing per-frame
// ebiten.NewImage churn — a known cause of the
// "IDXGISwapChain::Present failed: HANDLE(0x887A0005)" / DEVICE_REMOVED
// crashes on Windows. Hot draw paths use NewImage instead of
// ebiten.NewImage so we can read and reset the counter from the runner
// loop and log it alongside heap stats.
package instrument

import (
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
)

var imageAllocs atomic.Int64

// NewImage is a drop-in replacement for ebiten.NewImage that also
// increments the per-frame allocation counter. Use it in any code path
// that runs every frame; one-time / per-zoom allocations can stay on
// ebiten.NewImage.
func NewImage(w, h int) *ebiten.Image {
	imageAllocs.Add(1)
	return ebiten.NewImage(w, h)
}

// SwapImageAllocs returns the number of NewImage calls since the last
// call and resets the counter. Safe to call concurrently with NewImage.
func SwapImageAllocs() int64 {
	return imageAllocs.Swap(0)
}

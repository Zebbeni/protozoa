// Package instrument exposes a tiny counter for diagnosing per-frame ebiten.NewImage churn.
package instrument

import (
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
)

var imageAllocs atomic.Int64

// NewImage is a drop-in replacement for ebiten.NewImage that also increments the per-frame allocation counter.
func NewImage(w, h int) *ebiten.Image {
	imageAllocs.Add(1)
	return ebiten.NewImage(w, h)
}

// SwapImageAllocs returns the number of NewImage calls since the last call and resets the counter.
func SwapImageAllocs() int64 {
	return imageAllocs.Swap(0)
}

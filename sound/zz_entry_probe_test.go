package sound

import (
	"io"
	"math"
	"os"
	"testing"
	"time"

	r "github.com/Zebbeni/protozoa/resources"
)

// ENTRY_PROBE=1 go test ./sound/ -run TestEntryFadeProbe -v
//
// How loud each piece's own opening is, and what the scene's opening fade
// leaves of it. A piece that starts at the level it keeps needs a fade in; one
// that opens quietly already has it, and fading that twice buries it.
//
// This is where the figures in sceneFadeIn's comment came from, so it is the
// measurement to repeat when a piece is replaced. Asserts nothing: how fast an
// entrance should arrive is a judgement.
func TestEntryFadeProbe(t *testing.T) {
	if os.Getenv("ENTRY_PROBE") == "" {
		t.Skip("set ENTRY_PROBE=1 to measure each piece's opening against its fade")
	}
	r.UseDirAssets("..")
	for _, scene := range AllScenes {
		fade := sceneFadeIn[scene]
		for i, paths := range sceneTracks[scene] {
			for _, path := range paths {
				data, ok := r.ReadAsset(path)
				if !ok {
					continue
				}
				s, err := decode(path, data)
				if err != nil {
					t.Errorf("%s: %v", path, err)
					break
				}
				pcm, err := io.ReadAll(s)
				if err != nil {
					t.Fatalf("%s: %v", path, err)
				}
				full, _ := levels(pcm)
				t.Logf("scene %d piece %d %s: %.1fs, whole piece %.1f dBFS, opening fade %v",
					scene, i, path, float64(len(pcm))/float64(bytesPerSecond), full, fade)

				const window = 500 * time.Millisecond
				step := int(window) * bytesPerSecond / int(time.Second)
				step -= step % 4
				for w := 0; w < 16; w++ {
					lo := w * step
					if lo+step > len(pcm) {
						break
					}
					at := time.Duration(w) * window
					raw, _ := levels(pcm[lo : lo+step])
					// The fade at the middle of the window, which is where
					// the window's own level sits.
					faded := raw + gainDB(entryVolume(at+window/2, fade))
					t.Logf("    %5v  raw %6.1f dBFS   faded %6.1f dBFS  (%+5.1f dB against the whole piece)",
						at, raw, faded, faded-full)
				}
				break
			}
		}
	}
}

// gainDB is a volume multiplier as a level change, so a fade can be read in
// the same units as the audio it is applied to.
func gainDB(gain float64) float64 {
	if gain <= 0 {
		return math.Inf(-1)
	}
	return 20 * math.Log10(gain)
}

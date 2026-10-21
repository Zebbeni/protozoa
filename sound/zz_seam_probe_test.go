package sound

import (
	"io"
	"math"
	"os"
	"testing"

	r "github.com/Zebbeni/protozoa/resources"
)

// SEAM=1 go test ./sound/ -run TestLoopSeamProbe -v
//
// Where a piece meets itself. NewInfiniteLoop repeats the whole stream with
// no crossfade, so a piece that fades out at the end and starts at full level
// has an audible dip and re-entry on every pass — and one measured earlier
// had 9 peak amplitude in its last half second against 9,837 in its first,
// while being named "loop". A file's name is not evidence.
//
// Asserts nothing: how close the two ends have to be is a judgement.
func TestLoopSeamProbe(t *testing.T) {
	if os.Getenv("SEAM") == "" {
		t.Skip("set SEAM=1 to measure where each piece meets itself")
	}
	r.UseDirAssets("..")
	for _, scene := range AllScenes {
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
				window := bytesPerSecond // one second at each end
				if len(pcm) < 2*window {
					window = len(pcm) / 2
				}
				head, headPeak := levels(pcm[:window])
				tail, tailPeak := levels(pcm[len(pcm)-window:])
				// A finer look at the join: a piece that OPENS quietly and a
				// piece that fades IN are different things, and only the
				// second is a fault in a loop.
				for _, frac := range []float64{0.125, 0.25, 0.5, 1, 2} {
					w := int(float64(bytesPerSecond) * frac)
					if w*2 > len(pcm) {
						break
					}
					in, _ := levels(pcm[:w])
					out, _ := levels(pcm[len(pcm)-w:])
					t.Logf("    %.3fs window: in %.1f dBFS, out %.1f dBFS, step %+.1f dB",
						frac, in, out, in-out)
				}
				t.Logf("scene %d piece %d %s: %.1fs", scene, i, path,
					float64(len(pcm))/float64(bytesPerSecond))
				t.Logf("    first second: %.1f dBFS rms, peak %.1f dBFS", head, headPeak)
				t.Logf("    last second:  %.1f dBFS rms, peak %.1f dBFS", tail, tailPeak)
				t.Logf("    seam step:    %+.1f dB", head-tail)
				break
			}
		}
	}
}

// levels is one window's RMS and peak in dBFS, read as 16-bit samples.
func levels(pcm []byte) (rms, peak float64) {
	var sum float64
	var top float64
	n := len(pcm) / 2
	for i := 0; i+1 < len(pcm); i += 2 {
		v := float64(int16(uint16(pcm[i]) | uint16(pcm[i+1])<<8))
		sum += v * v
		if a := math.Abs(v); a > top {
			top = a
		}
	}
	if n == 0 {
		return math.Inf(-1), math.Inf(-1)
	}
	return dbfs(math.Sqrt(sum / float64(n))), dbfs(top)
}

func dbfs(v float64) float64 {
	if v <= 0 {
		return math.Inf(-1)
	}
	return 20 * math.Log10(v/32768)
}

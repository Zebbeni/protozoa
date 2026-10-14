package sound

import (
	"testing"

	r "github.com/Zebbeni/protozoa/resources"
)

// TestTheTrackIsThereAndDecodes: the music is an embedded asset, so a build
// that lost it is silent with nothing to say so at compile time. This reads
// it the way the game does and decodes it.
func TestTheTrackIsThereAndDecodes(t *testing.T) {
	r.UseDirAssets("..")
	var found string
	for _, path := range trackPaths {
		data, ok := r.ReadAsset(path)
		if !ok {
			continue
		}
		s, err := decode(path, data)
		if err != nil {
			t.Fatalf("%s is present and does not decode: %v", path, err)
		}
		if s.Length() <= 0 {
			t.Errorf("%s decodes to %d bytes of audio", path, s.Length())
		}
		// Long enough to be a theme rather than a click: the loop repeats
		// the whole stream, so a stub would be audible as a stutter.
		seconds := float64(s.Length()) / float64(sampleRate*4)
		if seconds < 5 {
			t.Errorf("%s is %.1fs long", path, seconds)
		}
		found = path
		break
	}
	if found == "" {
		t.Fatalf("no track found at any of %v", trackPaths)
	}
	t.Logf("playing %s", found)
}

// TestOggIsPreferredOverWav: an .ogg of the same music is far smaller, which
// is most of the browser build's download, so the loader has to pick it up
// without a code change when one is dropped in.
func TestOggIsPreferredOverWav(t *testing.T) {
	if len(trackPaths) < 2 {
		t.Fatalf("only %d track paths", len(trackPaths))
	}
	if got := trackPaths[0]; got[len(got)-4:] != ".ogg" {
		t.Errorf("the first track tried is %q, want the .ogg", got)
	}
}

// TestSilenceWithoutATrack: Init must not take the game down when the track
// is missing, and Enabled has to report the truth rather than the setting.
func TestSilenceWithoutATrack(t *testing.T) {
	enabled, saved := false, player
	player = nil
	defer func() { player = saved }()

	SetEnabled(true)
	if Enabled() {
		t.Error("music reports as on with no player")
	}
	SetEnabled(enabled)
}

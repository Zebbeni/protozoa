// Package sound plays the looping music theme.
//
// One track, one player, started once and left running: pausing and resuming
// an ebiten player keeps its position, so switching the music off and on
// again picks the loop up where it was rather than restarting it.
package sound

import (
	"bytes"
	"log"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	r "github.com/Zebbeni/protozoa/resources"
)

// sampleRate is the context's rate. Everything decoded is resampled to it,
// so a track at any rate plays correctly; this is just the mixer's rate.
const sampleRate = 44100

// trackPaths are tried in order, so dropping an .ogg in beside the .wav
// replaces the track without a code change. Ogg is the one worth having:
// the same music is an order of magnitude smaller, which matters most for
// the browser build, where the track ships inside the wasm binary.
var trackPaths = []string{
	"resources/audio/theme.ogg",
	"resources/audio/theme.wav",
}

var (
	context *audio.Context
	player  *audio.Player
	// enabled is kept even when there is no player, so a build with no track
	// still answers the toggle honestly rather than reporting music on.
	enabled bool
)

// Init decodes the track and starts it if music is on. Safe to call when the
// track is missing: it logs and leaves the game silent.
func Init(on bool) {
	enabled = on
	if context == nil {
		context = audio.NewContext(sampleRate)
	}
	if player == nil {
		player = loadLoop()
	}
	apply()
}

// loadLoop builds the looping player, or nil when there is no track to play.
func loadLoop() *audio.Player {
	for _, path := range trackPaths {
		data, ok := r.ReadAsset(path)
		if !ok {
			continue
		}
		stream, err := decode(path, data)
		if err != nil {
			log.Printf("sound: %s could not be decoded: %v", path, err)
			continue
		}
		// NewInfiniteLoop repeats the whole stream, so the track has to be
		// authored to meet itself at the ends.
		p, err := context.NewPlayer(audio.NewInfiniteLoop(stream, stream.Length()))
		if err != nil {
			log.Printf("sound: %s could not be played: %v", path, err)
			continue
		}
		return p
	}
	log.Printf("sound: no music track found; the game will be silent")
	return nil
}

// stream is the part of a decoded track the looper needs.
type stream interface {
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
	Length() int64
}

func decode(path string, data []byte) (stream, error) {
	if len(path) > 4 && path[len(path)-4:] == ".ogg" {
		return vorbis.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
	}
	return wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
}

// Enabled reports whether music is on, which is false whenever there is no
// track to play.
func Enabled() bool { return enabled && player != nil }

// SetEnabled starts or pauses the loop. Pausing holds the position, so this
// is a mute rather than a stop.
func SetEnabled(on bool) {
	enabled = on
	apply()
}

func Toggle() { SetEnabled(!enabled) }

func apply() {
	if player == nil {
		return
	}
	if enabled {
		if !player.IsPlaying() {
			player.Play()
		}
		return
	}
	if player.IsPlaying() {
		player.Pause()
	}
}

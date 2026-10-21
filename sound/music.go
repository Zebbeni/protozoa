// Package sound plays the music, one SET of pieces per scene.
//
// A scene is a group of screens that share music, not a single screen: the
// menus, the designer and a running simulation's log are one set, and the
// replay viewer is another. Each scene keeps its own players, built the first
// time it is entered and paused rather than stopped when it is left, because
// pausing an ebiten player holds its position — so returning to a scene picks
// up where it was rather than restarting, which is the same reason the on/off
// toggle is a mute.
//
// A scene naming ONE piece loops it forever at full volume, which is what a
// piece authored to meet itself at the ends wants. A scene naming SEVERAL
// plays a couple of loops of each in turn, fading out at the end of its turn
// and the next one in — so a long sitting in the replay viewer hears the two
// pieces alternate rather than one of them over and over.
//
// A scene can also fade in the first time its music plays (sceneFadeIn). An
// entrance is not a handover: at a handover the listener is already inside
// the music, where the replay viewer's first piece arrives over the menu's.
package sound

import (
	"bytes"
	"log"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"

	r "github.com/Zebbeni/protozoa/resources"
)

// sampleRate is the context's rate. Everything decoded is resampled to it,
// so a track at any rate plays correctly; this is just the mixer's rate.
const sampleRate = 44100

// bytesPerSecond is what a decoded stream's length is measured in: ebiten's
// decoders hand back 16-bit stereo at the context's rate, so four bytes a
// frame. It is how a piece's own length — and so its turn — is known.
const bytesPerSecond = sampleRate * 4

// musicLevel is the ceiling every piece plays at, and so what the fades mean
// by full volume. The music sits under a simulation rather than in front of
// one, so it is held below what the recordings were mastered to.
//
// It is an AMPLITUDE, so 0.5 is -6 dB rather than half the loudness a
// listener hears: perceived loudness halves nearer -10 dB, which is about
// 0.32. Lower it to that if "half" should mean half by ear.
const musicLevel = 0.5

const (
	// pieceLoops is how many times a piece repeats before its turn ends.
	pieceLoops = 2
	// pieceFade is how long the handover takes at each end: the outgoing
	// piece fades down over this and the incoming one fades up over it.
	pieceFade = 4 * time.Second
)

// Scene is which music is wanted. The runner maps its own states onto these,
// so a screen does not choose a track directly.
type Scene int

const (
	// SceneSilent is anything with no music of its own.
	SceneSilent Scene = iota
	// SceneMenu covers the splash, the menus, the designer and a running
	// simulation's log.
	SceneMenu
	// SceneReplay is the replay viewer.
	SceneReplay
)

// AllScenes is the list a test walks, so a scene added without a track table
// fails rather than playing silence nothing explains.
var AllScenes = []Scene{SceneSilent, SceneMenu, SceneReplay}

// sceneTracks is each scene's pieces, in the order they take their turns.
// Each piece names the files to try for it, so dropping a different format in
// beside one replaces it without a code change.
//
// A scene with no file on disk is silent, which is how a piece that has not
// been written yet behaves — the game runs quietly through it rather than
// refusing to start or borrowing another scene's music.
var sceneTracks = map[Scene][][]string{
	SceneMenu: {
		{"resources/audio/menu.ogg", "resources/audio/menu.wav"},
	},
	SceneReplay: {
		{"resources/audio/replay_a.ogg", "resources/audio/replay_a.wav"},
		{"resources/audio/replay_b.ogg", "resources/audio/replay_b.wav"},
	},
}

// sceneFadeIn is how long a scene takes to come up when it is entered, 0
// being no fade at all. It runs on EVERY entry — the first launch and every
// switch back — so no screen change drops the music in at full volume.
//
// The two lengths differ because the recordings do. Measured over their first
// half second, replay_a opens at -16.5 dBFS and replay_b at -17.0 against
// their own full-piece levels of -17.2 and -17.6, so each starts at the
// loudness it keeps and needs the whole fade. The menu's piece opens at -30.0
// against -23.8 and stays under its own average for about three seconds, so
// it is given half as long and the recording does the rest.
var sceneFadeIn = map[Scene]time.Duration{
	SceneMenu:   4 * time.Second,
	SceneReplay: 8 * time.Second,
}

// sceneMusic is one scene's set of pieces and which of them is taking its
// turn.
type sceneMusic struct {
	players []*audio.Player
	// turns is how long each piece plays for, including its fade out. A
	// single-piece scene has no turn: it loops at full volume forever.
	turns []time.Duration
	idx   int
	// played is how long the current piece has been audible, advanced by
	// Update rather than read off a clock — the scene is not always the one
	// on screen, and music can be switched off part way through a turn.
	played time.Duration
	// fadeIn is this scene's opening fade and entry is how far into it we
	// are. SetScene puts entry back to 0 on the way in, so a scene returned
	// to comes up from silence the way one entered for the first time does;
	// between entries it counts up to fadeIn and stops there.
	fadeIn time.Duration
	entry  time.Duration
}

var (
	context *audio.Context
	// scenes holds one entry per scene entered. A scene present with a nil
	// value is one whose files are all missing, which is remembered so the
	// miss is logged once rather than on every frame.
	scenes map[Scene]*sceneMusic
	scene  Scene
	// enabled is kept even when there is no player, so a build with no track
	// still answers the toggle honestly rather than reporting music on.
	enabled bool
)

// Init builds the audio context and records whether music is on. Nothing is
// decoded here: the scene decides which pieces to load, and Init runs before
// the game knows which screen it is opening on.
func Init(on bool) {
	enabled = on
	if context == nil {
		context = audio.NewContext(sampleRate)
	}
	if scenes == nil {
		scenes = map[Scene]*sceneMusic{}
	}
	apply()
}

// SetScene switches to a scene's music, pausing whatever was playing. Safe to
// call every frame: a scene that is already current is left alone, so the
// music is not restarted from the top by being asked for again.
func SetScene(s Scene) {
	if s == scene {
		return
	}
	if p := scenes[scene].current(); p != nil && p.IsPlaying() {
		p.Pause()
	}
	scene = s
	// Every entry fades in, not only the first. A scene not built yet starts
	// at 0 of its own accord, so this is only for one being returned to.
	if m := scenes[s]; m != nil {
		m.entry = 0
	}
	apply()
}

// Update advances the current scene's turn by the time since the last call,
// which is what fades a piece out at the end of its turn and brings the next
// one in. Called once a frame; a scene of one piece has nothing to do.
func Update(dt time.Duration) {
	if !enabled || context == nil {
		return
	}
	m := scenes[scene]
	p := m.current()
	if p == nil {
		return
	}
	// Stops counting at its own length, so the fade is an opening rather
	// than a number that grows for as long as the scene is on screen.
	if m.entry < m.fadeIn {
		m.entry += dt
	}
	if len(m.players) > 1 {
		m.played += dt
		if m.played >= m.turns[m.idx] {
			// The turn is over: stop this piece where its fade left it and
			// put it back to the top, so its next turn starts as written.
			p.Pause()
			if err := p.Rewind(); err != nil {
				log.Printf("sound: could not rewind piece %d: %v", m.idx, err)
			}
			m.idx = (m.idx + 1) % len(m.players)
			m.played = 0
			if next := m.current(); next != nil {
				next.SetVolume(m.volume())
				next.Play()
			}
			return
		}
	}
	p.SetVolume(m.volume())
}

// volume is how loud the piece taking its turn should be: the quieter of the
// turn's own fade and the scene's opening fade, so an entrance still coming
// up is not overridden by a turn that has already reached full volume.
// The fades run 0 to 1 and musicLevel is what their 1 means, applied here
// because every player's volume is set from this one function.
func (m *sceneMusic) volume() float64 {
	v := 1.0
	if len(m.players) > 1 {
		v = fadeVolume(m.played, m.turns[m.idx])
	}
	return musicLevel * min(v, entryVolume(m.entry, m.fadeIn))
}

// entryVolume is a scene's opening fade: the amplitude rises as the SQUARE of
// how far through it we are, where the handover fade is linear.
//
// Linear is front-loaded in dB and an entrance is where that shows. The 4s
// handover curve is within 9 dB of full after one second and within 2.5 dB
// after three, so most of it is over before it sounds like it has begun —
// which is how an 8s linear fade in would still arrive suddenly. Squaring
// spends the fade where it is audible: a quarter of the way through is -24 dB
// rather than -12.
func entryVolume(elapsed, fade time.Duration) float64 {
	if fade <= 0 || elapsed >= fade {
		return 1
	}
	if elapsed <= 0 {
		return 0
	}
	p := float64(elapsed) / float64(fade)
	return p * p
}

// fadeVolume is the HANDOVER fade — a piece's volume at one point in its
// turn, linear in amplitude: up over the first
// pieceFade, full in the middle, and down over the last pieceFade. The fade
// down happens AFTER the piece has had its loops, so what is faded is the
// start of a repeat nobody was going to hear out.
//
// A turn too short to hold two fades gives each half of it, so a very short
// piece fades rather than jumping.
func fadeVolume(played, turn time.Duration) float64 {
	fade := pieceFade
	if half := turn / 2; fade > half {
		fade = half
	}
	if fade <= 0 {
		return 1
	}
	switch {
	case played < fade:
		return float64(played) / float64(fade)
	case played > turn-fade:
		return float64(turn-played) / float64(fade)
	}
	return 1
}

// CurrentScene is what is playing, for tests and logging.
func CurrentScene() Scene { return scene }

// musicFor builds a scene's players on first use and remembers the result,
// including a miss.
func musicFor(s Scene) *sceneMusic {
	if m, loaded := scenes[s]; loaded {
		return m
	}
	m := loadScene(sceneTracks[s])
	if m != nil {
		m.fadeIn = sceneFadeIn[s]
	}
	scenes[s] = m
	return m
}

// current is the piece taking its turn, nil-safe on the scene so a silent
// scene and one never entered answer the same way.
func (m *sceneMusic) current() *audio.Player {
	if m == nil || m.idx >= len(m.players) {
		return nil
	}
	return m.players[m.idx]
}

// loadScene builds one scene's pieces, or nil when none of them will play.
func loadScene(pieces [][]string) *sceneMusic {
	m := &sceneMusic{}
	for _, paths := range pieces {
		p, length := loadLoop(paths)
		if p == nil {
			continue
		}
		m.players = append(m.players, p)
		m.turns = append(m.turns, turnFor(length))
	}
	if len(m.players) == 0 {
		return nil
	}
	return m
}

// turnFor is how long a piece plays before the next one takes over: the loops
// it was asked for, and then the fade that hands over. Named rather than
// written where it is used, so what decides a turn can be measured without an
// audio device to build players on.
func turnFor(length time.Duration) time.Duration { return length*pieceLoops + pieceFade }

// loadLoop builds the looping player for one piece and reports how long one
// time through it is, or nil when there is no file to play.
func loadLoop(paths []string) (*audio.Player, time.Duration) {
	if context == nil {
		return nil, 0 // Init has not run; there is nothing to build a player on
	}
	for _, path := range paths {
		data, ok := r.ReadAsset(path)
		if !ok {
			continue
		}
		s, err := decode(path, data)
		if err != nil {
			log.Printf("sound: %s could not be decoded: %v", path, err)
			continue
		}
		// NewInfiniteLoop repeats the whole stream, so a piece has to be
		// authored to meet itself at the ends.
		p, err := context.NewPlayer(audio.NewInfiniteLoop(s, s.Length()))
		if err != nil {
			log.Printf("sound: %s could not be played: %v", path, err)
			continue
		}
		return p, time.Duration(s.Length()) * time.Second / bytesPerSecond
	}
	if len(paths) > 0 {
		log.Printf("sound: no track found at any of %v; this part of the game will be silent", paths)
	}
	return nil, 0
}

// stream is the part of a decoded track the looper needs.
type stream interface {
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
	Length() int64
}

func decode(path string, data []byte) (stream, error) {
	if strings.HasSuffix(path, ".ogg") {
		return vorbis.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
	}
	return wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
}

// Enabled reports whether music is audible now, which is false both when the
// user has switched it off and when the scene being shown has no track.
func Enabled() bool { return enabled && scenes[scene] != nil }

// Preferred is the user's choice about music for this session, whether or not
// there is a track to act on it. It is what a new simulation inherits, so
// switching the music off and then starting a run leaves it off: music is a
// choice about the machine looking at the simulation, like the theme and the
// window size, rather than a property of the simulation being run.
func Preferred() bool { return enabled }

// SetEnabled starts or pauses the current scene's music. Pausing holds the
// position, so this is a mute rather than a stop.
func SetEnabled(on bool) {
	enabled = on
	apply()
}

func Toggle() { SetEnabled(!enabled) }

func apply() {
	if context == nil {
		return
	}
	if !enabled {
		// Only the current scene's piece can be running; the sweep is what
		// makes that true rather than assumed.
		for _, m := range scenes {
			if m == nil {
				continue
			}
			for _, p := range m.players {
				if p != nil && p.IsPlaying() {
					p.Pause()
				}
			}
		}
		return
	}
	m := musicFor(scene)
	p := m.current()
	if p == nil {
		return
	}
	if !p.IsPlaying() {
		// Picked up from where both fades had reached, so resuming is not an
		// entrance and an entrance does not start at full volume.
		p.SetVolume(m.volume())
		p.Play()
	}
}

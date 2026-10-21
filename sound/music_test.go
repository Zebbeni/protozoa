package sound

import (
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	r "github.com/Zebbeni/protozoa/resources"
)

// TestEverySceneNamesItsOwnTrack: a scene whose table entry was never written
// is silent, and silence is also what a scene with a missing file does — so
// nothing at runtime tells the two apart. SceneSilent is the one that is
// meant to have no track.
func TestEverySceneNamesItsOwnTrack(t *testing.T) {
	seen := map[string]Scene{}
	for _, s := range AllScenes {
		pieces := sceneTracks[s]
		if s == SceneSilent {
			if len(pieces) != 0 {
				t.Errorf("SceneSilent names %v", pieces)
			}
			continue
		}
		if len(pieces) == 0 {
			t.Errorf("scene %d names no track", s)
			continue
		}
		for _, paths := range pieces {
			if len(paths) == 0 {
				t.Errorf("scene %d has a piece naming no file at all", s)
			}
			for _, path := range paths {
				if other, clash := seen[path]; clash {
					t.Errorf("%s is named by scene %d and scene %d, so the two play the same music",
						path, other, s)
				}
				seen[path] = s
			}
		}
	}
	if len(sceneTracks) != len(AllScenes)-1 {
		t.Errorf("%d scenes have tracks against %d scenes that should (every one but SceneSilent)",
			len(sceneTracks), len(AllScenes)-1)
	}
}

// TestAlternatingScenesNameMoreThanOnePiece states what the shape means: a
// scene of one piece loops it forever, and only a scene of several takes
// turns and fades between them. The replay viewer is the one that alternates.
func TestAlternatingScenesNameMoreThanOnePiece(t *testing.T) {
	if n := len(sceneTracks[SceneReplay]); n < 2 {
		t.Errorf("the replay viewer names %d piece(s), so there is nothing to alternate", n)
	}
	if n := len(sceneTracks[SceneMenu]); n != 1 {
		t.Errorf("the menu names %d pieces; it is meant to be one loop", n)
	}
}

// TestOggIsPreferredOverWav: an .ogg of the same music is far smaller, and
// uncompressed PCM is most of the browser build's download, so every scene
// has to pick one up without a code change when it is dropped in.
func TestOggIsPreferredOverWav(t *testing.T) {
	for _, s := range AllScenes {
		for i, paths := range sceneTracks[s] {
			if len(paths) == 0 {
				continue
			}
			if !strings.HasSuffix(paths[0], ".ogg") {
				t.Errorf("scene %d piece %d tries %q first, want the .ogg", s, i, paths[0])
			}
		}
	}
}

// TestEveryTrackOnDiskDecodes: the music is an embedded asset, so a build that
// lost one is silent with nothing to say so at compile time, and a file in a
// format the decoder refuses is the same silence — ebiten's wav decoder takes
// 8- or 16-bit PCM only, so a 24-bit track logs an error and plays nothing.
// This reads each one the way the game does and decodes it.
//
// A scene whose file is absent is skipped rather than failed, since a piece
// that has not been written yet is deliberately quiet. The replay scene is the
// one that has to be there.
func TestEveryTrackOnDiskDecodes(t *testing.T) {
	r.UseDirAssets("..")
	for _, scene := range AllScenes {
		for i, paths := range sceneTracks[scene] {
			found := ""
			for _, path := range paths {
				data, ok := r.ReadAsset(path)
				if !ok {
					continue
				}
				s, err := decode(path, data)
				if err != nil {
					t.Errorf("%s is present and does not decode: %v", path, err)
					break
				}
				if s.Length() <= 0 {
					t.Errorf("%s decodes to %d bytes of audio", path, s.Length())
				}
				// Long enough to be a theme rather than a click: the loop
				// repeats the whole stream, so a stub would stutter.
				seconds := float64(s.Length()) / float64(bytesPerSecond)
				if seconds < 5 {
					t.Errorf("%s is %.1fs long", path, seconds)
				}
				found = path
				t.Logf("scene %d piece %d plays %s (%.0fs, turn %.0fs)",
					scene, i, path, seconds, (seconds*pieceLoops)+pieceFade.Seconds())
				break
			}
			if found == "" {
				t.Errorf("scene %d piece %d: no file found at any of %v", scene, i, paths)
			}
		}
	}
}

// TestASceneWithNoTrackIsSilentAndNotFatal: a piece that has not been written
// yet must leave the game running quietly rather than taking it down or
// borrowing another scene's music.
func TestASceneWithNoTrackIsSilentAndNotFatal(t *testing.T) {
	r.UseDirAssets("..")
	for _, paths := range sceneTracks[SceneSilent] {
		t.Fatalf("SceneSilent names %v", paths)
	}
	if got, _ := loadLoop(nil); got != nil {
		t.Error("a piece with no file built a player")
	}
	if got, _ := loadLoop([]string{"resources/audio/nothing_here.ogg"}); got != nil {
		t.Error("a missing file built a player")
	}
	if got := loadScene([][]string{{"resources/audio/nothing_here.ogg"}}); got != nil {
		t.Error("a scene whose every piece is missing built a scene")
	}
}

// TestMusicIsOffWhereTheSceneHasNoTrack is the semantics the button reads:
// Enabled answers what is audible, which a scene with no music is not, while
// Preferred still carries what the user asked for.
func TestMusicIsOffWhereTheSceneHasNoTrack(t *testing.T) {
	savedScenes, savedScene, savedEnabled := scenes, scene, enabled
	defer func() { scenes, scene, enabled = savedScenes, savedScene, savedEnabled }()

	// A scene that has been entered and has no track: the nil value is what
	// remembers the miss.
	scenes = map[Scene]*sceneMusic{SceneMenu: nil}
	scene = SceneMenu
	enabled = true

	if Enabled() {
		t.Error("music reports as audible in a scene with no track")
	}
	if !Preferred() {
		t.Error("the user asked for music and the preference says otherwise")
	}
}

// TestThePreferenceOutlivesASilentBuild: Enabled answers what is audible and
// Preferred answers what the user asked for, and the two part company exactly
// when there is no track. A new simulation inherits the preference, so reading
// Enabled there would silently turn music off in the settings of everyone on a
// build whose track failed to load.
func TestThePreferenceOutlivesASilentBuild(t *testing.T) {
	savedScenes, savedEnabled := scenes, enabled
	defer func() { scenes, enabled = savedScenes, savedEnabled }()

	scenes = map[Scene]*sceneMusic{}
	SetEnabled(true)
	if Enabled() {
		t.Error("music reports as audible with no player")
	}
	if !Preferred() {
		t.Error("the user asked for music and the preference says otherwise")
	}
	SetEnabled(false)
	if Preferred() {
		t.Error("the preference stayed on after being switched off")
	}
	Toggle()
	if !Preferred() {
		t.Error("the toggle did not reach the preference")
	}
}

// TestAskingForTheCurrentSceneAgainChangesNothing: SetScene is called every
// frame, so a scene that is already current has to be left alone — acting on
// it would restart the loop from the top sixty times a second.
func TestAskingForTheCurrentSceneAgainChangesNothing(t *testing.T) {
	savedScenes, savedScene, savedEnabled := scenes, scene, enabled
	defer func() { scenes, scene, enabled = savedScenes, savedScene, savedEnabled }()

	scenes = map[Scene]*sceneMusic{}
	enabled = false
	scene = SceneSilent

	SetScene(SceneReplay)
	if CurrentScene() != SceneReplay {
		t.Fatalf("the scene is %d after switching to the replay viewer", CurrentScene())
	}
	SetScene(SceneReplay)
	if CurrentScene() != SceneReplay {
		t.Errorf("asking for the current scene again left it on %d", CurrentScene())
	}
	SetScene(SceneMenu)
	if CurrentScene() != SceneMenu {
		t.Errorf("the scene is %d after switching back to the menu", CurrentScene())
	}
}

// TestAPieceFadesInTakesItsTurnAndFadesOut is the shape of a turn: silence at
// the start, full volume through the loops the piece was given, and back to
// silence at the end — so two pieces hand over rather than cutting.
func TestAPieceFadesInTakesItsTurnAndFadesOut(t *testing.T) {
	const length = 30 * time.Second
	turn := length*pieceLoops + pieceFade

	if got := fadeVolume(0, turn); got != 0 {
		t.Errorf("a piece starts its turn at volume %.2f, want silence", got)
	}
	if got := fadeVolume(pieceFade, turn); got != 1 {
		t.Errorf("a piece is at %.2f when its fade in ends, want full", got)
	}
	// Full volume through every loop it was given: the fade out starts only
	// once the loops are done, so what it fades is a repeat nobody hears out.
	for _, at := range []time.Duration{pieceFade, length, length * pieceLoops} {
		if got := fadeVolume(at, turn); got != 1 {
			t.Errorf("at %s of a %s turn the volume is %.2f, want full", at, turn, got)
		}
	}
	if got := fadeVolume(turn, turn); got != 0 {
		t.Errorf("a piece ends its turn at volume %.2f, want silence", got)
	}
	// Monotonic into the handover, or the fade would be audible as a wobble.
	prev := 1.0
	for at := length * pieceLoops; at <= turn; at += pieceFade / 10 {
		v := fadeVolume(at, turn)
		if v > prev+1e-9 {
			t.Errorf("the fade out rises from %.3f to %.3f at %s", prev, v, at)
		}
		prev = v
	}
}

// TestAShortPieceStillFades: the fade is a fixed four seconds and a piece
// could be shorter than two of them, which would make the volume run past 1
// at the start and below 0 at the end if the turn were not consulted.
func TestAShortPieceStillFades(t *testing.T) {
	turn := 2 * time.Second
	for at := time.Duration(0); at <= turn; at += turn / 20 {
		if v := fadeVolume(at, turn); v < 0 || v > 1 {
			t.Fatalf("a %s turn is at volume %.3f after %s", turn, v, at)
		}
	}
	if got := fadeVolume(turn/2, turn); got != 1 {
		t.Errorf("the middle of a short turn is %.2f, want full", got)
	}
}

// TestATurnIsTheLoopsPlusTheHandover: the turn decides when the next piece
// starts, so it has to be the loops the piece was asked for AND the fade that
// follows them. Measured against the pieces actually on disk.
func TestATurnIsTheLoopsPlusTheHandover(t *testing.T) {
	r.UseDirAssets("..")
	for i, paths := range sceneTracks[SceneReplay] {
		var length time.Duration
		for _, path := range paths {
			data, ok := r.ReadAsset(path)
			if !ok {
				continue
			}
			s, err := decode(path, data)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			length = time.Duration(s.Length()) * time.Second / bytesPerSecond
			break
		}
		if length == 0 {
			t.Fatalf("piece %d: no file found at any of %v", i, paths)
		}
		turn := turnFor(length)
		if turn <= length {
			t.Errorf("piece %d is %s long and gets a %s turn, so it is cut short", i, length, turn)
		}
		if got := turn - length*pieceLoops; got != pieceFade {
			t.Errorf("piece %d leaves %s for the handover, want %s", i, got, pieceFade)
		}
		t.Logf("piece %d: %s long, %d loops, %s turn", i, length, pieceLoops, turn)
	}
}

// TestEverySceneWithMusicFadesIn: no screen change drops the music in at
// full volume, so every scene that names a piece has an opening fade. The
// lengths differ because the recordings do — the replay pieces start at the
// loudness they keep (-16.5 and -17.0 dBFS over their first half second
// against full-piece levels of -17.2 and -17.6) where the menu's opens 6 dB
// under its own average, so the menu is given less and the recording does
// the rest.
func TestEverySceneWithMusicFadesIn(t *testing.T) {
	for _, s := range AllScenes {
		if len(sceneTracks[s]) == 0 {
			continue // SceneSilent has nothing to fade
		}
		if sceneFadeIn[s] <= 0 {
			t.Errorf("scene %d names music and has no opening fade, so it starts at full volume", s)
		}
	}
	if sceneFadeIn[SceneMenu] >= sceneFadeIn[SceneReplay] {
		t.Errorf("the menu fades in over %v against the replay viewer's %v; the menu's piece opens "+
			"quietly on its own and is meant to need less",
			sceneFadeIn[SceneMenu], sceneFadeIn[SceneReplay])
	}
}

// TestEveryOpeningFadeNamesASceneWithPieces: a fade named for a scene with no
// music is a line that cannot do anything, which is the stale-entry trap this
// project has hit with settings lists.
func TestEveryOpeningFadeNamesASceneWithPieces(t *testing.T) {
	for s, fade := range sceneFadeIn {
		if len(sceneTracks[s]) == 0 {
			t.Errorf("scene %d has a %v opening fade and names no music", s, fade)
		}
		if fade < 0 {
			t.Errorf("scene %d has a negative opening fade of %v", s, fade)
		}
	}
}

// TestAnOpeningFadeStartsSilentAndRisesToFull is the shape of an entrance.
func TestAnOpeningFadeStartsSilentAndRisesToFull(t *testing.T) {
	const fade = 8 * time.Second
	if got := entryVolume(0, fade); got != 0 {
		t.Errorf("an entrance starts at volume %.4f, want silence", got)
	}
	if got := entryVolume(fade, fade); got != 1 {
		t.Errorf("an entrance ends at volume %.4f, want full", got)
	}
	if got := entryVolume(fade+time.Second, fade); got != 1 {
		t.Errorf("an entrance is at %.4f past its end, want full", got)
	}
	prev := -1.0
	for at := time.Duration(0); at <= fade; at += fade / 40 {
		v := entryVolume(at, fade)
		if v < 0 || v > 1 {
			t.Fatalf("volume %.4f at %v of a %v entrance", v, at, fade)
		}
		if v < prev-1e-9 {
			t.Errorf("the entrance falls from %.4f to %.4f at %v", prev, v, at)
		}
		prev = v
	}
	// No fade asked for is no fade applied, which is what every other scene
	// relies on to keep playing at full volume.
	if got := entryVolume(0, 0); got != 1 {
		t.Errorf("a scene with no opening fade starts at %.4f, want full", got)
	}
}

// TestTheOpeningFadeIsGentlerThanTheHandover is the point of having two
// curves. The handover is linear in amplitude, which is front-loaded in dB:
// it is within 9 dB of full after one second, so lengthening it would not fix
// an entrance that arrives too fast. The squared curve is quieter at every
// point of its first four seconds, and more than ten times quieter at one.
func TestTheOpeningFadeIsGentlerThanTheHandover(t *testing.T) {
	const fade = 8 * time.Second
	// A turn long enough that its own fade out is nowhere near these points.
	turn := 10 * time.Minute
	for at := fade / 80; at <= pieceFade; at += fade / 80 {
		entry := entryVolume(at, fade)
		handover := fadeVolume(at, turn)
		if entry >= handover {
			t.Errorf("at %v the entrance is at %.4f against the handover's %.4f", at, entry, handover)
		}
	}
	if entry, handover := entryVolume(time.Second, fade), fadeVolume(time.Second, turn); entry > handover/10 {
		t.Errorf("one second in, the entrance is at %.4f against a tenth of the handover's %.4f", entry, handover)
	}
}

// TestATurnReachingFullDoesNotOverrideAnEntrance: the two fades run at once
// when a scene is entered, and the piece has to follow whichever is quieter
// or the entrance ends after four seconds whatever it was set to.
func TestATurnReachingFullDoesNotOverrideAnEntrance(t *testing.T) {
	m := &sceneMusic{
		players: make([]*audio.Player, 2),
		turns:   []time.Duration{10 * time.Minute, 10 * time.Minute},
		fadeIn:  8 * time.Second,
	}
	if got := m.volume(); got != 0 {
		t.Errorf("a scene opens at volume %.4f, want silence", got)
	}
	// Past the handover fade, where the turn alone would say full volume.
	m.played, m.entry = 5*time.Second, 5*time.Second
	if turn := fadeVolume(m.played, m.turns[0]); turn != 1 {
		t.Fatalf("the handover fade is %.4f at %v, so this is not testing the overlap", turn, m.played)
	}
	want := musicLevel * entryVolume(m.entry, m.fadeIn)
	if got := m.volume(); got != want {
		t.Errorf("five seconds into an %v entrance the volume is %.4f, want the entrance's %.4f", m.fadeIn, got, want)
	}
}

// TestSwitchingScreensFadesInAgain: the fade is per ENTRY, not per scene, so
// leaving a scene and coming back starts from silence rather than resuming at
// whatever volume it had. Needs no audio device — SetScene does this
// bookkeeping whether or not there is anything to play.
func TestSwitchingScreensFadesInAgain(t *testing.T) {
	savedScenes, savedScene, savedEnabled := scenes, scene, enabled
	defer func() { scenes, scene, enabled = savedScenes, savedScene, savedEnabled }()

	enabled = false
	replay := &sceneMusic{
		players: make([]*audio.Player, 2),
		turns:   []time.Duration{10 * time.Minute, 10 * time.Minute},
		fadeIn:  8 * time.Second,
	}
	menu := &sceneMusic{
		players: make([]*audio.Player, 1),
		turns:   []time.Duration{10 * time.Minute},
		fadeIn:  4 * time.Second,
	}
	scenes = map[Scene]*sceneMusic{SceneMenu: menu, SceneReplay: replay}
	scene = SceneSilent

	SetScene(SceneReplay)
	if got := replay.volume(); got != 0 {
		t.Fatalf("the replay viewer opens at volume %.4f, want silence", got)
	}
	// Let its fade run out, which is where the old behaviour stopped.
	replay.entry, replay.played = replay.fadeIn, 30*time.Second
	if got := replay.volume(); got != musicLevel {
		t.Fatalf("the replay viewer is at %.4f once its fade is done, want the %.2f ceiling", got, musicLevel)
	}

	SetScene(SceneMenu)
	if got := menu.volume(); got != 0 {
		t.Errorf("switching to the menu starts at volume %.4f, want silence", got)
	}
	menu.entry = menu.fadeIn

	SetScene(SceneReplay)
	if replay.entry != 0 {
		t.Errorf("returning to the replay viewer left %v of its fade already elapsed", replay.entry)
	}
	if got := replay.volume(); got != 0 {
		t.Errorf("returning to the replay viewer resumes at volume %.4f, want silence", got)
	}
}

// TestASinglePieceSceneStillPlaysAtFullVolume: a scene of one piece takes no
// turns, so once its opening fade is done it stays at full volume forever
// rather than fading for a handover that never comes.
func TestASinglePieceSceneStillPlaysAtFullVolume(t *testing.T) {
	m := &sceneMusic{players: make([]*audio.Player, 1), turns: []time.Duration{time.Minute}}
	if got := m.volume(); got != musicLevel {
		t.Errorf("a one-piece scene plays at %.4f, want the %.2f ceiling", got, musicLevel)
	}
	m.played = 59 * time.Second
	if got := m.volume(); got != musicLevel {
		t.Errorf("a one-piece scene is at %.4f near the end of a turn it does not take", got)
	}
}

// TestTheReplaySceneIsBuiltWithItsOpeningFade is the wiring: the fade is read
// in musicFor, where the scene is known, so a table entry that never reaches
// the players would leave the entrance at full volume with the table looking
// right. Needs an audio device to build players on, so it skips rather than
// failing where there is none.
func TestTheReplaySceneIsBuiltWithItsOpeningFade(t *testing.T) {
	savedScenes, savedScene, savedEnabled := scenes, scene, enabled
	defer func() { scenes, scene, enabled = savedScenes, savedScene, savedEnabled }()

	r.UseDirAssets("..")
	Init(false)
	scenes = map[Scene]*sceneMusic{}
	m := musicFor(SceneReplay)
	if m == nil {
		t.Skip("no audio device: the replay scene built no players")
	}
	if m.fadeIn != sceneFadeIn[SceneReplay] {
		t.Errorf("the replay scene was built with a %v opening fade, want %v", m.fadeIn, sceneFadeIn[SceneReplay])
	}
	if got := m.volume(); got != 0 {
		t.Errorf("the replay viewer opens at volume %.4f, want silence", got)
	}
}

// TestTheAppStartsQuietAndEverySwitchFadesIn walks the lifecycle the runner
// drives — Init, the first scene, a switch and a switch back — against real
// players, because the volume actually set on a player is the thing the
// listener hears and the unit tests above only reach the arithmetic.
//
// Needs an audio device to build players on, so it skips rather than failing
// where there is none.
func TestTheAppStartsQuietAndEverySwitchFadesIn(t *testing.T) {
	savedScenes, savedScene, savedEnabled := scenes, scene, enabled
	defer func() { scenes, scene, enabled = savedScenes, savedScene, savedEnabled }()

	r.UseDirAssets("..")
	Init(true)
	scenes, scene = map[Scene]*sceneMusic{}, SceneSilent

	step := func(s Scene, dt time.Duration) (*audio.Player, float64) {
		t.Helper()
		SetScene(s)
		Update(dt)
		m := scenes[s]
		if m == nil {
			t.Skipf("no audio device: scene %d built no players", s)
		}
		return m.current(), m.current().Volume()
	}

	// The app's first frame: the menu must not arrive at full volume.
	if _, v := step(SceneMenu, 0); v != 0 {
		t.Errorf("the app opens at volume %.4f, want silence", v)
	}
	if _, v := step(SceneMenu, time.Second); v <= 0 || v >= musicLevel {
		t.Errorf("one second into the opening fade the menu is at %.4f, want part way up", v)
	}
	// Finish the menu's fade so the switch below is from full volume.
	_, v := step(SceneMenu, sceneFadeIn[SceneMenu])
	if v != musicLevel {
		t.Fatalf("the menu is at %.4f once its fade is done, want the %.2f ceiling", v, musicLevel)
	}

	// Switching screens fades in rather than cutting in.
	if _, v := step(SceneReplay, 0); v != 0 {
		t.Errorf("switching to the replay viewer starts at volume %.4f, want silence", v)
	}
	_, _ = step(SceneReplay, sceneFadeIn[SceneReplay])

	// And so does switching back, which is the half that used to resume at
	// whatever volume the scene was left at.
	if _, v := step(SceneMenu, 0); v != 0 {
		t.Errorf("switching back to the menu resumes at volume %.4f, want silence", v)
	}
}

// TestNoPieceEverPlaysAboveTheCeiling: musicLevel is what the fades mean by
// full volume, and it is applied in ONE place so that every player's volume
// passes through it. A second path that set a volume directly would play the
// music at the level the recording was mastered to, over the top of whichever
// piece was already fading.
func TestNoPieceEverPlaysAboveTheCeiling(t *testing.T) {
	if musicLevel <= 0 || musicLevel > 1 {
		t.Fatalf("musicLevel is %v, which is not a volume", musicLevel)
	}
	for _, pieces := range []int{1, 2} {
		m := &sceneMusic{
			players: make([]*audio.Player, pieces),
			turns:   []time.Duration{time.Minute, time.Minute},
			fadeIn:  8 * time.Second,
		}
		for entry := time.Duration(0); entry <= 2*m.fadeIn; entry += time.Second {
			for played := time.Duration(0); played <= m.turns[0]; played += 2 * time.Second {
				m.entry, m.played = entry, played
				v := m.volume()
				if v < 0 || v > musicLevel {
					t.Fatalf("%d piece(s), %v into the entrance and %v into the turn: volume %.4f, ceiling %.2f",
						pieces, entry, played, v, musicLevel)
				}
			}
		}
	}
}

// Package animation owns the data and timing needed to animate organism
// sprites across sim cycle transitions.
//
// # Pacing overview
//
// The simulation advances in discrete cycles: each cycle, every organism
// chooses an action (move, attack, eat, ...) and that action is resolved.
// To give the viewer time to see what happened, we decouple the sim from
// the ebiten tick loop and drive playback off wall-clock time:
//
//   - We target AnimationFPS (12) visible frames per second.
//   - A full cycle transition spans BaseFramesPerCycle (4) frames at
//     normal speed, giving 3 sim cycles per second at Speed 1.
//   - Speed is a linear multiplier on that base cycle rate.
//
// # Speed semantics
//
// Speed is the playback multiplier requested by the user. We do NOT scale
// the animation frame rate (sprite frames still tick at 12 fps so the
// animation always feels smooth); instead we shrink the time per cycle,
// which collapses the available animation window:
//
//	Speed 1  -> 1 cycle / 333ms -> 4 animation frames / cycle (full animation)
//	Speed 2  -> 1 cycle / 167ms -> 2 animation frames / cycle (compressed)
//	Speed 3  -> 1 cycle / 111ms -> ~1 animation frame / cycle (barely animated)
//	Speed 4  -> 1 cycle /  83ms -> 1 animation frame / cycle  (no interpolation)
//	Speed >4 -> multiple cycles per animation frame           (animation skipped)
//
// At Speed > BaseFramesPerCycle the cycle duration is shorter than a
// render frame, so the renderer cannot display every intermediate state.
// We deliberately accept that tradeoff: users who ask for very fast
// playback want to see results, not animation.
//
// # Usage
//
// The caller driving playback (replay.Controller) owns a *State. Each
// ebiten Update it:
//
//  1. Computes the elapsed wall-clock time since the current cycle began.
//  2. If enough time has passed to complete the current cycle, calls
//     BeforeUpdate(pre-update infos) → sim.Update() → AfterUpdate(post-update infos).
//     AfterUpdate rebuilds the Frames map and advances the cycle clock.
//  3. The renderer reads Progress() / FrameIndex() / Frames to draw.
package animation

import (
	"time"

	"github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
	"github.com/lucasb-eyer/go-colorful"
)

const (
	// AnimationFPS is the target frame rate for organism animation rendering.
	AnimationFPS = 12

	// BaseFramesPerCycle is how many animation frames make up a full cycle
	// transition at Speed 1. See package docs for the reasoning.
	BaseFramesPerCycle = 4

	// frameDuration is how long a single animation frame lasts on the wall clock.
	frameDuration = time.Second / AnimationFPS

	// baseCycleDuration is how long one sim cycle takes to play at Speed 1.
	baseCycleDuration = frameDuration * BaseFramesPerCycle
)

// Animation identifies which per-action sprite sheet to play. One sheet per
// Animation; direction is expressed via rotation at draw time, not by having
// per-direction sheets.
type Animation int

const (
	AnimIdle Animation = iota
	AnimMove
	AnimBlocked
	AnimTurn
	AnimAttack
	AnimEat
	AnimChemo
)

// AllAnimations lists every Animation value, for resource preloading.
var AllAnimations = [...]Animation{AnimIdle, AnimMove, AnimBlocked, AnimTurn, AnimAttack, AnimEat, AnimChemo}

// ForAction maps a resolved decision.Action to the Animation sheet that
// should play during its cycle transition. This is the position-agnostic
// default; for cases where the result of the action matters (e.g. a move
// that didn't change position because the path was blocked), use ForFrame.
func ForAction(a decision.Action) Animation {
	switch a {
	case decision.ActMove:
		return AnimMove
	case decision.ActAttack:
		return AnimAttack
	case decision.ActEat:
		return AnimEat
	case decision.ActTurnLeft, decision.ActTurnRight:
		return AnimTurn
	case decision.ActChemosynthesis:
		return AnimChemo
	default:
		return AnimIdle
	}
}

// ForFrame picks the Animation for a Frame, accounting for outcomes that
// aren't visible from the action alone. A Move whose FromLocation equals
// its ToLocation is a blocked move (the organism tried to advance but
// couldn't) and gets its own animation so we don't play the 2-cell travel
// sprite in place.
func ForFrame(f Frame) Animation {
	if f.Action == decision.ActMove && f.FromLocation == f.ToLocation {
		return AnimBlocked
	}
	return ForAction(f.Action)
}

// Frame captures everything the renderer needs to animate one organism's
// transition from its pre-cycle state to its current state. Populated by
// State.AfterUpdate; consumed by the grid renderer.
type Frame struct {
	FromLocation utils.Point
	ToLocation   utils.Point
	Direction    utils.Point
	Action       decision.Action
	Color        colorful.Color
	Size         float64
}

// State holds the current animation batch and cycle timing. One instance per
// playback session; owned by the thing driving sim.Update (replay.Controller).
type State struct {
	// Frames indexed by organism ID for the most recent cycle transition.
	// Safe to read from the render goroutine between BeforeUpdate/AfterUpdate calls.
	Frames map[int]Frame

	// Current playback speed. The caller updates this when the user changes speed;
	// we consult it each call to CycleDuration.
	Speed int

	// preSnap holds pre-Update locations captured in BeforeUpdate. Consumed by
	// the next AfterUpdate to build Frames.
	preSnap map[int]utils.Point

	// cycleStart is when the current cycle's animation window began. The
	// renderer's Progress() is computed relative to this.
	cycleStart time.Time
}

// NewState returns a State ready for use at Speed 1. cycleStart is set to now.
func NewState() *State {
	return &State{
		Frames:     make(map[int]Frame),
		Speed:      1,
		preSnap:    make(map[int]utils.Point),
		cycleStart: time.Now(),
	}
}

// CycleDuration returns how long a cycle's animation window lasts at the
// current Speed. Used both for deciding when to advance the sim and for
// computing render progress.
func (s *State) CycleDuration() time.Duration {
	sp := s.Speed
	if sp < 1 {
		sp = 1
	}
	return baseCycleDuration / time.Duration(sp)
}

// ShouldAdvance reports whether enough wall-clock time has elapsed since the
// last cycle started to begin the next one.
func (s *State) ShouldAdvance() bool {
	return time.Since(s.cycleStart) >= s.CycleDuration()
}

// BeforeUpdate snapshots current organism locations. Must be called
// immediately before sim.Update so we know where each organism started
// this cycle.
func (s *State) BeforeUpdate(infos map[int]*organism.Info) {
	snap := make(map[int]utils.Point, len(infos))
	for id, info := range infos {
		snap[id] = info.Location
	}
	s.preSnap = snap
}

// AfterUpdate builds the Frame batch from post-update infos, pairing each
// organism with the location it came from. Must be called immediately after
// sim.Update (before any further UpdateAction calls overwrite info.Action).
//
// Advances the cycle clock by one CycleDuration rather than resetting it to
// time.Now(), so if the caller is catching up on multiple cycles in one
// ebiten tick, the accumulated time is consumed correctly instead of being
// dropped on each iteration.
func (s *State) AfterUpdate(infos map[int]*organism.Info) {
	frames := make(map[int]Frame, len(infos))
	for id, info := range infos {
		from, ok := s.preSnap[id]
		if !ok {
			// Newly-born organism this cycle: no prior location, so
			// animate in place.
			from = info.Location
		}
		frames[id] = Frame{
			FromLocation: from,
			ToLocation:   info.Location,
			Direction:    info.Direction,
			Action:       info.Action,
			Color:        info.Color,
			Size:         info.Size,
		}
	}
	s.Frames = frames
	s.cycleStart = s.cycleStart.Add(s.CycleDuration())

	// Guard: if we fall far behind (e.g. long pause, tab inactive), snap the
	// clock up to now so we don't fast-forward through hundreds of cycles.
	if time.Since(s.cycleStart) > time.Second {
		s.cycleStart = time.Now()
	}
}

// ResetClock re-anchors the cycle clock to now. Callers use this after
// unpausing or seeking to avoid a catch-up burst.
func (s *State) ResetClock() {
	s.cycleStart = time.Now()
}

// Progress returns how far into the current cycle's animation we are,
// clamped to [0, 1]. 0 means the cycle just advanced; 1 means the cycle
// is complete and we're waiting for the next advance.
func (s *State) Progress() float64 {
	dur := s.CycleDuration()
	if dur <= 0 {
		return 1
	}
	p := float64(time.Since(s.cycleStart)) / float64(dur)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// FrameIndex returns which of the BaseFramesPerCycle sprite frames to show.
// At Speed > BaseFramesPerCycle there is only one visible frame per cycle,
// so we snap to the final frame.
func (s *State) FrameIndex() int {
	sp := s.Speed
	if sp < 1 {
		sp = 1
	}
	// Number of distinct sprite frames we can actually display this cycle.
	visibleFrames := BaseFramesPerCycle / sp
	if visibleFrames < 1 {
		return BaseFramesPerCycle - 1
	}
	idx := int(s.Progress() * float64(BaseFramesPerCycle))
	if idx >= BaseFramesPerCycle {
		idx = BaseFramesPerCycle - 1
	}
	return idx
}

// AnimatesPosition reports whether the renderer should interpolate positions
// this cycle. False at very high speeds where a single render frame covers
// the full cycle (or more); the renderer should just show the final state.
func (s *State) AnimatesPosition() bool {
	return s.Speed <= BaseFramesPerCycle
}

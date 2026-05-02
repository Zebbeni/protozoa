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
// # Per-sprite-set frame counts
//
// Each sprite set authors a different number of frames per cycle
// (resolution / 4: 4x4 → 1 frame, 8x8 → 2, 16x16 → 4). Cycle timing
// doesn't change with the sprite set — we just divide the same
// Progress() into a different number of equal slots. SpriteFrameIndex
// is the helper for that: callers pass the active sprite set's frame
// count and get back the index to display.
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

// Animation identifies which per-action sprite sheet to play. One sheet
// per Animation; direction is expressed via rotation at draw time.
type Animation int

const (
	AnimIdle Animation = iota
	AnimMove
	AnimBlocked
	AnimTurnLeft
	AnimTurnRight
	AnimAttack
	AnimEat
	AnimChemo
	AnimChemoFail
	AnimDie
)

// AllAnimations lists every Animation value, for resource preloading.
var AllAnimations = [...]Animation{AnimIdle, AnimMove, AnimBlocked, AnimTurnLeft, AnimTurnRight, AnimAttack, AnimEat, AnimChemo, AnimChemoFail, AnimDie}

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
	case decision.ActTurnLeft:
		return AnimTurnLeft
	case decision.ActTurnRight:
		return AnimTurnRight
	case decision.ActChemosynthesis:
		return AnimChemo
	case decision.ActIdle:
		return AnimIdle
	default:
		return AnimIdle
	}
}

// ForFrame picks the Animation for a Frame, accounting for outcomes that
// aren't visible from the action alone:
//
//   - Dying frames (the organism died this cycle) always play AnimDie,
//     overriding whatever their last action was.
//   - A Move whose FromLocation equals its ToLocation is a blocked move —
//     the organism tried to advance but couldn't — and gets AnimBlocked
//     so we don't play the 2-cell travel sprite in place.
//   - A Chemosynthesis action whose ChemoFailed flag is set (organism
//     was outside its pH tolerance range and gained no health) plays
//     AnimChemoFail instead of AnimChemo.
func ForFrame(f Frame) Animation {
	if f.Dying {
		return AnimDie
	}
	if f.Action == decision.ActMove && f.FromLocation == f.ToLocation {
		return AnimBlocked
	}
	if f.Action == decision.ActChemosynthesis && f.ChemoFailed {
		return AnimChemoFail
	}
	return ForAction(f.Action)
}

// Frame captures everything the renderer needs to animate one organism's
// transition from its pre-cycle state to its current state. Populated by
// State.AfterUpdate; consumed by the grid renderer.
//
// Dying is true for organisms that existed before the most recent cycle
// but are gone afterwards. Their FromLocation / Direction / Size / Color
// are the snapshot from just before death, and ForFrame forces AnimDie on
// them regardless of their last action. These frames only live for one
// cycle — the next AfterUpdate rebuilds Frames and drops them.
type Frame struct {
	FromLocation utils.Point
	ToLocation   utils.Point
	Direction    utils.Point
	Action       decision.Action
	Color        colorful.Color
	Size         float64
	PhEffect     float64
	Dying        bool
	// ChemoFailed is true when the organism attempted chemosynthesis but
	// was outside its pH tolerance range (no health gained). Routed
	// through ForFrame to pick AnimChemoFail instead of AnimChemo.
	ChemoFailed bool
}

// State holds the current animation batch and cycle timing. One instance per
// playback session; owned by the thing driving sim.Update (replay.Controller).
type State struct {
	// Frames indexed by organism ID for the most recent cycle transition.
	// Safe to read from the render goroutine between BeforeUpdate/AfterUpdate calls.
	Frames map[int]Frame

	// Current playback speed multiplier. 1 = real-time. Values < 1 (e.g.
	// 0.5, 0.25) stretch the cycle window for slow-motion playback;
	// values > 1 compress it. The caller updates this when the user
	// changes speed; we consult it each call to CycleDuration.
	Speed float64

	// preSnap holds pre-Update organism snapshots captured in BeforeUpdate.
	// Used by the next AfterUpdate both to pair each surviving organism
	// with its prior location (move animations) and to detect organisms
	// that died during the cycle (present in preSnap but not in the post
	// infos) so we can emit Dying frames for them.
	preSnap map[int]organism.Info

	// cycleStart is when the current cycle's animation window began. The
	// renderer's Progress() is computed relative to this.
	cycleStart time.Time
}

// NewState returns a State ready for use at Speed 1. cycleStart is set to now.
func NewState() *State {
	return &State{
		Frames:     make(map[int]Frame),
		Speed:      1,
		preSnap:    make(map[int]organism.Info),
		cycleStart: time.Now(),
	}
}

// CycleDuration returns how long a cycle's animation window lasts at the
// current Speed. Used both for deciding when to advance the sim and for
// computing render progress.
func (s *State) CycleDuration() time.Duration {
	sp := s.Speed
	if sp <= 0 {
		sp = 1
	}
	return time.Duration(float64(baseCycleDuration) / sp)
}

// ShouldAdvance reports whether enough wall-clock time has elapsed since the
// last cycle started to begin the next one.
func (s *State) ShouldAdvance() bool {
	return time.Since(s.cycleStart) >= s.CycleDuration()
}

// BeforeUpdate snapshots current organism Infos (value copies). Must be
// called immediately before sim.Update so we know each organism's
// location, direction, size, colour etc. at the start of the cycle — and
// so we can detect which organisms were alive then but gone by AfterUpdate.
func (s *State) BeforeUpdate(infos map[int]*organism.Info) {
	snap := make(map[int]organism.Info, len(infos))
	for id, info := range infos {
		snap[id] = *info
	}
	s.preSnap = snap
}

// AfterUpdate builds the Frame batch from post-update infos. Must be called
// immediately after sim.Update (before any further UpdateAction calls
// overwrite info.Action).
//
// Produces two kinds of frames:
//   - Live organisms (in post infos): paired with their pre-update
//     location for movement interpolation.
//   - Dying organisms (in preSnap but NOT in post infos): rendered at
//     their last-known location with Dying = true. ForFrame routes them
//     to AnimDie. The frame only persists until the next AfterUpdate,
//     giving the death animation exactly one cycle to play.
//
// Advances the cycle clock by one CycleDuration rather than resetting it
// to time.Now(), so if the caller is catching up on multiple cycles in
// one ebiten tick the accumulated time is consumed correctly instead of
// being dropped on each iteration.
func (s *State) AfterUpdate(infos map[int]*organism.Info) {
	frames := make(map[int]Frame, len(infos)+len(s.preSnap))
	for id, info := range infos {
		from := info.Location
		action := info.Action
		switch {
		case info.BornThisCycle:
			// Newborn: animate as if the organism just moved into
			// its starting cell from the parent's cell. Spawn logic
			// always orients the child away from its parent
			// (Direction = parent→child step), so
			// Location.Sub(Direction) recovers the parent's cell
			// without needing to carry it through. Override the
			// action to Move regardless of info.Action (newborns
			// default to chemosynthesis) so ForFrame picks AnimMove.
			//
			// We use the explicit flag rather than preSnap membership
			// so post-seek frames don't misclassify surviving
			// organisms as newborns — SeekToCycle's catch-up loop
			// bypasses BeforeUpdate, so preSnap can be stale.
			from = info.Location.Sub(info.Direction)
			action = decision.ActMove
		default:
			if pre, ok := s.preSnap[id]; ok {
				from = pre.Location
			}
		}
		frames[id] = Frame{
			FromLocation: from,
			ToLocation:   info.Location,
			Direction:    info.Direction,
			Action:       action,
			Color:        info.Color,
			Size:         info.Size,
			PhEffect:     info.PhEffect,
			ChemoFailed:  info.ChemoFailed,
		}
	}
	for id, pre := range s.preSnap {
		if _, alive := infos[id]; alive {
			continue
		}
		frames[id] = Frame{
			FromLocation: pre.Location,
			ToLocation:   pre.Location,
			Direction:    pre.Direction,
			Action:       pre.Action,
			Color:        pre.Color,
			Size:         pre.Size,
			PhEffect:     pre.PhEffect,
			Dying:        true,
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

// SpriteFrameIndex returns which of framesInSet sprite frames to show for
// the current cycle progress, parameterised by the active sprite set's
// per-cycle frame count (1 for 4x4, 2 for 8x8, 4 for 16x16).
//
// At Speed > framesInSet the cycle window is shorter than one visible
// frame per sprite frame, so we snap to the last frame (the resolved
// state of the animation). framesInSet <= 0 is treated as 1.
func (s *State) SpriteFrameIndex(framesInSet int) int {
	if framesInSet < 1 {
		return 0
	}
	if s.Speed > float64(framesInSet) {
		return framesInSet - 1
	}
	idx := int(s.Progress() * float64(framesInSet))
	if idx >= framesInSet {
		idx = framesInSet - 1
	}
	return idx
}

// AnimatesPosition reports whether the renderer should animate at the
// current Speed — both sprite-frame cycling AND position interpolation.
// False above 2x, where each cycle's wall-clock window is short enough
// that frame/position changes read as flicker more than motion; the
// renderer should pin to frame 0 and snap to the final position.
func (s *State) AnimatesPosition() bool {
	return s.Speed <= 2
}

// LoopProgress computes a looping (progress, frameIdx) pair from an elapsed
// wall-clock duration, as if the animation had been playing at Speed 1 from
// time zero. framesInSet is the sprite set's per-cycle frame count — the
// returned index is divided proportionally across that many frames so the
// loop displays each authored sprite for an equal share of the cycle.
// Intended for standalone demos / previews that want to loop the same
// animation forever without a full sim-driven State.
func LoopProgress(elapsed time.Duration, framesInSet int) (progress float64, frameIdx int) {
	if baseCycleDuration <= 0 {
		return 0, 0
	}
	mod := elapsed % baseCycleDuration
	if mod < 0 {
		mod += baseCycleDuration
	}
	progress = float64(mod) / float64(baseCycleDuration)
	if framesInSet < 1 {
		return progress, 0
	}
	frameIdx = int(progress * float64(framesInSet))
	if frameIdx >= framesInSet {
		frameIdx = framesInSet - 1
	}
	return
}

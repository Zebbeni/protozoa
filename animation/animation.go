// Package animation owns the data and timing needed to animate organism sprites across sim cycle transitions.
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

	// BaseFramesPerCycle is how many animation frames make up a full cycle transition at Speed 1. See package docs for the reasoning.
	BaseFramesPerCycle = 4

	// frameDuration is how long a single animation frame lasts on the wall clock.
	frameDuration = time.Second / AnimationFPS

	// baseCycleDuration is how long one sim cycle takes to play at Speed 1.
	baseCycleDuration = frameDuration * BaseFramesPerCycle
)

type Animation int

const (
	AnimIdle Animation = iota
	AnimMove
	AnimBlocked
	AnimTurnLeft
	AnimTurnRight
	AnimAttack
	AnimEat
	AnimEatFail
	AnimChemo
	AnimChemoFail
	AnimDie
	AnimDig
	// AnimAttackMove is an attack that killed and took the victim's cell. Its
	// two cells are the one the killer left and the one it took.
	AnimAttackMove
	// AnimSpawn is the parent on the cycle it hands a child its health. The
	// child has no animation of its own: it is drawn moving out of the
	// parent's cell (see BornThisCycle in AfterUpdate).
	AnimSpawn
)

var AllAnimations = [...]Animation{
	AnimIdle, AnimMove, AnimBlocked,
	AnimTurnLeft, AnimTurnRight,
	AnimAttack, AnimEat, AnimEatFail,
	AnimChemo, AnimChemoFail,
	AnimDie,
	AnimDig,
	AnimAttackMove,
	AnimSpawn,
}

// ForStatus maps a resolved organism.Status to the Animation sheet that should play during its cycle transition.
func ForStatus(s organism.Status) Animation {
	switch s {
	case organism.StatusMoveSuccess:
		return AnimMove
	case organism.StatusMoveBlocked:
		return AnimBlocked
	case organism.StatusAttacking:
		return AnimAttack
	case organism.StatusAttackMove:
		return AnimAttackMove
	case organism.StatusEatSuccess:
		return AnimEat
	case organism.StatusEatFailed:
		return AnimEatFail
	case organism.StatusTurnLeft:
		return AnimTurnLeft
	case organism.StatusTurnRight:
		return AnimTurnRight
	case organism.StatusChemoSuccess:
		return AnimChemo
	case organism.StatusChemoFailed:
		return AnimChemoFail
	case organism.StatusDying:
		return AnimDie
	case organism.StatusDigging:
		return AnimDig
	case organism.StatusSpawning:
		return AnimSpawn
	default:
		// Every status has a sheet of its own; idle is what is left.
		return AnimIdle
	}
}

func ForFrame(f Frame) Animation {
	return ForStatus(f.Status)
}

// Frame captures everything the renderer needs to animate one organism's transition from its pre-cycle state to its current state.
type Frame struct {
	FromLocation utils.Point
	ToLocation   utils.Point
	Direction    utils.Point
	Action       decision.Action
	Color        colorful.Color
	Size         float64
	Status       organism.Status
}

type State struct {
	// Frames indexed by organism ID for the most recent cycle transition.
	Frames map[int]Frame

	Speed float64

	// preSnap holds pre-Update organism snapshots captured in BeforeUpdate.
	preSnap map[int]organism.Info

	// cycleStart is when the current cycle's animation window began.
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

// CycleDuration returns how long a cycle's animation window lasts at the current Speed.
func (s *State) CycleDuration() time.Duration {
	sp := s.Speed
	if sp <= 0 {
		sp = 1
	}
	return time.Duration(float64(baseCycleDuration) / sp)
}

// ShouldAdvance reports whether enough wall-clock time has elapsed since the last cycle started to begin the next one.
func (s *State) ShouldAdvance() bool {
	return time.Since(s.cycleStart) >= s.CycleDuration()
}

// BeforeUpdate snapshots current organism Infos (value copies).
func (s *State) BeforeUpdate(infos map[int]*organism.Info) {
	snap := make(map[int]organism.Info, len(infos))
	for id, info := range infos {
		snap[id] = *info
	}
	s.preSnap = snap
}

// AfterUpdate builds the Frame batch from post-update infos.
func (s *State) AfterUpdate(infos map[int]*organism.Info) {
	frames := make(map[int]Frame, len(infos)+len(s.preSnap))
	for id, info := range infos {
		from := info.Location
		action := info.Action
		switch {
		case info.BornThisCycle:
			// Newborn: animate as if the organism just moved into its starting cell from the parent's cell.
			from = info.Location.Sub(info.Direction)
			action = decision.ActMove
		default:
			if pre, ok := s.preSnap[id]; ok {
				from = pre.Location
			}
		}
		// Newborns synthesize an inbound "move from parent" frame (see the BornThisCycle case above).
		status := info.Status
		if info.BornThisCycle {
			status = organism.StatusMoveSuccess
		}
		frames[id] = Frame{
			FromLocation: from,
			ToLocation:   info.Location,
			Direction:    drawDirection(info.Direction, status),
			Action:       action,
			Color:        info.Color,
			Size:         info.Size,
			Status:       status,
		}
	}
	// Note: with the dying lifecycle, organisms that took lethal damage stay in `infos` with Status = Dying for one more cycle before finalizeDeaths removes them.
	s.Frames = frames
	s.cycleStart = s.cycleStart.Add(s.CycleDuration())

	// Guard: if we fall far behind (e.g. long pause, tab inactive), snap the clock up to now so we don't fast-forward through hundreds of cycles.
	if time.Since(s.cycleStart) > time.Second {
		s.cycleStart = time.Now()
	}
}

// drawDirection is the heading a frame's sprite is drawn at.
func drawDirection(direction utils.Point, status organism.Status) utils.Point {
	switch status {
	case organism.StatusTurnLeft:
		return direction.Right()
	case organism.StatusTurnRight:
		return direction.Left()
	}
	return direction
}

func (s *State) ResetClock() {
	s.cycleStart = time.Now()
}

// Progress returns how far into the current cycle's animation we are, clamped to [0, 1].
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

// SpriteFrameIndex returns which of framesInSet sprite frames to show for the current cycle progress, parameterised by the active sprite set's per-cycle frame count (1 for 4x4, 2 for 8x8, 4 for 16x16).
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

// AnimatesPosition reports whether the renderer should animate at the current Speed.
func (s *State) AnimatesPosition() bool {
	return s.Speed <= 2
}

// LoopProgress computes a looping (progress, frameIdx) pair from an elapsed wall-clock duration, as if the animation had been playing at Speed 1 from time zero.
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

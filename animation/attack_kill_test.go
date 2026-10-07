package animation

import (
	"testing"

	"github.com/Zebbeni/protozoa/organism"
)

// TestEveryStatusRoutesSomewhere: a Status with no case falls through to
// AnimIdle, which looks like an organism standing still through whatever it
// was doing. Only the statuses that genuinely have no art of their own may
// land there, and they are named here so adding one is a decision.
func TestEveryStatusRoutesSomewhere(t *testing.T) {
	idleIsCorrect := map[organism.Status]bool{
		organism.StatusIdle:     true,
		organism.StatusSpawning: true,
	}
	seen := map[Animation]organism.Status{}
	for s := organism.Status(0); s <= organism.StatusAttackMove; s++ {
		anim := ForStatus(s)
		if anim == AnimIdle && !idleIsCorrect[s] {
			t.Errorf("status %d falls through to AnimIdle; give it a case or list it", s)
		}
		if anim != AnimIdle {
			if other, dup := seen[anim]; dup {
				t.Logf("statuses %d and %d share animation %d", other, s, anim)
			}
			seen[anim] = s
		}
	}
}

// TestAKillRoutesToItsOwnAnimation: a killer that took its victim's cell
// moved, and the plain attack sheet shows a strike that stays put.
func TestAKillRoutesToItsOwnAnimation(t *testing.T) {
	if got := ForStatus(organism.StatusAttackMove); got != AnimAttackMove {
		t.Errorf("StatusAttackMove plays animation %d, want AnimAttackMove (%d)", got, AnimAttackMove)
	}
	if ForStatus(organism.StatusAttacking) == ForStatus(organism.StatusAttackMove) {
		t.Error("an attack and a killing attack play the same animation")
	}
}

// TestAllAnimationsListsEveryAnimation is what the sheet loader walks, so an
// animation missing from it is one whose art is never loaded at all.
func TestAllAnimationsListsEveryAnimation(t *testing.T) {
	seen := map[Animation]bool{}
	for _, a := range AllAnimations {
		if seen[a] {
			t.Errorf("animation %d is listed twice", a)
		}
		seen[a] = true
	}
	for a := AnimIdle; a <= AnimAttackMove; a++ {
		if !seen[a] {
			t.Errorf("animation %d is not in AllAnimations, so its sheets never load", a)
		}
	}
}

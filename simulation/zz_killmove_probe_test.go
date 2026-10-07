package simulation

import (
	"os"
	"testing"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/organism"
	"github.com/Zebbeni/protozoa/utils"
)

// KILLMOVE_PROBE=1 go test ./simulation/ -run TestKillMoveProbe -v
//
// Records where a killer stood, where its victim stood and where the killer
// ended up, so a report of it landing in the wrong cell can be checked
// against what the simulation actually does.
func TestKillMoveProbe(t *testing.T) {
	if os.Getenv("KILLMOVE_PROBE") == "" {
		t.Skip("set KILLMOVE_PROBE=1")
	}
	loadDefaultGlobals(t)
	g := config.GetCurrentGlobals()
	g.AttackHealthGain = 0.5
	config.SetGlobals(g)

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 101, CheckpointInterval: 1 << 30})

	type snap struct {
		loc utils.Point
		dir utils.Point
	}
	ahead, behind, side, elsewhere, total := 0, 0, 0, 0, 0
	keptStatus, movedOn := 0, 0
	for cycle := 0; cycle < 4000 && total < 40; cycle++ {
		// Before the cycle: where is everyone, and who is dying with a killer named?
		was := map[int]snap{}
		for _, o := range sim.organismManager.Organisms() {
			was[o.ID] = snap{loc: o.Location, dir: o.Direction}
		}
		sim.Update()
		// The claim happens inside this Update, so the pairing is read after.
		pending := map[int]utils.Point{}
		for _, o := range sim.organismManager.Organisms() {
			if o.Status == organism.StatusDying && o.Age > 0 {
				for killer, k := range was {
					if k.loc.Add(k.dir) == o.Location && killer != o.ID {
						if ki := sim.organismManager.GetOrganismInfoByID(killer); ki != nil &&
							ki.Status == organism.StatusAttackMove && ki.Location == o.Location {
							pending[killer] = o.Location
						}
					}
				}
			}
		}
		for killer, victimCell := range pending {
			info := sim.organismManager.GetOrganismInfoByID(killer)
			if info == nil {
				continue
			}
			before, ok := was[killer]
			if !ok {
				continue
			}
			total++
			switch {
			case victimCell == before.loc.Add(before.dir):
				ahead++
			case victimCell == before.loc.Sub(before.dir):
				behind++
			case victimCell.Add(utils.Point{}) == victimCell &&
				(victimCell == before.loc.Add(before.dir.Left()) ||
					victimCell == before.loc.Add(before.dir.Right())):
				side++
			default:
				elsewhere++
			}
			if info.Status == organism.StatusAttackMove {
				keptStatus++
			}
			if info.Location != victimCell {
				movedOn++
			}
			if total <= 12 {
				t.Logf("cycle %d killer %d: stood %v facing %v, victim %v, ended %v, status %d (AttackMove is %d)",
					cycle, killer, before.loc, before.dir, victimCell, info.Location,
					info.Status, organism.StatusAttackMove)
			}
		}
	}
	t.Logf("of %d kill-moves, %d still wore StatusAttackMove at the end of the cycle "+
		"(the rest had it overwritten by their own action, so the travel art never plays), "+
		"and %d moved on past the victim cell in the same cycle", total, keptStatus, movedOn)
	t.Logf("victim cell relative to the killer's pre-cycle facing: ahead %d, behind %d, beside %d, elsewhere %d (of %d)",
		ahead, behind, side, elsewhere, total)
}

// TestAKillMoveIsRenderedAsTheMoveItIs plays a real world forward and checks
// what the RENDERER is handed, which is where this mechanic's bugs have
// lived while every manager test passed.
//
// Status picks the animation and the frame is anchored at the organism's
// pre-cycle cell, so a killer whose status was overwritten is drawn with a
// one-cell animation in the cell it just left, one cell behind the ground it
// took.
func TestAKillMoveIsRenderedAsTheMoveItIs(t *testing.T) {
	loadDefaultGlobals(t)
	g := config.GetCurrentGlobals()
	config.SetGlobals(g)

	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 101, CheckpointInterval: 1 << 30})

	checked := 0
	for cycle := 0; cycle < 4000 && checked < 20; cycle++ {
		was := map[int]utils.Point{}
		for _, o := range sim.organismManager.Organisms() {
			was[o.ID] = o.Location
		}
		sim.Update()
		for _, o := range sim.organismManager.Organisms() {
			if o.Status != organism.StatusAttackMove {
				continue
			}
			checked++
			from, ok := was[o.ID]
			if !ok {
				continue
			}
			if from == o.Location {
				t.Fatalf("cycle %d: organism %d wears StatusAttackMove without having moved", cycle, o.ID)
			}
			if anim := animation.ForStatus(o.Status); anim != animation.AnimAttackMove {
				t.Fatalf("cycle %d: killer %d plays animation %d, want AnimAttackMove", cycle, o.ID, anim)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no kill-move happened in 4000 cycles, so nothing was checked")
	}
	t.Logf("checked %d kill-moves", checked)
}

// TestAKillerMovesInTheCycleItKills: the body keeps its place on screen for
// a cycle so its death animation plays, and that is meant to be visual only.
// Before this it was not — the claim ran from finalizeDeaths at the top of
// the NEXT cycle, so the killer waited a cycle for ground it had won.
//
// Quantified over dying BODIES, not over StatusAttackMove: an attack into
// open water now carries its attacker forward and wears the same status, so
// that status no longer implies a kill. The property here is that the only
// organism ever sharing a cell with a body is the killer that took it, in
// the same cycle.
func TestAKillerMovesInTheCycleItKills(t *testing.T) {
	loadDefaultGlobals(t)
	sim := NewSimulation(&config.Options{IsHeadless: true, Seed: 101, CheckpointInterval: 1 << 30})

	shared := 0
	for cycle := 0; cycle < 4000 && shared < 20; cycle++ {
		sim.Update()
		at := map[utils.Point][]*organism.Organism{}
		for _, o := range sim.organismManager.Organisms() {
			at[o.Location] = append(at[o.Location], o)
		}
		for cell, here := range at {
			if len(here) < 2 {
				continue
			}
			var body, other *organism.Organism
			for _, o := range here {
				if o.Status == organism.StatusDying {
					body = o
				} else {
					other = o
				}
			}
			if body == nil || other == nil {
				t.Fatalf("cycle %d: %d organisms share %v and they are not one body and one killer",
					cycle, len(here), cell)
			}
			if len(here) > 2 {
				t.Fatalf("cycle %d: %d organisms share %v, want at most a body and its killer",
					cycle, len(here), cell)
			}
			shared++
			if other.Status != organism.StatusAttackMove {
				t.Fatalf("cycle %d: organism %d shares %v with a dying body on status %d, "+
					"want StatusAttackMove (%d) — only a killer taking the cell should be there",
					cycle, other.ID, cell, other.Status, organism.StatusAttackMove)
			}
		}
	}
	if shared == 0 {
		t.Fatal("no killer ever shared a cell with the body it made, so nothing was checked")
	}
	t.Logf("checked %d body-and-killer cells", shared)
}

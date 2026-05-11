// Package physiology defines the evolved physical features that organisms
// can inherit, and the decision-tree actions and conditions that each
// feature unlocks for selection during mutation.
//
// This is the data model only — nothing here is wired into mutation,
// snapshots, organism behaviour, or rendering yet. Subsequent slices
// thread it through the rest of the codebase.
package physiology

import (
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/decision"
)

// Feature identifies one evolved physical feature. Features are organised
// into four modality trees (see Tree). Each Spec declares its single
// Parent feature; the tree shape is implicit in those parent links.
// Mutual exclusion between sibling branches (e.g. Cilia vs Stinger)
// falls out for free: an organism can only gain features that are
// children of its deepest-held feature in the relevant tree, so once
// it's chosen one branch the other is unreachable.
type Feature int

// FeatNone is the sentinel returned for "no feature here" — used as
// the Parent of tree roots and as the Deepest result for trees the
// organism hasn't entered.
const FeatNone Feature = -1

const (
	// Flagellae tree — locomotion and physical contact.
	FeatFlagellae Feature = iota // base: turning
	FeatCilia                    // advanced: forward movement
	FeatStinger                  // advanced: stinging attack

	// Sensors tree — environmental awareness.
	FeatAntennae // base: nearby food / organism perception
	FeatFeelers  // advanced: organism size, walls
	FeatTasters  // advanced: pH gradient awareness

	// Defense tree — passive protection plus an active defensive action.
	FeatShell      // base: damage reduction + Hunker
	FeatSpikes     // advanced: reflect damage + Flare
	FeatCamouflage // advanced: harder to sense + Hide

	// Teeth tree — directly altering food/walls/other organisms.
	FeatTeeth // base: eating
	FeatFangs // advanced: attacking
	FeatTusks // advanced: wall-digging
)

// Tree identifies one of the four modality trees a feature belongs to.
type Tree int

const (
	TreeFlagellae Tree = iota
	TreeSensors
	TreeDefense
	TreeTeeth
)

// AllTrees is the canonical iteration order over modality trees.
// Used wherever per-tree logic needs deterministic ordering — most
// importantly Set.Eligible, which feeds the random feature pick during
// mutation and therefore must replay identically.
var AllTrees = []Tree{TreeFlagellae, TreeSensors, TreeDefense, TreeTeeth}

var treeNames = map[Tree]string{
	TreeFlagellae: "Flagellae",
	TreeSensors:   "Sensors",
	TreeDefense:   "Defense",
	TreeTeeth:     "Teeth",
}

// Name returns the human-readable name of the modality tree, used
// for UI labels.
func (t Tree) Name() string { return treeNames[t] }

// All is the canonical declaration order of features. Iteration over
// this slice (rather than the Specs map) keeps allowed-pool ordering
// deterministic — important because random picks from the pool drive
// mutation, and mutation must replay identically across runs.
var All = []Feature{
	FeatFlagellae, FeatCilia, FeatStinger,
	FeatAntennae, FeatFeelers, FeatTasters,
	FeatShell, FeatSpikes, FeatCamouflage,
	FeatTeeth, FeatFangs, FeatTusks,
}

// Set is a bitmask of features an organism currently has. The empty Set
// represents a base chemosynthesizer with no evolved features.
type Set uint64

// Has reports whether the feature is present in the set.
func (s Set) Has(f Feature) bool { return s&(1<<f) != 0 }

// With returns a copy of the set with the feature added.
func (s Set) With(f Feature) Set { return s | (1 << f) }

// Without returns a copy of the set with the feature removed. Provided
// for completeness; the current design treats features as additive
// (gained only), so callers should rarely need this.
func (s Set) Without(f Feature) Set { return s &^ (1 << f) }

// Tradeoffs describes the cumulative passive effect of a feature on the
// organism's stats. Multipliers are 1.0 for "no effect"; additive deltas
// are 0.0. Only the passive costs/benefits live here — the active
// effects of trait-unlocked actions (Hunker, Flare, Hide, etc.) are
// applied at action-resolution time, not derived from this struct.
//
// Slice 1 declares these fields; later slices wire them into the
// chemosynthesis, movement, and combat code paths.
type Tradeoffs struct {
	ChemoEfficiencyMult   float64 // multiplies health gained from chemosynthesis
	MoveCostMult          float64 // multiplies the health cost of moving
	AttackDamageTakenMult float64 // multiplies damage received from attacks
	AttackDamageDealtMult float64 // multiplies damage dealt by ActAttack
	SpawnHealthMult       float64 // multiplies the health a child spawns with
	PerceivedSizeAdd      float64 // added to size when other organisms compare
}

// noEffect returns a Tradeoffs struct with all multipliers at 1.0 and
// additive deltas at 0.0 — i.e. a feature that imposes no passive
// stat change.
func noEffect() Tradeoffs {
	return Tradeoffs{
		ChemoEfficiencyMult:   1.0,
		MoveCostMult:          1.0,
		AttackDamageTakenMult: 1.0,
		AttackDamageDealtMult: 1.0,
		SpawnHealthMult:       1.0,
	}
}

// Spec describes one feature: which modality tree it lives in, the
// Parent feature it descends from, and what decision-tree actions
// and conditions it adds to the organism's allowed pool.
//
// Tree roots set Parent = FeatNone. All other features list their
// immediate ancestor; the children-of relation is computed at init
// from those Parent links. Mutual exclusion between sibling branches
// is implicit — Eligible only ever exposes the children of an
// organism's current deepest-held feature in each tree, so once
// either Cilia or Stinger has been gained, the other is unreachable.
//
// Tradeoffs are NOT stored on Spec — they live in config so the user
// can tune every per-feature penalty / benefit from default.json or
// the in-game settings editor. See tradeoffsFor() below for the
// feature → Tradeoffs lookup.
type Spec struct {
	Name              string
	Tree              Tree
	Parent            Feature
	UnlocksActions    []decision.Action
	UnlocksConditions []decision.Condition
}

// Specs is the registry of all features — names, tree placement,
// parent linkage, and the actions/conditions each unlocks. Passive
// tradeoff values live in config (see tradeoffsFor) so they're
// user-tunable from default.json or the in-game settings editor.
var Specs = map[Feature]Spec{
	// --- Flagellae tree ---
	FeatFlagellae: {
		Name: "Flagellae", Tree: TreeFlagellae, Parent: FeatNone,
		UnlocksActions: []decision.Action{
			decision.ActTurnLeft, decision.ActTurnRight,
		},
	},
	FeatCilia: {
		Name: "Cilia", Tree: TreeFlagellae, Parent: FeatFlagellae,
		UnlocksActions:    []decision.Action{decision.ActMove},
		UnlocksConditions: []decision.Condition{decision.CanMove},
	},
	FeatStinger: {
		Name: "Stinger", Tree: TreeFlagellae, Parent: FeatFlagellae,
		UnlocksActions: []decision.Action{decision.ActSting},
	},

	// --- Sensors tree ---
	FeatAntennae: {
		Name: "Antennae", Tree: TreeSensors, Parent: FeatNone,
		UnlocksConditions: []decision.Condition{
			decision.IsFoodAhead, decision.IsFoodLeft, decision.IsFoodRight,
			decision.IsOrganismAhead, decision.IsOrganismLeft, decision.IsOrganismRight,
		},
	},
	FeatFeelers: {
		Name: "Feelers", Tree: TreeSensors, Parent: FeatAntennae,
		// Feelers unlocks size-comparison and wall-detection. The
		// three IsWall* conditions sense burrowed terrain in the
		// organism's three forward-facing cardinal directions —
		// useful for nest-builders and pack hunters working around
		// obstacles.
		UnlocksConditions: []decision.Condition{
			decision.IsBiggerOrganismAhead,
			decision.IsWallAhead,
			decision.IsWallLeft,
			decision.IsWallRight,
		},
	},
	FeatTasters: {
		Name: "Tasters", Tree: TreeSensors, Parent: FeatAntennae,
		// IsHealthyPhHere is a self-state check (the cell the organism
		// already occupies) and lives in the base condition pool.
		// Tasters specifically unlocks comparisons between the current
		// cell and a neighbour — that's the "extra information" payoff.
		UnlocksConditions: []decision.Condition{
			decision.IsHealthierPhAhead,
		},
	},

	// --- Defense tree ---
	FeatShell: {
		Name: "Shell", Tree: TreeDefense, Parent: FeatNone,
		UnlocksActions: []decision.Action{decision.ActHunker},
	},
	FeatSpikes: {
		Name: "Spikes", Tree: TreeDefense, Parent: FeatShell,
		UnlocksActions: []decision.Action{decision.ActFlare},
	},
	FeatCamouflage: {
		Name: "Camouflage", Tree: TreeDefense, Parent: FeatShell,
		UnlocksActions: []decision.Action{decision.ActHide},
	},

	// --- Teeth tree ---
	FeatTeeth: {
		Name: "Teeth", Tree: TreeTeeth, Parent: FeatNone,
		UnlocksActions: []decision.Action{decision.ActEat},
	},
	FeatFangs: {
		Name: "Fangs", Tree: TreeTeeth, Parent: FeatTeeth,
		UnlocksActions: []decision.Action{decision.ActAttack},
	},
	FeatTusks: {
		Name: "Tusks", Tree: TreeTeeth, Parent: FeatTeeth,
		// Tusks unlock both ActDig (remove food/walls ahead) and
		// ActBurrow (add walls left/right). The two actions form
		// complementary niches — diggers carve corridors, burrowers
		// build shelters.
		UnlocksActions: []decision.Action{decision.ActDig, decision.ActBurrow},
	},
}

// tradeoffsFor returns the resolved passive Tradeoffs for a single
// feature, reading the values live from config so the user can tune
// each per-feature penalty / benefit without recompiling. Aspects
// the feature doesn't affect stay at their identity (mult 1.0,
// additive 0).
func tradeoffsFor(f Feature) Tradeoffs {
	out := noEffect()
	switch f {
	case FeatFlagellae:
		out.ChemoEfficiencyMult = config.FlagellaeChemoEfficiencyMult()
	case FeatCilia:
		out.ChemoEfficiencyMult = config.CiliaChemoEfficiencyMult()
		out.MoveCostMult = config.CiliaMoveCostMult()
	case FeatStinger:
		out.ChemoEfficiencyMult = config.StingerChemoEfficiencyMult()
	case FeatAntennae:
		out.ChemoEfficiencyMult = config.AntennaeChemoEfficiencyMult()
	case FeatFeelers:
		out.ChemoEfficiencyMult = config.FeelersChemoEfficiencyMult()
	case FeatTasters:
		out.ChemoEfficiencyMult = config.TastersChemoEfficiencyMult()
	case FeatShell:
		out.ChemoEfficiencyMult = config.ShellChemoEfficiencyMult()
		out.MoveCostMult = config.ShellMoveCostMult()
		out.AttackDamageTakenMult = config.ShellDamageTakenMult()
	case FeatSpikes:
		out.ChemoEfficiencyMult = config.SpikesChemoEfficiencyMult()
		out.MoveCostMult = config.SpikesMoveCostMult()
		out.AttackDamageTakenMult = config.SpikesDamageTakenMult()
		out.AttackDamageDealtMult = config.SpikesDamageDealtMult()
		out.PerceivedSizeAdd = config.SpikesPerceivedSizeAdd()
	case FeatCamouflage:
		out.ChemoEfficiencyMult = config.CamouflageChemoEfficiencyMult()
		out.MoveCostMult = config.CamouflageMoveCostMult()
		out.AttackDamageTakenMult = config.CamouflageDamageTakenMult()
	case FeatTeeth:
		out.ChemoEfficiencyMult = config.TeethChemoEfficiencyMult()
	case FeatFangs:
		out.ChemoEfficiencyMult = config.FangsChemoEfficiencyMult()
		out.AttackDamageDealtMult = config.FangsDamageDealtMult()
	case FeatTusks:
		out.ChemoEfficiencyMult = config.TusksChemoEfficiencyMult()
		out.MoveCostMult = config.TusksMoveCostMult()
	}
	return out
}

// Always-available actions, regardless of feature set. A base
// chemosynthesizer with no features still has these in its allowed
// mutation pool.
var baseActions = []decision.Action{
	decision.ActChemosynthesis,
	decision.ActIdle,
}

// Always-available conditions: self-state checks that don't require a
// sense organ — the organism can introspect its own health, age, and
// the pH of the cell it sits in. Environment-perception conditions
// (food/organism awareness, pH gradient comparisons with adjacent
// cells) are gated by the Sensors tree.
var baseConditions = []decision.Condition{
	decision.IsHealthAboveFiftyPercent,
	decision.IsHealthyPhHere,
	decision.IsAgeMultipleOfTwo,
	decision.IsAgeMultipleOfTen,
}

// AllowedActions returns the deterministic-ordered union of base
// actions and the actions unlocked by every feature present in s.
// The result is the random-pick pool decision-tree mutation should
// draw from for this organism.
func (s Set) AllowedActions() []decision.Action {
	out := make([]decision.Action, 0, len(baseActions)+8)
	out = append(out, baseActions...)
	for _, f := range All {
		if !s.Has(f) {
			continue
		}
		out = append(out, Specs[f].UnlocksActions...)
	}
	return out
}

// AllowedConditions mirrors AllowedActions for the condition pool.
func (s Set) AllowedConditions() []decision.Condition {
	out := make([]decision.Condition, 0, len(baseConditions)+8)
	out = append(out, baseConditions...)
	for _, f := range All {
		if !s.Has(f) {
			continue
		}
		out = append(out, Specs[f].UnlocksConditions...)
	}
	return out
}

// childrenOf maps each feature to its directly-declared children, in
// the order declared in All. Built once at init from each Spec's
// Parent link so Eligible can do an O(branching-factor) lookup
// instead of scanning Specs every gain roll.
var childrenOf map[Feature][]Feature

// treeRoots maps each Tree to its root feature (the one whose Parent
// is FeatNone). Built once at init and consulted by Eligible when
// the organism hasn't yet entered that tree.
var treeRoots map[Tree]Feature

// actionRequires / conditionRequires invert the Spec's UnlocksActions /
// UnlocksConditions lists. Used by Set.ActionAvailable /
// ConditionAvailable so the action-resolution and condition-evaluation
// code paths can degrade gracefully when an organism has lost the
// feature that gated a node still present in its inherited decision
// tree (the "junk DNA" case).
//
// Actions / conditions not present in either inverse map are treated
// as base — always available regardless of feature set. That covers
// the always-on actions (Chemosynthesis, Idle, Spawn) and self-state
// conditions (health, age, IsHealthyPhHere).
var (
	actionRequires    map[decision.Action]Feature
	conditionRequires map[decision.Condition]Feature
)

func init() {
	childrenOf = make(map[Feature][]Feature)
	treeRoots = make(map[Tree]Feature)
	actionRequires = make(map[decision.Action]Feature)
	conditionRequires = make(map[decision.Condition]Feature)
	for _, f := range All {
		spec := Specs[f]
		if spec.Parent == FeatNone {
			treeRoots[spec.Tree] = f
		} else {
			childrenOf[spec.Parent] = append(childrenOf[spec.Parent], f)
		}
		for _, a := range spec.UnlocksActions {
			actionRequires[a] = f
		}
		for _, c := range spec.UnlocksConditions {
			conditionRequires[c] = f
		}
	}
}

// ActionAvailable reports whether this set's organism can resolve the
// action — true for base actions and for feature-gated actions whose
// gating feature is held. Used by the action-resolution code to fall
// back to ActIdle when an inherited decision tree picks an action the
// organism no longer has the physiology for.
func (s Set) ActionAvailable(a decision.Action) bool {
	f, gated := actionRequires[a]
	if !gated {
		return true
	}
	return s.Has(f)
}

// ConditionAvailable mirrors ActionAvailable for conditions. A gated
// condition that the organism can't currently sense is treated as
// false at evaluation time — same fallback strategy as actions.
func (s Set) ConditionAvailable(c decision.Condition) bool {
	f, gated := conditionRequires[c]
	if !gated {
		return true
	}
	return s.Has(f)
}

// Path returns the root-to-leaf list of features the set holds in
// tree, or nil if the set holds none. Useful for UI rendering of a
// "what has this organism evolved" path.
func (s Set) Path(tree Tree) []Feature {
	deepest := s.Deepest(tree)
	if deepest == FeatNone {
		return nil
	}
	// Walk up from deepest to root, prepending. The depth here is
	// bounded by the tree's height — currently 2, with room to grow.
	var path []Feature
	for cur := deepest; cur != FeatNone; cur = Specs[cur].Parent {
		path = append([]Feature{cur}, path...)
	}
	return path
}

// Deepest returns the deepest feature the set holds in tree, or
// FeatNone if it holds none. Walks down from the tree's root by
// following whichever child is also in the set; sibling mutual
// exclusion (enforced by Eligible at gain time) guarantees at most
// one child per level is held, so the walk is unambiguous.
func (s Set) Deepest(tree Tree) Feature {
	root, ok := treeRoots[tree]
	if !ok || !s.Has(root) {
		return FeatNone
	}
	cur := root
	for {
		next := FeatNone
		for _, child := range childrenOf[cur] {
			if s.Has(child) {
				next = child
				break
			}
		}
		if next == FeatNone {
			return cur
		}
		cur = next
	}
}

// CanGain reports whether the feature could be added to the set: it is
// not already present and is currently exposed by Eligible — i.e.
// it's the next step along the branch the organism has already
// committed to in its tree, or the root of a tree the organism hasn't
// entered yet.
func (s Set) CanGain(f Feature) bool {
	if s.Has(f) {
		return false
	}
	for _, e := range s.Eligible() {
		if e == f {
			return true
		}
	}
	return false
}

// Eligible returns the deterministic-ordered list of features the set
// could currently gain. For each tree in AllTrees order: if the set
// hasn't entered the tree, the root is offered; otherwise the
// children of its deepest-held feature are offered (an empty slice
// means the organism has reached a leaf and can't advance further in
// that tree until deeper branches are declared).
func (s Set) Eligible() []Feature {
	out := make([]Feature, 0, len(AllTrees))
	for _, tree := range AllTrees {
		deepest := s.Deepest(tree)
		if deepest == FeatNone {
			out = append(out, treeRoots[tree])
			continue
		}
		out = append(out, childrenOf[deepest]...)
	}
	return out
}

// Loseable returns the deterministic-ordered list of features the set
// could currently lose without orphaning a descendant — exactly the
// deepest held feature in each non-empty tree. Removing one of these
// via Without preserves the invariant "every held feature has every
// ancestor held" because the leaf has no held descendants by
// definition. Used by the loss-mutation path so lineages can shorten
// a branch and re-grow it down a sibling, restoring physiological
// diversity to a population that would otherwise saturate at the
// leaves of every tree.
func (s Set) Loseable() []Feature {
	out := make([]Feature, 0, len(AllTrees))
	for _, tree := range AllTrees {
		if d := s.Deepest(tree); d != FeatNone {
			out = append(out, d)
		}
	}
	return out
}

// Combined returns the resolved passive Tradeoffs for s. For every
// tree only the deepest-held feature contributes — earlier ancestors
// in the same tree are the lineage that led there, not stacking
// layers. Cilia is "a cilia body", not "Flagellae plus Cilia"; Fangs
// is "a fanged body", not "Teeth plus Fangs"; etc.
//
// Actions and Conditions still inherit through the full tree path —
// a Cilia organism can still Turn because Flagellae unlocked it.
// AllowedActions / AllowedConditions handle that separately by
// walking every held feature in the bitmask.
//
// Per-feature Tradeoffs values come from config (see tradeoffsFor)
// so every penalty / benefit is user-tunable without recompiling.
func (s Set) Combined() Tradeoffs {
	out := noEffect()
	for _, tree := range AllTrees {
		deepest := s.Deepest(tree)
		if deepest == FeatNone {
			continue
		}
		t := tradeoffsFor(deepest)
		out.ChemoEfficiencyMult *= t.ChemoEfficiencyMult
		out.MoveCostMult *= t.MoveCostMult
		out.AttackDamageTakenMult *= t.AttackDamageTakenMult
		out.AttackDamageDealtMult *= t.AttackDamageDealtMult
		out.SpawnHealthMult *= t.SpawnHealthMult
		out.PerceivedSizeAdd += t.PerceivedSizeAdd
	}
	return out
}

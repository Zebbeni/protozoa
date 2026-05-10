package organism

import (
	"image/color"
	"math"
	"sync"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

type Organism struct {
	ID                   int
	Age                  int
	Health               float64
	Size                 float64
	Children             int
	TraveledDist         int
	CyclesSinceLastSpawn int
	Location             utils.Point
	Direction            utils.Point
	OriginalAncestorID   int
	TreeNode             *DescendantNode

	// AttackTotal counts every cycle the organism resolved an attack
	// action, regardless of whether anything was in front of it.
	// AttackHits counts the subset where there was an organism in the
	// target cell at attack time. The pair drives the "MOST AGGRESSIVE"
	// highlight (sorted by AttackHits) and the "Attacks: hits/total"
	// stat in the panel.
	AttackTotal int
	AttackHits  int

	// PhPositive / PhNegative are the lifetime cumulative magnitudes
	// of pH the organism has pushed *up* (eating) and *down* (chemo)
	// at its surrounding cells. Both values are non-negative; the
	// renderer derives a colour from max/min and which one is larger
	// to tint organisms by their net behavioural pH effect.
	PhPositive float64
	PhNegative float64

	traits Traits

	decisionTree *d.Tree
	action       d.Action
	// ChemoFailed is set each cycle the organism performs
	// chemosynthesis: true when the pH at its location fell outside
	// its tolerance range (no health gained), false when it succeeded.
	// Stale between non-chemo cycles; renderer only consults it when
	// action == ActChemosynthesis.
	ChemoFailed bool
	// EatFailed is set each cycle the organism performs ActEat: true
	// when the cell ahead held no food (the eat attempt cost health
	// but produced none), false when food was actually consumed.
	// Stale between non-eat cycles; renderer only consults it when
	// action == ActEat.
	EatFailed bool
	// BornThisCycle is set to true in NewChild so the animation layer
	// can build a birth Frame (2-cell move from the parent's cell into
	// the child's cell). Cleared by UpdateStats at the start of the
	// next cycle. Explicit signal instead of inferring from preSnap
	// membership — preSnap can be stale after a seek, which would
	// otherwise cause surviving organisms to be rendered as newborns
	// and land one cell off from their true location.
	BornThisCycle bool

	lookupAPI LookupAPI

	mutex sync.Mutex
}

// NewRandom initializes organism at with random grid location and direction
func NewRandom(rng *simrand.RNG, id int, point utils.Point, api LookupAPI) *Organism {
	traits := newRandomTraits(rng)
	allowedActions := traits.Features.AllowedActions()
	allowedConditions := traits.Features.AllowedConditions()
	decisionTree := d.TreeFromAction(d.ActChemosynthesis)
	for mutations := 0; mutations < c.InitialDecisionTreeMutations(); mutations++ {
		decisionTree = d.MutateTree(rng, decisionTree, allowedActions, allowedConditions)
	}
	organism := Organism{
		ID:                   id,
		Age:                  0,
		Health:               traits.SpawnHealth,
		Size:                 traits.SpawnHealth,
		Children:             0,
		CyclesSinceLastSpawn: 0,
		Location:             point,
		Direction:            utils.GetRandomDirection(rng),
		OriginalAncestorID:   id,

		traits:       traits,
		decisionTree: decisionTree,
		action:       d.ActChemosynthesis,

		lookupAPI: api,
	}
	return &organism
}

// NewChild initializes and returns a new organism with a copied TreeLibrary
// from its parent. direction is the unit vector from parent.Location to
// point (i.e. "away from parent"): children are spawned facing outward
// so the birth animation can render as a standard move from parent cell
// into child cell (FromLocation = point.Sub(direction)).
func (o *Organism) NewChild(rng *simrand.RNG, id int, point utils.Point, direction utils.Point, api LookupAPI) *Organism {
	traits := o.traits.copyMutated(rng)
	inheritedTree := o.GetDecisionTreeCopy()
	if rng.Float64() < c.ChanceToMutateDecisionTree() {
		inheritedTree = d.MutateTree(rng, inheritedTree,
			traits.Features.AllowedActions(),
			traits.Features.AllowedConditions(),
		)
	}
	// SpawnHealthMult is the parent's investment-in-offspring tradeoff:
	// scaling the parent's InitialHealth before assigning it to the
	// child means features that buff or nerf reproduction take effect
	// at the spawn site without needing to rewrite the child's
	// SpawnHealth trait (which still drift-mutates independently and
	// becomes the basis for the grandchild).
	startHealth := o.InitialHealth() * o.Tradeoffs().SpawnHealthMult
	organism := Organism{
		ID:                   id,
		Age:                  0,
		Health:               startHealth,
		Size:                 startHealth,
		Children:             0,
		CyclesSinceLastSpawn: 0,
		Location:             point,
		Direction:            direction,
		OriginalAncestorID:   o.OriginalAncestorID,

		traits:        traits,
		decisionTree:  inheritedTree,
		action:        d.ActChemosynthesis,
		BornThisCycle: true,

		lookupAPI: api,
	}
	return &organism
}

// Restore creates an organism from fully specified state (for checkpoint restore).
func Restore(id, age int, health, size float64, children, traveledDist, cyclesSinceLastSpawn int,
	location, direction utils.Point, ancestorID int,
	traits Traits, tree *d.Tree, action d.Action,
	attackTotal, attackHits int, phPositive, phNegative float64, api LookupAPI) *Organism {
	return &Organism{
		ID:                   id,
		Age:                  age,
		Health:               health,
		Size:                 size,
		Children:             children,
		TraveledDist:         traveledDist,
		CyclesSinceLastSpawn: cyclesSinceLastSpawn,
		Location:             location,
		Direction:            direction,
		OriginalAncestorID:   ancestorID,
		traits:               traits,
		decisionTree:         tree,
		action:               action,
		AttackTotal:          attackTotal,
		AttackHits:           attackHits,
		PhPositive:           phPositive,
		PhNegative:           phNegative,
		lookupAPI:            api,
	}
}

func (o *Organism) Info() *Info {
	return &Info{
		ID:            o.ID,
		Health:        o.Health,
		Location:      o.Location,
		Direction:     o.Direction,
		Size:          o.Size,
		Action:        o.action,
		AncestorID:    o.OriginalAncestorID,
		Color:         o.traits.OrganismColor,
		Age:           o.Age,
		Children:      o.Children,
		TraveledDist:  o.TraveledDist,
		PhPositive:    o.PhPositive,
		PhNegative:    o.PhNegative,
		ChemoFailed:   o.ChemoFailed,
		EatFailed:     o.EatFailed,
		BornThisCycle: o.BornThisCycle,
		AttackTotal:   o.AttackTotal,
		AttackHits:    o.AttackHits,
	}
}

// UpdateStats runs on each cycle and updates Age, CyclesSinceLastSpawn, etc.
// Also calculates the change in health since the last cycle and applies this
// to the success metrics of the last-used decision tree.
func (o *Organism) UpdateStats() {
	o.Age++
	o.CyclesSinceLastSpawn++
	o.decisionTree.ResetUsedLastCycle()
	// Birth flag lives for exactly the spawn cycle (set in NewChild,
	// consumed by the animation layer in AfterUpdate). Clear it at the
	// start of every subsequent cycle so the birth animation isn't
	// replayed. Newborns aren't in the organism iteration on their
	// spawn cycle, so they don't see this until the cycle after.
	o.BornThisCycle = false
}

// UpdateAction picks the decision tree's action and stores it on
// the organism. Spawn promotion happens at the manager level (which
// can verify against the live grid that a child can fit) — keeping
// it out of here means a "wants to spawn but trapped" organism
// resolves through the tree's actual choice, with the request map
// and side effects matching what will run.
func (o *Organism) UpdateAction() {
	o.action = o.chooseAction(o.decisionTree.Node)
}

// PromoteToSpawn is called by the manager during the decide phase
// when an organism is eligible to spawn AND the grid has room for a
// child. Sets the action to ActSpawn and resets the spawn cooldown.
// Trapped organisms (no empty neighbour) skip this call and keep
// their tree-picked action.
func (o *Organism) PromoteToSpawn() {
	o.action = d.ActSpawn
	o.CyclesSinceLastSpawn = 0
}

// ShouldSpawn reports whether the organism currently meets the
// per-organism preconditions for reproducing: enough cycles since
// its last spawn, enough health to bear the spawn cost, and
// global-population headroom. Doesn't check whether the grid has
// room for a child; the manager pairs this with getChildSpawnLocation.
func (o *Organism) ShouldSpawn() bool { return o.shouldSpawn() }

func (o *Organism) shouldSpawn() bool {
	if o.CyclesSinceLastSpawn < o.MinCyclesBetweenSpawns() {
		return false
	}
	if o.Health < o.MinHealthToSpawn() {
		return false
	}
	if o.lookupAPI.OrganismCount() >= c.MaxOrganisms() {
		return false
	}
	return true
}

// chooseAction walks through nodes of an organism's decision tree, eventually
// returning the chosen action
//
// As chooseAction walks through nodes, it also sets UsedLastCycle=true, allowing
// the organism to attribute success or failure to the previously-chosen path
func (o *Organism) chooseAction(node *d.Node) d.Action {
	node.UsedLastCycle = true
	node.WasTravelled = true
	if node.IsAction() {
		return node.NodeType.(d.Action)
	}
	if o.isConditionTrue(node.NodeType) {
		return o.chooseAction(node.YesNode)
	}
	return o.chooseAction(node.NoNode)
}

func (o *Organism) isConditionTrue(cond interface{}) bool {
	// Junk-DNA fallback for conditions: a tree node may probe a
	// condition the organism no longer has the sensor for (e.g. an
	// IsBiggerOrganismAhead test inherited after losing FeatFeelers).
	// Treat those as false so the No branch is taken — re-gaining the
	// feature reactivates the test naturally.
	if c, ok := cond.(d.Condition); ok && !o.traits.Features.ConditionAvailable(c) {
		return false
	}
	switch cond {
	case d.CanMove:
		return o.canMove()
	case d.IsFoodAhead:
		return o.isFoodAhead()
	case d.IsFoodLeft:
		return o.isFoodLeft()
	case d.IsFoodRight:
		return o.isFoodRight()
	case d.IsOrganismAhead:
		return o.isOrganismAhead()
	case d.IsBiggerOrganismAhead:
		return o.isBiggerOrganismAhead()
	case d.IsOrganismLeft:
		return o.isOrganismLeft()
	case d.IsOrganismRight:
		return o.isOrganismRight()
	//case d.IsRandomFiftyPercent:
	//	return rand.Float32() < 0.5
	case d.IsHealthAboveFiftyPercent:
		return o.Health > o.Size*0.5
	case d.IsHealthyPhHere:
		return o.isHealthyPhHere()
	case d.IsHealthierPhAhead:
		return o.isHealthierPhAhead()
	case d.IsAgeMultipleOfTwo:
		return o.isAgeMultipleOfTwo()
	case d.IsAgeMultipleOfTen:
		return o.isAgeMultipleOfTen()
	}
	return false
}

// X returns the x component of the organism's location Point
func (o *Organism) X() int { return o.Location.X }

// Y returns the y component of the organism's location Point
func (o *Organism) Y() int { return o.Location.Y }

// GetDecisionTreeCopy returns a copy of an organism's currently-used decision tree
func (o *Organism) GetDecisionTreeCopy() *d.Tree {
	return o.decisionTree.CopyTree()
}

// RebuildDecisionPath walks chooseAction on the current decision tree
// to set UsedLastCycle and WasTravelled along the path that current
// state would take. Called after a snapshot restore — the serialized
// tree string drops the flags, so without this the panel shows no
// highlights and no "◀◀" markers until the next sim Update cycles.
// Doesn't change o.action (the saved action stays put).
func (o *Organism) RebuildDecisionPath() {
	if o.decisionTree == nil || o.decisionTree.Node == nil {
		return
	}
	o.chooseAction(o.decisionTree.Node)
}

// Traits returns an organism's traits
func (o Organism) Traits() Traits    { return o.traits }
func (o *Organism) TraitsRef() *Traits { return &o.traits }

// Tradeoffs returns the organism's combined passive Tradeoffs from
// every feature it currently holds. Computed on demand from the
// feature bitmask; cheap (12-feature iteration) and called from a few
// per-cycle hot paths (chemo, move, attack, size-compare). If
// profiling ever shows this in the hot loop, cache on the organism
// at construction — features are immutable for a given organism.
func (o *Organism) Tradeoffs() physiology.Tradeoffs {
	return o.traits.Features.Combined()
}

// InitialHealth returns the health an organism and its children start life with
func (o Organism) InitialHealth() float64 { return o.traits.SpawnHealth }

// HealthCostToReproduce returns the health to lose upon spawning a child
func (o Organism) HealthCostToReproduce() float64 {
	return o.traits.SpawnHealth*-1.0 + (o.Size * c.HealthChangeFromSpawning())
}

// MinHealthToSpawn returns the minimum health required for an organism to spawn a child
func (o Organism) MinHealthToSpawn() float64 { return o.traits.MinHealthToSpawn }

// MinCyclesBetweenSpawns returns the minimum number of cycles needed for an
// organism to spawn
func (o Organism) MinCyclesBetweenSpawns() int { return o.traits.MinCyclesBetweenSpawns }

// Action returns the Organism's currently-chosen action
func (o Organism) Action() d.Action { return o.action }

// SetAction overwrites the organism's currently-chosen action. Used by
// the reverse-delta apply path to roll the action back to its
// pre-cycle value during step-back.
func (o *Organism) SetAction(a d.Action) { o.action = a }

// Color returns an organism's color
func (o Organism) Color() color.Color { return o.traits.OrganismColor }

// MaxSize returns an organism's maximum size
func (o *Organism) MaxSize() float64 { return o.traits.MaxSize }

func (o *Organism) setDecisionTree(decisionTree *d.Tree) {
	if o.decisionTree != nil {
		o.decisionTree.SetUsedInCurrentTree(false)
	}
	o.decisionTree = decisionTree
	o.decisionTree.SetUsedInCurrentTree(true)
}

// ApplyHealthChange adds a value to the organism's health, bounded by 0 and MaxSize
// If new health is greater than the organism's Size, this is updated too.
func (o *Organism) ApplyHealthChange(change float64) {
	o.Health += change
	if o.Health > o.Size {
		// When health increase causes size to increase, increase slowly, not all at once.
		difference := o.Health - o.Size
		o.Size = math.Min(o.Size+(difference*c.GrowthFactor()), o.traits.MaxSize)
	}
	o.Health = math.Min(o.Health, o.Size)
}

func (o *Organism) isFoodAhead() bool {
	return o.isFoodAtPoint(o.Location.Add(o.Direction))
}

func (o *Organism) isFoodLeft() bool {
	return o.isFoodAtPoint(o.Location.Add(o.Direction.Left()))
}

func (o *Organism) isFoodRight() bool {
	return o.isFoodAtPoint(o.Location.Add(o.Direction.Right()))
}

func (o *Organism) isFoodAtPoint(point utils.Point) bool {
	return o.lookupAPI.CheckFoodAtPoint(point, func(f *food.Item, exists bool) bool {
		return exists
	})
}

func (o *Organism) isOrganismAhead() bool {
	return o.isOrganismAtPoint(o.Location.Add(o.Direction))
}

func (o *Organism) isWallAhead() bool {
	return o.isWallAtPoint(o.Location.Add(o.Direction))
}

func (o *Organism) isBiggerOrganismAhead() bool {
	return o.isBiggerOrganismAtPoint(o.Location.Add(o.Direction))
}

func (o *Organism) isOrganismLeft() bool {
	return o.isOrganismAtPoint(o.Location.Add(o.Direction.Left()))
}

func (o *Organism) isOrganismRight() bool {
	return o.isOrganismAtPoint(o.Location.Add(o.Direction.Right()))
}

func (o *Organism) isHealthyPhHere() bool {
	return o.isPhHealthyAtPoint(o.Location, o.Traits().IdealPh, c.PhTolerance())
}

func (o *Organism) isHealthierPhAhead() bool {
	return o.isPhHealthierAtPoint(o.Location, o.Location.Add(o.Direction), o.Traits().IdealPh)
}

func (o *Organism) isAgeMultipleOfTwo() bool {
	return o.Age%2 == 0
}

func (o *Organism) isAgeMultipleOfTen() bool {
	return o.Age%10 == 0
}

func (o *Organism) isBiggerOrganismAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		// Use the OTHER organism's perceived size (Size + Tradeoffs.PerceivedSizeAdd)
		// so features like Spikes can make their bearer look bigger
		// than they are to a sensing neighbour. The observer's own
		// size is the raw value — they know themselves accurately.
		return x != nil && x.Size+x.Tradeoffs().PerceivedSizeAdd > o.Size
	})
}

func (o *Organism) isOrganismAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		return x != nil
	})
}

func (o *Organism) isWallAtPoint(p utils.Point) bool {
	return p.IsWall()
}

func (o *Organism) checkOrganismAtPoint(p utils.Point, checkFunc OrgCheck) bool {
	return o.lookupAPI.CheckOrganismAtPoint(p, checkFunc)
}

func (o *Organism) isPhHealthyAtPoint(p utils.Point, ideal, tolerance float64) bool {
	ph := o.lookupAPI.GetPhAtPoint(p)
	return math.Abs(ph-ideal) < tolerance
}

func (o *Organism) isPhHealthierAtPoint(control, test utils.Point, ideal float64) bool {
	controlPh := o.lookupAPI.GetPhAtPoint(control)
	testPh := o.lookupAPI.GetPhAtPoint(test)
	return math.Abs(testPh-ideal) < math.Abs(controlPh-ideal)
}

func (o *Organism) canMove() bool {
	if o.isWallAhead() {
		return false
	}
	if o.isOrganismAhead() {
		return false
	}
	if o.isFoodAhead() {
		return false
	}
	return true
}

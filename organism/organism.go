package organism

import (
	"image/color"
	"math"
	"sync"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/effects"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/physiology"
	"github.com/Zebbeni/protozoa/simrand"
	"github.com/Zebbeni/protozoa/utils"
)

// Status is the resolved outcome of an organism's most recent cycle.
type Status int

const (
	StatusIdle Status = iota
	StatusChemoSuccess
	StatusChemoFailed
	StatusEatSuccess
	StatusEatFailed
	StatusMoveSuccess
	StatusMoveBlocked
	StatusTurnLeft
	StatusTurnRight
	StatusAttacking
	StatusSpawning
	StatusDigging
	StatusDying // one cycle after lethal damage — AnimDie plays, then finalizeDeaths replaces with food
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

	// AttackTotal counts every cycle the organism resolved an attack action, regardless of whether anything was in front of it.
	AttackTotal int
	AttackHits  int

	// PhPositive / PhNegative are the lifetime cumulative magnitudes of pH the organism has pushed *up* (eating) and *down* (chemo) at its surrounding cells.
	PhPositive float64
	PhNegative float64

	traits Traits

	decisionTree *d.Tree
	action       d.Action
	// Status is the resolved outcome of the most recent apply* pass — see the Status enum.
	Status Status
	// BornThisCycle is set to true in NewChild so the animation layer can build a birth Frame (2-cell move from the parent's cell into the child's cell).
	BornThisCycle bool

	// appearance is derived from the ability scores and the decision tree, both of which are fixed for an organism's lifetime.
	appearance physiology.Appearance

	lookupAPI LookupAPI

	mutex sync.Mutex
}

// Appearance returns the organism's derived look — body silhouette, motor, mouth and sensor overlays.
func (o *Organism) Appearance() physiology.Appearance {
	return o.appearance
}

// NewRandom initializes organism at with random grid location and direction
func NewRandom(rng *simrand.RNG, id int, point utils.Point, api LookupAPI) *Organism {
	traits := newRandomTraits(rng)
	decisionTree := d.TreeFromAction(d.ActChemosynthesis)
	for mutations := 0; mutations < c.InitialDecisionTreeMutations(); mutations++ {
		decisionTree = d.MutateTree(rng, decisionTree)
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
		appearance:   physiology.AppearanceFor(traits.Abilities, decisionTree),

		lookupAPI: api,
	}
	return &organism
}

// NewChild initializes and returns a new organism with a copied TreeLibrary from its parent.
func (o *Organism) NewChild(rng *simrand.RNG, id int, point utils.Point, direction utils.Point, api LookupAPI) *Organism {
	traits := o.traits.copyMutated(rng)
	inheritedTree := o.GetDecisionTreeCopy()
	if rng.Float64() < c.ChanceToMutateDecisionTree() {
		inheritedTree = d.MutateTree(rng, inheritedTree)
	}
	startHealth := o.InitialHealth()
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
		appearance:    physiology.AppearanceFor(traits.Abilities, inheritedTree),
		BornThisCycle: true,

		lookupAPI: api,
	}
	return &organism
}

// Restore creates an organism from fully specified state (for checkpoint restore).
func Restore(id, age int, health, size float64, children, traveledDist, cyclesSinceLastSpawn int,
	location, direction utils.Point, ancestorID int,
	traits Traits, tree *d.Tree, action d.Action, status Status,
	attackTotal, attackHits int, phPositive, phNegative float64,
	api LookupAPI) *Organism {
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
		appearance:           physiology.AppearanceFor(traits.Abilities, tree),
		Status:               status,
		AttackTotal:          attackTotal,
		AttackHits:           attackHits,
		PhPositive:           phPositive,
		PhNegative:           phNegative,
		lookupAPI:            api,
	}
}

func (o *Organism) Info() *Info {
	return &Info{
		ID:              o.ID,
		Health:          o.Health,
		Location:        o.Location,
		Direction:       o.Direction,
		Size:            o.Size,
		Action:          o.action,
		AncestorID:      o.OriginalAncestorID,
		Color:           o.traits.OrganismColor,
		SecondaryColor:  o.traits.SecondaryColor,
		Age:             o.Age,
		Children:        o.Children,
		TraveledDist:    o.TraveledDist,
		PhPositive:      o.PhPositive,
		PhNegative:      o.PhNegative,
		Status:          o.Status,
		BornThisCycle:   o.BornThisCycle,
		AttackTotal:     o.AttackTotal,
		AttackHits:      o.AttackHits,
		IdealPh:         o.traits.IdealPh,
		Abilities:       o.traits.Abilities,
		Appearance:      o.appearance,
		LineageEndCycle: lineageEndCycle(o.TreeNode),
	}
}

// UpdateStats runs on each cycle and updates Age, CyclesSinceLastSpawn, etc. Also calculates the change in health since the last cycle and applies this to the success metrics of the last-used decision tree.
func (o *Organism) UpdateStats() {
	o.Age++
	o.CyclesSinceLastSpawn++
	o.decisionTree.ResetUsedLastCycle()
	// Birth flag lives for exactly the spawn cycle (set in NewChild, consumed by the animation layer in AfterUpdate).
	o.BornThisCycle = false
	// Status is consumed within the cycle that sets it (every apply* handler stamps the outcome).
	o.Status = StatusIdle
}

// UpdateAction picks the decision tree's action and stores it on the organism.
func (o *Organism) UpdateAction() {
	o.action = o.chooseAction(o.decisionTree.Node)
}

// PromoteToSpawn is called by the manager during the decide phase when an organism is eligible to spawn AND the grid has room for a child.
func (o *Organism) PromoteToSpawn() {
	o.action = d.ActSpawn
	o.CyclesSinceLastSpawn = 0
}

// ShouldSpawn reports whether the organism currently meets the per-organism preconditions for reproducing.
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

// The health family's thresholds, as fractions of an organism's size.
const (
	healthyFraction       = 0.5
	veryHealthyFraction   = 0.8
	veryUnhealthyFraction = 0.2
)

func (o *Organism) isConditionTrue(cond interface{}) bool {
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
	case d.IsRelativeAhead:
		return o.isRelativeAhead()
	case d.IsOrganismLeft:
		return o.isOrganismLeft()
	case d.IsOrganismRight:
		return o.isOrganismRight()
	case d.IsHealthy:
		return o.Health > o.Size*healthyFraction
	case d.IsVeryHealthy:
		return o.Health > o.Size*veryHealthyFraction
	case d.IsVeryUnhealthy:
		return o.Health < o.Size*veryUnhealthyFraction
	case d.IsHealthyPhHere:
		return o.isHealthyPhHere()
	case d.CanChemosynthesizeHere:
		return o.canChemosynthesizeHere()
	case d.IsHealthierPhAhead:
		return o.isHealthierPhAhead()
	case d.IsAgeMultipleOfTwo:
		return o.isAgeMultipleOfTwo()
	case d.IsAgeMultipleOfTen:
		return o.isAgeMultipleOfTen()
	case d.IsWallAhead:
		return o.isWallAhead()
	case d.IsWallLeft:
		return o.isWallAtPoint(o.Location.Add(o.Direction.Left()))
	case d.IsWallRight:
		return o.isWallAtPoint(o.Location.Add(o.Direction.Right()))
	case d.IsSomethingLeft:
		return o.isSomethingAtPoint(o.Location.Add(o.Direction.Left()))
	case d.IsSomethingRight:
		return o.isSomethingAtPoint(o.Location.Add(o.Direction.Right()))
	case d.IsBiggerOrganismLeft:
		return o.isBiggerOrganismAtPoint(o.Location.Add(o.Direction.Left()))
	case d.IsBiggerOrganismRight:
		return o.isBiggerOrganismAtPoint(o.Location.Add(o.Direction.Right()))
	case d.IsRelativeLeft:
		return o.isRelativeAtPoint(o.Location.Add(o.Direction.Left()))
	case d.IsRelativeRight:
		return o.isRelativeAtPoint(o.Location.Add(o.Direction.Right()))
	case d.IsPhTooLowHere:
		return o.isPhTooLowHere()
	case d.IsPhTooHighHere:
		return o.isPhTooHighHere()
	case d.IsHealthAboveTwentyPercent:
		return o.Health > o.Size*0.2
	case d.IsMuchBiggerOrganismAhead:
		return o.isMuchBiggerOrganismAtPoint(o.Location.Add(o.Direction))
	case d.IsMuchBiggerOrganismLeft:
		return o.isMuchBiggerOrganismAtPoint(o.Location.Add(o.Direction.Left()))
	case d.IsMuchBiggerOrganismRight:
		return o.isMuchBiggerOrganismAtPoint(o.Location.Add(o.Direction.Right()))
	case d.IsMuchSmallerOrganismAhead:
		return o.isMuchSmallerOrganismAtPoint(o.Location.Add(o.Direction))
	case d.IsMuchSmallerOrganismLeft:
		return o.isMuchSmallerOrganismAtPoint(o.Location.Add(o.Direction.Left()))
	case d.IsMuchSmallerOrganismRight:
		return o.isMuchSmallerOrganismAtPoint(o.Location.Add(o.Direction.Right()))
	case d.IsFoodHere:
		return o.isFoodAtPoint(o.Location)
	case d.IsFoodBuriedHere:
		return o.lookupAPI.GetBuriedFoodAtPoint(o.Location) > 0
	case d.IsMuchFoodHere:
		return o.isMuchFoodAtPoint(o.Location)
	case d.IsMuchFoodBuriedHere:
		return o.isMuchFood(float64(o.lookupAPI.GetBuriedFoodAtPoint(o.Location)))
	}
	return false
}

func (o *Organism) X() int { return o.Location.X }

func (o *Organism) Y() int { return o.Location.Y }

func (o *Organism) GetDecisionTreeCopy() *d.Tree {
	return o.decisionTree.CopyTree()
}

// RebuildDecisionPath walks chooseAction on the current decision tree to set UsedLastCycle and WasTravelled along the path that current state would take.
func (o *Organism) RebuildDecisionPath() {
	if o.decisionTree == nil || o.decisionTree.Node == nil {
		return
	}
	o.chooseAction(o.decisionTree.Node)
}

func (o Organism) Traits() Traits      { return o.traits }
func (o *Organism) TraitsRef() *Traits { return &o.traits }

// Tradeoffs returns the organism's combined passive Tradeoffs from / Abilities returns the organism's ability-score distribution.
func (o *Organism) Abilities() physiology.Scores {
	return o.traits.Abilities
}

func (o Organism) InitialHealth() float64 { return o.traits.SpawnHealth }

func (o Organism) HealthCostToReproduce() float64 {
	return o.traits.SpawnHealth*-1.0 + (o.Size * c.HealthChangeFromSpawning())
}

func (o Organism) MinHealthToSpawn() float64 { return o.traits.MinHealthToSpawn }

// MinCyclesBetweenSpawns returns the minimum number of cycles needed for an organism to spawn
func (o Organism) MinCyclesBetweenSpawns() int { return o.traits.MinCyclesBetweenSpawns }

func (o Organism) Action() d.Action { return o.action }

// SetAction overwrites the organism's currently-chosen action.
func (o *Organism) SetAction(a d.Action) { o.action = a }

func (o Organism) Color() color.Color { return o.traits.OrganismColor }

func (o *Organism) MaxSize() float64 { return o.traits.MaxSize }

func (o *Organism) setDecisionTree(decisionTree *d.Tree) {
	if o.decisionTree != nil {
		o.decisionTree.SetUsedInCurrentTree(false)
	}
	o.decisionTree = decisionTree
	o.decisionTree.SetUsedInCurrentTree(true)
}

// ApplyHealthChange adds a value to the organism's health, bounded by 0 and MaxSize If new health is greater than the organism's Size, this is updated too.
func (o *Organism) ApplyHealthChange(change float64) {
	o.ApplyHealthChangeWithGrowth(change, c.GrowthFactor())
}

// ApplyHealthChangeWithGrowth applies a health change, turning growthFactor of any health above the organism's current size into growth (capped at its max size).
func (o *Organism) ApplyHealthChangeWithGrowth(change, growthFactor float64) {
	o.Health += change
	if o.Health > o.Size {
		difference := o.Health - o.Size
		o.Size = math.Min(o.Size+(difference*growthFactor), o.traits.MaxSize)
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

// isMuchFood reports whether an amount of food is large against this organism's own size.
func (o *Organism) isMuchFood(value float64) bool {
	return value > 0 && value > o.Size*c.MuchFoodPerSize()
}

func (o *Organism) isMuchFoodAtPoint(p utils.Point) bool {
	return o.lookupAPI.CheckFoodAtPoint(p, func(item *food.Item, exists bool) bool {
		return exists && item != nil && o.isMuchFood(float64(item.Value))
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

func (o *Organism) isRelativeAhead() bool {
	return o.isRelativeAtPoint(o.Location.Add(o.Direction))
}

func (o *Organism) isRelativeAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		return x != nil && areRelatives(o.TreeNode, x.TreeNode)
	})
}

func (o *Organism) isPhTooLowHere() bool {
	width := effects.PhToleranceWidth(c.GetCurrentGlobals(), o.traits.Abilities[physiology.AbilityTolerance])
	return o.lookupAPI.GetPhAtPoint(o.Location) <= o.Traits().IdealPh-width
}

func (o *Organism) isPhTooHighHere() bool {
	width := effects.PhToleranceWidth(c.GetCurrentGlobals(), o.traits.Abilities[physiology.AbilityTolerance])
	return o.lookupAPI.GetPhAtPoint(o.Location) >= o.Traits().IdealPh+width
}

func (o *Organism) isOrganismLeft() bool {
	return o.isOrganismAtPoint(o.Location.Add(o.Direction.Left()))
}

func (o *Organism) isOrganismRight() bool {
	return o.isOrganismAtPoint(o.Location.Add(o.Direction.Right()))
}

// isHealthyPhHere reports whether this cell's pH is inside the band the organism's Tolerance lets it bear.
func (o *Organism) isHealthyPhHere() bool {
	width := effects.PhToleranceWidth(c.GetCurrentGlobals(), o.traits.Abilities[physiology.AbilityTolerance])
	return o.isPhHealthyAtPoint(o.Location, o.Traits().IdealPh, width)
}

// canChemosynthesizeHere reports whether chemosynthesizing in this cell would gain health rather than cost it.
func (o *Organism) canChemosynthesizeHere() bool {
	width := effects.ChemoWidth(c.GetCurrentGlobals(), o.traits.Abilities[physiology.AbilityChemosynthesis])
	return o.isPhHealthyAtPoint(o.Location, o.Traits().IdealPh, width)
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

// isMuchBiggerOrganismAtPoint / isMuchSmallerOrganismAtPoint answer the size-RATIO conditions, where isBiggerOrganismAtPoint answers only "larger at all".
func (o *Organism) isMuchBiggerOrganismAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		return x != nil && x.Size+x.perceivedSizeBonus() > o.Size*c.MuchBiggerSizeRatio()
	})
}

func (o *Organism) isMuchSmallerOrganismAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		return x != nil && x.Size+x.perceivedSizeBonus() < o.Size*c.MuchSmallerSizeRatio()
	})
}

func (o *Organism) isBiggerOrganismAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		// Compare against the OTHER organism's perceived size so a future posture can make its bearer read as larger to a sensing neighbour.
		return x != nil && x.Size+x.perceivedSizeBonus() > o.Size
	})
}

// perceivedSizeBonus returns any per-cycle addition to how large this organism appears to another's size comparison.
func (o *Organism) perceivedSizeBonus() float64 {
	return 0
}

func (o *Organism) isOrganismAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		return x != nil
	})
}

// isSomethingAtPoint is the coarse flank read: anything at all in the cell — an organism, a wall or food.
func (o *Organism) isSomethingAtPoint(p utils.Point) bool {
	return o.isOrganismAtPoint(p) || o.isWallAtPoint(p) || o.isFoodAtPoint(p)
}

func (o *Organism) isWallAtPoint(p utils.Point) bool {
	return o.lookupAPI.IsWallAtPoint(p)
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

// canMove answers the CanMove condition: is the cell ahead somewhere this organism can end up this cycle.
func (o *Organism) canMove() bool {
	if o.isWallAhead() && !o.CanBurrowAhead() {
		return false
	}
	if o.isOrganismAhead() {
		return false
	}
	return true
}

// CanBurrowAhead reports whether the organism can shoulder through the wall in front of it.
func (o *Organism) CanBurrowAhead() bool {
	ahead := o.Location.Add(o.Direction)
	strength := o.lookupAPI.GetWallStrengthAtPoint(ahead)
	if strength <= 0 {
		return false
	}
	return effects.CanBreakWall(c.GetCurrentGlobals(),
		o.Abilities()[physiology.AbilityDigging], o.Size, strength)
}

// lineageEndCycle reads a node's LineageEndCycle, treating an organism with no tree node as having a surviving line.
func lineageEndCycle(n *DescendantNode) int {
	if n == nil {
		return 0
	}
	return n.LineageEndCycle
}

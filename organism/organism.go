package organism

import (
	"image/color"
	"math"
	"sync"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
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
	decisionTree := d.TreeFromAction(d.ActChemosynthesis)
	for mutations := 0; mutations < c.InitialDecisionTreeMutations(); mutations++ {
		decisionTree = d.MutateTree(rng, decisionTree)
	}
	traits.OrganismColor = d.TreeColor(decisionTree)
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
	if rng.Float64() < o.ChanceToMutateDecisionTree() {
		inheritedTree = d.MutateTree(rng, inheritedTree)
	}
	traits.OrganismColor = d.TreeColor(inheritedTree)
	organism := Organism{
		ID:                   id,
		Age:                  0,
		Health:               o.InitialHealth(),
		Size:                 o.InitialHealth(),
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
	traits Traits, tree *d.Tree, action d.Action, api LookupAPI) *Organism {
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
		PhEffect:      o.traits.PhGrowthEffect,
		ChemoFailed:   o.ChemoFailed,
		EatFailed:     o.EatFailed,
		BornThisCycle: o.BornThisCycle,
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

// UpdateAction runs on each cycle, occasionally changing the current decision
// tree before running it to determine its next action.
//
// chooseAction is called every cycle even when the sim is about to
// override the result with ActSpawn, so the tree's UsedLastCycle markers
// stay populated for the panel's decision-tree display. Without this,
// spawn cycles would leave every node at UsedLastCycle=false (cleared
// by UpdateStats and never re-set) and the panel would show no ◀◀
// arrows at all.
func (o *Organism) UpdateAction() {
	chosen := o.chooseAction(o.decisionTree.Node)
	if o.shouldSpawn() {
		o.CyclesSinceLastSpawn = 0
		o.action = d.ActSpawn
		return
	}
	o.action = chosen
}

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

// Traits returns an organism's traits
func (o Organism) Traits() Traits    { return o.traits }
func (o *Organism) TraitsRef() *Traits { return &o.traits }

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

// ChanceToMutateDecisionTree returns the chance this organism will give a
// mutated copy of its decision tree to each spawned child
func (o Organism) ChanceToMutateDecisionTree() float64 { return o.traits.ChanceToMutateDecisionTree }

// Action returns the Organism's currently-chosen action
func (o Organism) Action() d.Action { return o.action }

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
	return o.isPhHealthyAtPoint(o.Location, o.Traits().IdealPh, o.Traits().PhTolerance)
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
		return x != nil && x.Size > o.Size
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

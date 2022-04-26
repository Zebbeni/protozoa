package organism

import (
	"image/color"
	"math"
	"math/rand"

	c "github.com/Zebbeni/protozoa/config"
	d "github.com/Zebbeni/protozoa/decision"
	"github.com/Zebbeni/protozoa/food"
	"github.com/Zebbeni/protozoa/utils"
)

// Organism contains:
// - location (X, Y)
// - direction (angle, x & y vectors)
// - current action (Action)
// - algorithm code (String? or []int?)
// - algorithm (func)
type Organism struct {
	ID                   int
	Age                  int
	Health               float64
	PrevHealth           float64
	Size                 float64
	Children             int
	CyclesSinceLastSpawn int
	Location             utils.Point
	Direction            utils.Point
	OriginalAncestorID   int

	traits Traits

	decisionTree *d.Tree
	action       d.Action

	lookupAPI LookupAPI
}

// NewRandom initializes organism at with random grid location and direction
func NewRandom(id int, point utils.Point, api LookupAPI) *Organism {
	traits := newRandomTraits()
	decisionTree := d.TreeFromAction(d.GetRandomAction())
	for mutations := 0; mutations < c.InitialDecisionTreeMutations(); mutations++ {
		decisionTree = d.MutateTree(decisionTree)
	}
	organism := Organism{
		ID:                   id,
		Age:                  0,
		Health:               traits.SpawnHealth,
		PrevHealth:           traits.SpawnHealth,
		Size:                 traits.SpawnHealth,
		Children:             0,
		CyclesSinceLastSpawn: 0,
		Location:             point,
		Direction:            utils.GetRandomDirection(),
		OriginalAncestorID:   id,

		traits:       traits,
		decisionTree: decisionTree,
		action:       d.ActChemosynthesis,

		lookupAPI: api,
	}
	return &organism
}

// NewChild initializes and returns a new organism with a copied TreeLibrary from its parent
func (o *Organism) NewChild(id int, point utils.Point, api LookupAPI) *Organism {
	traits := o.traits.copyMutated()
	inheritedTree := o.GetDecisionTreeCopy()
	if rand.Float64() < o.ChanceToMutateDecisionTree() {
		inheritedTree = d.MutateTree(inheritedTree)
	}
	organism := Organism{
		ID:                   id,
		Age:                  0,
		Health:               o.InitialHealth(),
		PrevHealth:           o.InitialHealth(),
		Size:                 o.InitialHealth(),
		Children:             0,
		CyclesSinceLastSpawn: 0,
		Location:             point,
		Direction:            utils.GetRandomDirection(),
		OriginalAncestorID:   o.OriginalAncestorID,

		traits:       traits,
		decisionTree: inheritedTree,
		action:       d.ActChemosynthesis,

		lookupAPI: api,
	}
	return &organism
}

func (o *Organism) Info() *Info {
	return &Info{
		ID:         o.ID,
		Health:     o.Health,
		Location:   o.Location,
		Size:       o.Size,
		Action:     o.action,
		AncestorID: o.OriginalAncestorID,
		Color:      o.traits.OrganismColor,
		Age:        o.Age,
		Children:   o.Children,
		PhEffect:   o.traits.PhEffect,
	}
}

// UpdateStats runs on each cycle and updates Age, CyclesSinceLastSpawn, etc.
// Also calculates the change in health since the last cycle and applies this
// to the success metrics of the last-used decision tree.
func (o *Organism) UpdateStats() {
	o.Age++
	o.CyclesSinceLastSpawn++

	healthChange := o.Health - o.PrevHealth
	// compensate for health cost due to reproduction if applicable
	// (don't penalize decision tree for a drop in health it didn't cause)
	if o.CyclesSinceLastSpawn == 1 && o.Age > 1 {
		healthChange -= o.Size * o.HealthCostToReproduce()
	}

	o.decisionTree.ResetUsedLastCycle()
	o.PrevHealth = o.Health
}

// UpdateAction runs on each cycle, occasionally changing the current decision
// tree before running it to determine its next action
func (o *Organism) UpdateAction() {
	if o.shouldSpawn() {
		o.CyclesSinceLastSpawn = 0
		o.action = d.ActSpawn
		return
	}

	o.action = o.chooseAction(o.decisionTree.Node)
}

func (o *Organism) shouldSpawn() bool {
	cyclesRequirementMet := o.CyclesSinceLastSpawn >= o.MinCyclesBetweenSpawns()
	healthRequirementMet := o.Health > o.MinHealthToSpawn()
	populationRequirementMet := o.lookupAPI.OrganismCount() < c.MaxOrganisms()
	return populationRequirementMet && cyclesRequirementMet && healthRequirementMet
}

// chooseAction walks through nodes of an organism's decision tree, eventually
// returning the chosen action
//
// As chooseAction walks through nodes, it also sets UsedLastCycle=true, allowing
// the organism to attribute success or failure to the previously-chosen path
func (o *Organism) chooseAction(node *d.Node) d.Action {
	node.UsedLastCycle = true
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
	case d.IsSmallerOrganismAhead:
		return o.isSmallerOrganismAhead()
	case d.IsRelatedOrganismAhead:
		return o.isRelatedOrganismAhead()
	case d.IsOrganismLeft:
		return o.isOrganismLeft()
	case d.IsBiggerOrganismLeft:
		return o.isBiggerOrganismLeft()
	case d.IsSmallerOrganismLeft:
		return o.isSmallerOrganismLeft()
	case d.IsRelatedOrganismLeft:
		return o.isRelatedOrganismLeft()
	case d.IsOrganismRight:
		return o.isOrganismRight()
	case d.IsBiggerOrganismRight:
		return o.isBiggerOrganismRight()
	case d.IsSmallerOrganismRight:
		return o.isSmallerOrganismRight()
	case d.IsRelatedOrganismRight:
		return o.isRelatedOrganismRight()
	case d.IsRandomFiftyPercent:
		return rand.Float32() < 0.5
	case d.IsHealthAboveFiftyPercent:
		return o.Health > o.Size*0.5
	case d.IsHealthyPhHere:
		return o.isHealthyPhHere()
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

// GetCurrentDecisionTreeLength returns the number of nodes in the organism's currently-used
// decision tree
func (o *Organism) GetCurrentDecisionTreeLength() int {
	return o.decisionTree.Size()
}

// GetAction returns the last-chosen Organism action
func (o Organism) GetAction() d.Action { return o.action }

// Traits returns an organism's traits
func (o Organism) Traits() Traits { return o.traits }

// InitialHealth returns the health an organism and its children start life with
func (o Organism) InitialHealth() float64 { return o.traits.SpawnHealth }

// HealthCostToReproduce returns the health to lose upon spawning a child
func (o Organism) HealthCostToReproduce() float64 { return o.traits.SpawnHealth * -1.0 }

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
	o.Health = math.Min(math.Max(o.Health, 0.0), o.Size)
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
	return o.lookupAPI.CheckFoodAtPoint(point, func(f *food.Item) bool {
		return f != nil
	})
}

func (o *Organism) isOrganismAhead() bool {
	return o.isOrganismAtPoint(o.Location.Add(o.Direction))
}

func (o *Organism) isBiggerOrganismAhead() bool {
	return o.isBiggerOrganismAtPoint(o.Location.Add(o.Direction))
}

func (o *Organism) isSmallerOrganismAhead() bool {
	return o.isSmallerOrganismAtPoint(o.Location.Add(o.Direction))
}

func (o *Organism) isRelatedOrganismAhead() bool {
	return o.isRelatedOrganismAtPoint(o.Location.Add(o.Direction))
}

func (o *Organism) isOrganismLeft() bool {
	return o.isOrganismAtPoint(o.Location.Add(o.Direction.Left()))
}

func (o *Organism) isBiggerOrganismLeft() bool {
	return o.isBiggerOrganismAtPoint(o.Location.Add(o.Direction.Left()))
}

func (o *Organism) isSmallerOrganismLeft() bool {
	return o.isSmallerOrganismAtPoint(o.Location.Add(o.Direction.Left()))
}

func (o *Organism) isRelatedOrganismLeft() bool {
	return o.isRelatedOrganismAtPoint(o.Location.Add(o.Direction.Left()))
}

func (o *Organism) isOrganismRight() bool {
	return o.isOrganismAtPoint(o.Location.Add(o.Direction.Right()))
}

func (o *Organism) isBiggerOrganismRight() bool {
	return o.isBiggerOrganismAtPoint(o.Location.Add(o.Direction.Right()))
}

func (o *Organism) isSmallerOrganismRight() bool {
	return o.isSmallerOrganismAtPoint(o.Location.Add(o.Direction.Right()))
}

func (o *Organism) isRelatedOrganismRight() bool {
	return o.isRelatedOrganismAtPoint(o.Location.Add(o.Direction.Right()))
}

func (o *Organism) isHealthyPhHere() bool {
	return o.isPhHealthyAtPoint(o.Location, o.Traits().IdealPh, o.Traits().PhTolerance)
}

func (o *Organism) isBiggerOrganismAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		return x != nil && x.Size > o.Size
	})
}

func (o *Organism) isSmallerOrganismAtPoint(p utils.Point) bool {
	return o.lookupAPI.CheckOrganismAtPoint(p, func(x *Organism) bool {
		return x != nil && x.Size < o.Size
	})
}

func (o *Organism) isRelatedOrganismAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		return x != nil && x.OriginalAncestorID == o.OriginalAncestorID
	})
}

func (o *Organism) isOrganismAtPoint(p utils.Point) bool {
	return o.checkOrganismAtPoint(p, func(x *Organism) bool {
		return x != nil
	})
}

func (o *Organism) checkOrganismAtPoint(p utils.Point, checkFunc OrgCheck) bool {
	return o.lookupAPI.CheckOrganismAtPoint(p, checkFunc)
}

func (o *Organism) isPhHealthyAtPoint(p utils.Point, ideal, tolerance float64) bool {
	ph := o.lookupAPI.GetPhAtPoint(p)
	return math.Abs(ph-ideal) < tolerance
}

func (o *Organism) canMove() bool {
	if o.isOrganismAhead() {
		return false
	}
	if o.isFoodAhead() {
		return false
	}
	return true
}

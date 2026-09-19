package ux

import (
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/organism"
)

// The FAMILY colour mode: tint every organism by how it is related to the
// selected one, so a lineage's shape in the world is visible at a glance.
//
// It replaces the faded highlight boxes that used to be stamped over
// every living descendant. Boxes answered one question — descendant or
// not — and answered it identically for a child and a great-great-
// grandchild, while saying nothing about ancestors or cousins. A tint
// carries the whole relationship, and costs nothing extra to draw, since
// the organism sprite is being tinted anyway.
//
// Relatedness is measured through the lowest common ancestor. For a
// candidate organism, `up` is how many generations from the selected
// organism up to that ancestor and `down` how many from the ancestor
// down to the candidate:
//
//	up 0, down 0   the selected organism
//	up 0, down d   a direct descendant, d generations below
//	up a, down 0   a direct ancestor, a generations above
//	up a, down d   a cousin: the line split a generations up
//	no ancestor    unrelated — a different founding tree entirely
type kinship struct {
	up, down int
	// related is false when the two share no ancestor at all, which the
	// zero value therefore does not mean.
	related bool
}

// familyFadeGenerations is the distance at which each ramp reaches its
// far end. Deliberately short: most of what is on screen at any moment
// is distant kin, and a ramp long enough to distinguish the 30th
// generation from the 40th would leave the first few — the ones actually
// worth telling apart — crowded into the same green.
const familyFadeGenerations = 8

// familyCousinFade is how far a cousin line has to diverge before it
// desaturates to flat gray. Shorter than familyFadeGenerations because
// there are vastly more cousins than direct kin: this is what keeps the
// majority of the screen gray, so the direct line reads as the subject
// rather than as one more coloured thing among many.
const familyCousinFade = 6

// Hues on the HSLuv wheel, the same space the rest of the grid's colour
// ramps use. Descendants run green→blue and ancestors green→yellow, so
// the two directions of the family tree are told apart by hue rather
// than by brightness — brightness is already carrying size and health
// information in the sprite art.
const (
	familyHueSelected   = 120.0 // green
	familyHueDescendant = 255.0 // blue
	familyHueAncestor   = 70.0  // yellow
	familyHueCousin     = 12.0  // red
)

// familyTinter answers "how is this organism related to the selected
// one" in O(1) per organism, amortised.
//
// The direct approach — walk up from each organism looking for the
// selected one's ancestor chain — is O(depth) per organism per frame,
// on a tree that is walked for the population graph already. Instead the
// answer is memoised per node: a walk up stops at the first node whose
// kinship is known and assigns back down the stack it collected. Since
// an organism's parent is always memoised by the time the child is born,
// steady-state cost is one map lookup per organism.
//
// The memo is only valid for one selection and one generation of trees.
// A replay seek rebuilds the trees from a snapshot, after which cached
// node pointers are meaningless, which is what treesGen guards.
type familyTinter struct {
	selID     int
	treesGen  int
	ancestors map[int]int // node ID -> generations above the selected organism
	memo      map[int]kinship
}

// newFamilyTinter builds the selected organism's ancestor chain, which is
// the spine every other answer is measured against. Cheap: one walk from
// the selection to the root of its tree.
func newFamilyTinter(sel *organism.DescendantNode, selID, treesGen int) *familyTinter {
	ft := &familyTinter{
		selID:     selID,
		treesGen:  treesGen,
		ancestors: map[int]int{},
		memo:      map[int]kinship{},
	}
	for n, up := sel, 0; n != nil; n, up = n.Parent, up+1 {
		ft.ancestors[n.ID] = up
	}
	return ft
}

// valid reports whether this tinter still answers for the given
// selection and tree generation.
func (ft *familyTinter) valid(selID, treesGen int) bool {
	return ft != nil && ft.selID == selID && ft.treesGen == treesGen
}

// cached is the answer for an organism already seen under this selection,
// which after the first frame is nearly all of them. It takes an ID
// rather than a node so the caller can skip the tree lookup — that lookup
// takes the organism manager's read lock, and doing it per organism per
// frame is the one part of this that scales with population.
func (ft *familyTinter) cached(id int) (kinship, bool) {
	k, ok := ft.memo[id]
	return k, ok
}

// kinshipOf walks up from n until it reaches something it already knows —
// a node on the selected organism's ancestor chain, or a node already in
// the memo — then fills in every node it passed on the way. Each of those
// is one generation further down than the node above it.
func (ft *familyTinter) kinshipOf(n *organism.DescendantNode) kinship {
	if n == nil {
		return kinship{}
	}
	if k, ok := ft.memo[n.ID]; ok {
		return k
	}

	var stack []*organism.DescendantNode
	cur := n
	var base kinship
	for {
		if up, ok := ft.ancestors[cur.ID]; ok {
			base = kinship{up: up, related: true}
			break
		}
		if k, ok := ft.memo[cur.ID]; ok {
			base = k
			break
		}
		stack = append(stack, cur)
		if cur.Parent == nil {
			// Ran out of tree without meeting the selection's line: a
			// different founder entirely.
			base = kinship{}
			break
		}
		cur = cur.Parent
	}

	// Walk back down, each step one generation further from the common
	// ancestor. An unrelated base stays unrelated however far it goes.
	for i := len(stack) - 1; i >= 0; i-- {
		if base.related {
			base = kinship{up: base.up, down: base.down + 1, related: true}
		}
		ft.memo[stack[i].ID] = base
	}
	return base
}

// familyColor is the tint for one kinship. Saturation is what separates
// the direct line from everything else: direct kin stay fully saturated
// whatever the distance, cousins fade to gray with it.
func familyColor(k kinship) colorful.Color {
	if !k.related {
		return familyUnrelatedColor()
	}
	switch {
	case k.up == 0 && k.down == 0:
		return colorful.HSLuv(familyHueSelected, 1, 0.6)
	case k.up == 0:
		// Straight down the line: green to blue.
		return colorful.HSLuv(familyRamp(familyHueSelected, familyHueDescendant, k.down), 0.95, 0.55)
	case k.down == 0:
		// Straight up the line: green to yellow.
		return colorful.HSLuv(familyRamp(familyHueSelected, familyHueAncestor, k.up), 0.95, 0.55)
	}

	// A cousin. Hue runs on from the yellow end towards red with the
	// whole distance, and saturation falls away with it — a cousin far
	// enough out is simply part of the background population.
	dist := k.up + k.down
	hue := familyRamp(familyHueAncestor, familyHueCousin, dist)
	fade := min(1, float64(dist)/familyCousinFade)
	sat := 0.85 * (1 - fade)
	if sat <= 0 {
		return familyUnrelatedColor()
	}
	return colorful.HSLuv(hue, sat, 0.5)
}

// familyRamp interpolates a hue over familyFadeGenerations and holds
// there. Linear rather than eased: the axis is a generation count, and a
// curve would make "two generations" mean different amounts of colour
// depending on where it sat.
func familyRamp(from, to float64, generations int) float64 {
	t := min(1, float64(generations)/familyFadeGenerations)
	return from + (to-from)*t
}

// familyUnrelatedColor is the flat gray worn by everything off the
// selected organism's tree, and by cousins far enough out to have faded
// into it. Mid-lightness in both themes so it reads as "no information"
// rather than as a dark or a bright reading.
func familyUnrelatedColor() colorful.Color {
	return colorful.HSLuv(0, 0, 0.45)
}

package ux

import (
	"github.com/lucasb-eyer/go-colorful"

	"github.com/Zebbeni/protozoa/organism"
)

// The FAMILY colour mode: tint every organism by how it is related to the selected one.
type kinship struct {
	up, down int
	// related is false when the two share no ancestor at all, which the zero value therefore does not mean.
	related bool
}

// familyFadeGenerations is the distance at which each ramp reaches its far end.
const familyFadeGenerations = 8

// familyCousinFade is how far a cousin line has to diverge before it desaturates to flat gray.
const familyCousinFade = 6

// Hues on the HSLuv wheel, the same space the rest of the grid's colour ramps use.
const (
	familyHueSelected   = 120.0 // green
	familyHueDescendant = 255.0 // blue
	familyHueAncestor   = 70.0  // yellow
	familyHueCousin     = 12.0  // red
)

// familyTinter answers "how is this organism related to the selected one" in O(1) per organism, amortised.
type familyTinter struct {
	selID     int
	treesGen  int
	ancestors map[int]int // node ID -> generations above the selected organism
	memo      map[int]kinship
}

// newFamilyTinter builds the selected organism's ancestor chain.
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

// valid reports whether this tinter still answers for the given selection and tree generation.
func (ft *familyTinter) valid(selID, treesGen int) bool {
	return ft != nil && ft.selID == selID && ft.treesGen == treesGen
}

func (ft *familyTinter) cached(id int) (kinship, bool) {
	k, ok := ft.memo[id]
	return k, ok
}

// kinshipOf walks up from n until it reaches something it already knows.
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
			// Ran out of tree without meeting the selection's line: a different founder entirely.
			base = kinship{}
			break
		}
		cur = cur.Parent
	}

	// Walk back down, each step one generation further from the common ancestor.
	for i := len(stack) - 1; i >= 0; i-- {
		if base.related {
			base = kinship{up: base.up, down: base.down + 1, related: true}
		}
		ft.memo[stack[i].ID] = base
	}
	return base
}

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

	dist := k.up + k.down
	hue := familyRamp(familyHueAncestor, familyHueCousin, dist)
	fade := min(1, float64(dist)/familyCousinFade)
	sat := 0.85 * (1 - fade)
	if sat <= 0 {
		return familyUnrelatedColor()
	}
	return colorful.HSLuv(hue, sat, 0.5)
}

func familyRamp(from, to float64, generations int) float64 {
	t := min(1, float64(generations)/familyFadeGenerations)
	return from + (to-from)*t
}

// familyUnrelatedColor is the flat gray worn by everything off the selected organism's tree.
func familyUnrelatedColor() colorful.Color {
	return colorful.HSLuv(0, 0, 0.45)
}

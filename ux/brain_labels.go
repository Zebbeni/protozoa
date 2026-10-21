package ux

import (
	d "github.com/Zebbeni/protozoa/decision"
)

// Short names for the brain diagram, where a label sits beside its node in a
// column 16 characters wide — about a fifth of what `decision.Map`'s own names
// need ("If Much Bigger Organism Ahead").
//
// They are short forms rather than initialisms: the column is narrow, but
// "MBOL" is not a reading of the world, and a direction is one word. So every
// label that reads a neighbouring cell names that cell — Ahead, Left, Right,
// Here — and the diagram needs no key under it to be read.
//
// Written out rather than derived from the full names, since a derivation that
// fits inside 16 characters is an abbreviation scheme, and the point of these
// is that there is nothing to decode.
//
// TestEveryBrainNodeHasAShortLabel walks the mutable pools against both maps,
// so a condition or action added without one fails rather than drawing a
// blank label beside its node.

var brainConditionLabels = map[d.Condition]string{
	d.CanMove:                    "Can Move",
	d.IsFoodAhead:                "Food Ahead",
	d.IsFoodLeft:                 "Food Left",
	d.IsFoodRight:                "Food Right",
	d.IsFoodHere:                 "Food Here",
	d.IsFoodBuriedHere:           "Buried Here",
	d.IsMuchFoodHere:             "Much Food Here",
	d.IsMuchFoodBuriedHere:       "Much Buried Here",
	d.IsOrganismAhead:            "Org Ahead",
	d.IsOrganismLeft:             "Org Left",
	d.IsOrganismRight:            "Org Right",
	d.IsBiggerOrganismAhead:      "Big Org Ahead",
	d.IsBiggerOrganismLeft:       "Big Org Left",
	d.IsBiggerOrganismRight:      "Big Org Right",
	d.IsMuchBiggerOrganismAhead:  "Huge Org Ahead",
	d.IsMuchBiggerOrganismLeft:   "Huge Org Left",
	d.IsMuchBiggerOrganismRight:  "Huge Org Right",
	d.IsMuchSmallerOrganismAhead: "Tiny Org Ahead",
	d.IsMuchSmallerOrganismLeft:  "Tiny Org Left",
	d.IsMuchSmallerOrganismRight: "Tiny Org Right",
	d.IsRelativeAhead:            "Relative Ahead",
	d.IsRelativeLeft:             "Relative Left",
	d.IsRelativeRight:            "Relative Right",
	d.IsSomethingLeft:            "Something Left",
	d.IsSomethingRight:           "Something Right",
	d.IsWallAhead:                "Wall Ahead",
	d.IsWallLeft:                 "Wall Left",
	d.IsWallRight:                "Wall Right",
	d.IsHealthy:                  "Healthy",
	d.IsVeryHealthy:              "Very Healthy",
	d.IsVeryUnhealthy:            "Very Unhealthy",
	d.IsHealthAboveTwentyPercent: "Health > 20%",
	d.IsHealthyPhHere:            "Healthy pH Here",
	d.IsPhTooLowHere:             "pH Too Low Here",
	d.IsPhTooHighHere:            "pH Too High Here",
	d.IsHealthierPhAhead:         "Better pH Ahead",
	d.CanChemosynthesizeHere:     "Can Chemo Here",
	d.IsAgeMultipleOfTwo:         "Age Mult of 2",
	d.IsAgeMultipleOfTen:         "Age Mult of 10",
}

var brainActionLabels = map[d.Action]string{
	d.ActChemosynthesis: "Chemo",
	d.ActEat:            "Eat",
	d.ActMove:           "Move",
	d.ActTurnLeft:       "Turn Left",
	d.ActTurnRight:      "Turn Right",
	d.ActAttack:         "Attack",
	d.ActDig:            "Dig",
	d.ActIdle:           "Idle",
	d.ActSpawn:          "Spawn",
}

// brainConditionLabel and brainActionLabel fall back to the full name rather
// than to nothing, so a node type with no short form is drawn too wide
// instead of drawn blank — a label running into the diagram is a visible
// prompt to add one, where an empty column is not.
func brainConditionLabel(c d.Condition) string {
	if short, ok := brainConditionLabels[c]; ok {
		return short
	}
	return d.Map[c]
}

func brainActionLabel(a d.Action) string {
	if short, ok := brainActionLabels[a]; ok {
		return short
	}
	return d.Map[a]
}

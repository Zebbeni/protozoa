package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/manager"
	"github.com/Zebbeni/protozoa/resources"
)

func TestFoodBucketsSplitOnQuarters(t *testing.T) {
	loadKeyGlobals(t)
	maxVal := config.MaxFoodValue()

	for _, tc := range []struct {
		value int
		want  resources.ImageRole
		what  string
	}{
		{1, resources.RoleFoodTiny, "the smallest item there is"},
		{maxVal/4 - 1, resources.RoleFoodTiny, "just under a quarter"},
		{maxVal / 4, resources.RoleFoodSmall, "exactly a quarter"},
		{maxVal/2 - 1, resources.RoleFoodSmall, "just under half"},
		{maxVal / 2, resources.RoleFoodMedium, "exactly half"},
		{3*maxVal/4 - 1, resources.RoleFoodMedium, "just under three quarters"},
		{3 * maxVal / 4, resources.RoleFoodLarge, "exactly three quarters"},
		{maxVal, resources.RoleFoodLarge, "the largest item there is"},
	} {
		if got := foodRoleForValue(tc.value); got != tc.want {
			t.Errorf("food worth %d (%s): role %v, want %v", tc.value, tc.what, got, tc.want)
		}
	}
}

func TestWallBucketsSplitOnQuarters(t *testing.T) {
	loadKeyGlobals(t)
	maxS := manager.MaxWallStrength

	for _, tc := range []struct {
		strength int
		want     resources.ImageRole
	}{
		{manager.MinWallStrength, resources.RoleWallWeak},
		{maxS / 4, resources.RoleWallWeak},
		{maxS/4 + 1, resources.RoleWallMedium},
		{maxS / 2, resources.RoleWallMedium},
		{maxS/2 + 1, resources.RoleWallStrong},
		{3 * maxS / 4, resources.RoleWallStrong},
		{3*maxS/4 + 1, resources.RoleWallGiant},
		{maxS, resources.RoleWallGiant},
	} {
		if got := wallRoleForStrength(tc.strength); got != tc.want {
			t.Errorf("wall strength %d: role %v, want %v", tc.strength, got, tc.want)
		}
	}
}

// TestEverySizeTierIsReachable: a bucket no value can land in is art nobody will ever see.
func TestEverySizeTierIsReachable(t *testing.T) {
	loadKeyGlobals(t)

	food := map[resources.ImageRole]bool{}
	for v := 1; v <= config.MaxFoodValue(); v++ {
		food[foodRoleForValue(v)] = true
	}
	for _, want := range []resources.ImageRole{
		resources.RoleFoodTiny, resources.RoleFoodSmall,
		resources.RoleFoodMedium, resources.RoleFoodLarge,
	} {
		if !food[want] {
			t.Errorf("no food value maps to role %v", want)
		}
	}

	walls := map[resources.ImageRole]bool{}
	for s := manager.MinWallStrength; s <= manager.MaxWallStrength; s++ {
		walls[wallRoleForStrength(s)] = true
	}
	for _, want := range []resources.ImageRole{
		resources.RoleWallWeak, resources.RoleWallMedium,
		resources.RoleWallStrong, resources.RoleWallGiant,
	} {
		if !walls[want] {
			t.Errorf("no wall strength maps to role %v", want)
		}
	}
}

// TestBuriedSpritesBucketAgainstTheBuriedCeiling: the buried layer draws
// with the food sprites, and bucketing it against the SURFACE limit drew
// every stocked cell as the largest tier once the ground could hold ten
// times as much.
func TestBuriedSpritesBucketAgainstTheBuriedCeiling(t *testing.T) {
	loadKeyGlobals(t)
	g := config.GetCurrentGlobals()
	g.MaxFoodValue = 100
	g.MaxBuriedFoodValue = 1000
	config.SetGlobals(g)

	seen := map[resources.ImageRole]bool{}
	for v := 1; v <= 1000; v += 7 {
		seen[foodRoleAgainst(v, config.MaxBuriedFoodValue())] = true
	}
	for _, want := range []resources.ImageRole{
		resources.RoleFoodTiny, resources.RoleFoodSmall,
		resources.RoleFoodMedium, resources.RoleFoodLarge,
	} {
		if !seen[want] {
			t.Errorf("no buried amount up to the ceiling draws %v; the tiers are not reachable", want)
		}
	}
	// A value a surface pile could hold is near the bottom of the buried
	// range, not the top.
	if got := foodRoleAgainst(100, 1000); got != resources.RoleFoodTiny {
		t.Errorf("100 buried of 1000 draws %v, want the smallest tier", got)
	}
}

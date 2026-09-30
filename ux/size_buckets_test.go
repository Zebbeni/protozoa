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

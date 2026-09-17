package ux

import (
	"testing"

	"github.com/Zebbeni/protozoa/physiology"
)

func findField(t *testing.T, cs *ConfigScreen, jsonTag string) configField {
	t.Helper()
	for _, section := range cs.sections {
		for _, field := range section.fields {
			if field.jsonTag == jsonTag && field.row == rowField {
				return field
			}
		}
	}
	t.Fatalf("no config field %q", jsonTag)
	return configField{}
}

func TestResetFieldRestoresDefault(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	f := findField(t, cs, "max_lifespan")
	want := cs.globals.MaxLifespan

	if cs.canReset(f) {
		t.Fatal("an unchanged setting shouldn't offer a reset")
	}
	cs.globals.MaxLifespan = want + 123
	if !cs.canReset(f) {
		t.Fatal("a changed setting should offer a reset")
	}
	cs.resetField(f)
	if cs.globals.MaxLifespan != want || cs.canReset(f) {
		t.Errorf("reset left max_lifespan at %d, want %d", cs.globals.MaxLifespan, want)
	}
}

func TestResetAbilityScoreRow(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	var row configField
	for _, section := range cs.sections {
		for _, field := range section.fields {
			if field.row == rowAbilityScore && field.ability == int(physiology.AbilityAttack) {
				row = field
			}
		}
	}
	want := cs.globals.InitialAbilityScores[physiology.AbilityAttack]
	cs.adjustAbilityScore(int(physiology.AbilityAttack), 9)
	if !cs.canReset(row) {
		t.Fatal("a changed ability score should offer a reset")
	}
	cs.resetField(row)
	if got := cs.globals.InitialAbilityScores[physiology.AbilityAttack]; got != want {
		t.Errorf("reset attack score = %d, want %d", got, want)
	}
}

// TestRestoreAllDefaults puts everything back, and the restored ability
// scores don't share storage with the defaults.
func TestRestoreAllDefaults(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	cs.globals.MaxLifespan += 50
	cs.globals.AttackDamageAtFullAttack = 1
	cs.globals.Theme = "light-custom"
	cs.adjustAbilityScore(int(physiology.AbilityDefense), 4)
	if cs.allDefaults() {
		t.Fatal("changed settings should not read as all defaults")
	}

	cs.restoreAllDefaults()
	if !cs.allDefaults() {
		t.Error("after restoring, every setting should match its default")
	}
	if cs.globals.Theme != "light-custom" {
		t.Errorf("restoring defaults changed the theme to %q", cs.globals.Theme)
	}
	cs.adjustAbilityScore(int(physiology.AbilityDefense), 1)
	if cs.defaults.InitialAbilityScores[physiology.AbilityDefense] == cs.globals.InitialAbilityScores[physiology.AbilityDefense] {
		t.Error("editing after a restore changed the stored defaults")
	}
}

// TestSectionChangedFollowsItsFields: a section reports changes only when
// one of its own settings differs from the default.
func TestSectionChangedFollowsItsFields(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	for _, section := range cs.sections {
		if cs.sectionChanged(section) {
			t.Fatalf("section %q reports changes on defaults", section.title)
		}
	}
	cs.globals.MaxLifespan += 5
	changed := 0
	for _, section := range cs.sections {
		if cs.sectionChanged(section) {
			changed++
			findInSection(t, section, "max_lifespan")
		}
	}
	if changed != 1 {
		t.Errorf("%d sections report changes after editing one setting, want 1", changed)
	}
}

func findInSection(t *testing.T, section configSection, jsonTag string) {
	t.Helper()
	for _, field := range section.fields {
		if field.jsonTag == jsonTag {
			return
		}
	}
	t.Errorf("section %q reports changes but doesn't hold %q", section.title, jsonTag)
}

// TestCurveRowResetsItsShape: a curve's row carries its shape setting, so
// the row offers a reset once the shape has been changed — and resets the
// shape rather than whatever field happens to sit at index zero.
func TestCurveRowResetsItsShape(t *testing.T) {
	cs, _ := abilityConfigScreen(t)
	var row configField
	for _, section := range cs.sections {
		for _, field := range section.fields {
			if field.row == rowCurveHeader && field.curve == physiology.CurveAttack {
				row = field
			}
		}
	}
	if row.jsonTag != "attack_curve_shape" {
		t.Fatalf("the attack curve row carries %q, want attack_curve_shape", row.jsonTag)
	}
	if cs.canReset(row) {
		t.Fatal("an unchanged curve shouldn't offer a reset")
	}

	want := cs.globals.AttackCurveShape
	seed := cs.globals.Seed
	cs.setCurveShape(physiology.CurveAttack, physiology.ShapeQuadratic)
	if !cs.canReset(row) {
		t.Fatal("a changed shape should offer a reset")
	}
	cs.resetField(row)
	if cs.globals.AttackCurveShape != want {
		t.Errorf("reset left the shape at %q, want %q", cs.globals.AttackCurveShape, want)
	}
	if cs.globals.Seed != seed {
		t.Errorf("reset changed the seed to %d — the row is pointing at the wrong field", cs.globals.Seed)
	}
}

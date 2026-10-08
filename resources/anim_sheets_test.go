package resources

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
)

// initSprites loads the shipped settings before Init, which reads the theme
// to pick which image directory the sprites come from.
func initSprites(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "settings", "default.json"))
	if err != nil {
		t.Fatalf("read settings/default.json: %v", err)
	}
	var g config.Globals
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("decode settings/default.json: %v", err)
	}
	config.SetGlobals(&g)
	UseDirAssets("..")
	Init()
}

// TestEveryAnimationHasAFileName: loadAnimSheets skips an animation whose
// stem is empty, so a missing entry is art that silently never loads.
func TestEveryAnimationHasAFileName(t *testing.T) {
	stems := map[string]animation.Animation{}
	for _, anim := range animation.AllAnimations {
		stem := animationFileName[anim]
		if stem == "" {
			t.Errorf("animation %d has no filename stem", anim)
			continue
		}
		if other, dup := stems[stem]; dup {
			t.Errorf("animations %d and %d both load %q", other, anim, stem)
		}
		stems[stem] = anim
	}
}

// TestBorrowedAnimationsNameRealSources: a borrow pointing at an animation
// that itself has no art would leave both nil.
func TestBorrowedAnimationsNameRealSources(t *testing.T) {
	listed := map[animation.Animation]bool{}
	for _, a := range animation.AllAnimations {
		listed[a] = true
	}
	for anim, from := range animationBorrows {
		if !listed[anim] || !listed[from] {
			t.Errorf("borrow %d <- %d names an animation outside AllAnimations", anim, from)
		}
		if _, chained := animationBorrows[from]; chained {
			t.Errorf("borrow %d <- %d points at another borrow; the fallback pass is one deep", anim, from)
		}
	}
}

// TestAKillBorrowsTheAttackSheetUntilItHasArt is the claim the README makes
// to the artist: the tag can be drawn later and nothing else has to change.
// It is also what stops a killer turning invisible today, since no
// attack_success art exists.
func TestAKillBorrowsTheAttackSheetUntilItHasArt(t *testing.T) {
	initSprites(t)

	borrowed, checked := 0, 0
	// Every zoom set, not just the active one: the layered overlays only
	// exist in the 16x16 art, which is where a missing sheet would show as
	// a killer losing its teeth and sensors mid-strike.
	for level, images := range ZoomImages {
		for role := range organismRoleName {
			for layer, frames := range images[role] {
				if frames == nil || frames[animation.AnimAttack] == nil {
					continue
				}
				checked++
				kill := frames[animation.AnimAttackMove]
				if kill == nil {
					t.Errorf("zoom %d role %d layer %d has attack art and no attack-kill frames",
						level, role, layer)
					continue
				}
				if len(kill) != len(frames[animation.AnimAttack]) {
					t.Errorf("zoom %d role %d layer %d borrowed %d kill frames from %d attack frames",
						level, role, layer, len(kill), len(frames[animation.AnimAttack]))
				}
				if &kill[0] != &frames[animation.AnimAttack][0] {
					// Real art has arrived; that is the intended end state,
					// not a failure, so say so rather than assert the borrow.
					t.Logf("zoom %d role %d layer %d has its own attack_success art", level, role, layer)
					continue
				}
				borrowed++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no role carried attack art, so nothing was checked")
	}
	t.Logf("%d of %d layer sets borrow the attack sheet", borrowed, checked)
}

// TestTheLuaExportKnowsEveryAnimationTag ties the artist's vocabulary to the
// loader's. The script and resources.go each hold the list by hand, which is
// exactly the shape that gets updated in one place and not the other.
func TestTheLuaExportKnowsEveryAnimationTag(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("gen", "export_spritesheets.lua"))
	if err != nil {
		t.Fatalf("read the export script: %v", err)
	}
	script := string(data)
	known := luaTagList(t, script, "expectedTagActions")
	known = append(known, luaTagList(t, script, "pendingTagActions")...)
	in := map[string]bool{}
	for _, tag := range known {
		in[tag] = true
	}
	for _, anim := range animation.AllAnimations {
		stem := animationFileName[anim]
		if stem != "" && !in[stem] {
			t.Errorf("the export script has no tag %q; the artist cannot author art the loader looks for", stem)
		}
	}
}

// luaTagList pulls the quoted strings out of one lua list literal.
func luaTagList(t *testing.T, script, name string) []string {
	t.Helper()
	at := strings.Index(script, "local "+name)
	if at < 0 {
		t.Fatalf("the export script has no %s", name)
	}
	body := script[at:]
	end := strings.Index(body, "}")
	if end < 0 {
		t.Fatalf("%s is not terminated", name)
	}
	var out []string
	for _, part := range strings.Split(body[:end], "\"") {
		if part = strings.TrimSpace(part); part != "" && !strings.ContainsAny(part, "={,") {
			out = append(out, part)
		}
	}
	return out
}

// TestSpawnArtIsLoadedNotBorrowed: the spawn sheets have been in the asset
// directory all along and nothing loaded them, because the animation had no
// filename stem. This checks the art reaches the renderer rather than the
// role's static image standing in for it.
func TestSpawnArtIsLoadedNotBorrowed(t *testing.T) {
	initSprites(t)

	withArt, checked := 0, 0
	for level, images := range ZoomImages {
		for role := range organismRoleName {
			for layer, frames := range images[role] {
				if frames == nil || frames[animation.AnimIdle] == nil {
					continue
				}
				checked++
				spawn := frames[animation.AnimSpawn]
				if spawn == nil {
					t.Errorf("zoom %d role %d layer %d has idle art and no spawn frames", level, role, layer)
					continue
				}
				if &spawn[0] != &frames[animation.AnimIdle][0] {
					withArt++
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no role carried idle art, so nothing was checked")
	}
	if withArt == 0 {
		t.Errorf("all %d layer sets fall back to idle for spawning; the sheets are not being loaded", checked)
	}
	t.Logf("%d of %d layer sets have spawn art of their own", withArt, checked)
}

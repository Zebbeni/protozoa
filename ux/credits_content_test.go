package ux

import (
	"os"
	"strings"
	"testing"

	r "github.com/Zebbeni/protozoa/resources"
)

// creditsProse is every line of the ABOUT document as one string, which is
// what the tests about the writing read.
func creditsProse() string {
	var sb strings.Builder
	for _, b := range creditsBlocks() {
		switch v := b.(type) {
		case rulesHeading:
			sb.WriteString(v.text + "\n")
		case rulesText:
			sb.WriteString(v.text + "\n")
		}
	}
	return sb.String()
}

// TestTheCreditsNameWhoMadeWhat: a credits page that leaves out one of the
// things it exists to credit is worse than none, since the omission reads as
// a claim.
func TestTheCreditsNameWhoMadeWhat(t *testing.T) {
	prose := creditsProse()
	for _, want := range []string{
		"Andrew Albers", // the game and its art
		"CAPTNPHANTOM",  // the music
		"Liminal Worlds Music & Ambience Pack",
		"Underground Techno Pack",
		"Source Code Pro",
		"Darrell Flood", // Inversionz
		"Hawtpixel",     // the licence it is used under, not the donationware listing
		"Ebitengine",
	} {
		if !strings.Contains(prose, want) {
			t.Errorf("the credits never mention %q", want)
		}
	}
	for _, heading := range []string{"ART", "MUSIC", "TYPEFACES", "BUILT WITH"} {
		if !strings.Contains(prose, heading) {
			t.Errorf("the credits have no %s section", heading)
		}
	}
}

// TestTheMusicAttributionMatchesThePack: the bundle license requires no
// credit at all, but it names the one it would like — the artist name
// "CAPTNPHANTOM" and a link to the pack or the artist's site. Both are used as
// asked, so a reword here would be this project paraphrasing someone else's
// request about their own work.
func TestTheMusicAttributionMatchesThePack(t *testing.T) {
	prose := creditsProse()
	if !strings.Contains(prose, "CAPTNPHANTOM") {
		t.Error("the credits do not carry the artist name the pack asks for")
	}
	linked := strings.Contains(prose, "captnphantom.itch.io") ||
		strings.Contains(prose, "captnphantom.itch.io/liminal-worlds") ||
		strings.Contains(prose, "captnphantom.itch.io/underground-techno-pack")
	if !linked {
		t.Error("the credits carry neither of the two links of the packs used")
	}
}

// TestTheCreditsNameTheFontsTheBuildActuallyLoads: a typeface credited but not
// used, or used but not credited, are both wrong and neither shows on screen.
// resources.initFonts loads exactly these two.
func TestTheCreditsNameTheFontsTheBuildActuallyLoads(t *testing.T) {
	prose := creditsProse()
	for _, font := range []string{"Source Code Pro", "Inversionz"} {
		if !strings.Contains(prose, font) {
			t.Errorf("%s is loaded by the build and not credited", font)
		}
		if _, err := os.Stat("../resources/fonts/" + strings.ReplaceAll(font, " ", "") + ".ttf"); err != nil {
			// Source Code Pro ships as SourceCodePro-Regular.ttf.
			if _, err := os.Stat("../resources/fonts/" + strings.ReplaceAll(font, " ", "") + "-Regular.ttf"); err != nil {
				t.Errorf("%s is credited and no such font file is in the repository: %v", font, err)
			}
		}
	}
}

// TestTheCreditsAreTheirOwnDocument: ABOUT and RULES are two entries on the
// menu and must not be two names for one page. The credits also carry no
// header, so nothing is drawn above the text.
func TestTheCreditsAreTheirOwnDocument(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	about, rules := NewAboutScreen(), NewRulesScreen()
	if about.title == rules.title {
		t.Errorf("both documents are titled %q", about.title)
	}
	if about.title != "ABOUT" || rules.title != "RULES" {
		t.Errorf("the documents are titled %q and %q", about.title, rules.title)
	}
	if len(about.tabs) != 1 {
		t.Errorf("the credits are split across %d tabs", len(about.tabs))
	}
	if len(rules.tabs) < 2 {
		t.Errorf("the rules are down to %d tabs, so the strip stops being drawn", len(rules.tabs))
	}
	// A one-tab document draws no strip, and its content starts where the
	// strip would have been.
	if about.stripHeight() != 0 {
		t.Errorf("the credits reserve %dpx for a strip of one tab", about.stripHeight())
	}
	if rules.stripHeight() == 0 {
		t.Error("the rules reserve no room for their tab strip")
	}
	if len(about.header) != 0 {
		t.Errorf("the credits carry %d header blocks, which are drawn above the text on every tab", len(about.header))
	}
}

// TestBothDocumentsFitTheirPanel: the ABOUT text is wrapped by the same
// helper the rules use, so a line too long for the panel is cut off at the
// edge rather than wrapped.
func TestBothDocumentsFitTheirPanel(t *testing.T) {
	loadKeyGlobals(t)
	r.UseDirAssets("..")
	r.Init()

	// The width the DRAWING wraps at, which is the panel itself: rulesText
	// passes rulesPanelW to wrapParagraph, so measuring against anything
	// narrower would be testing a layout the screen does not use.
	width := rulesPanelW
	for _, b := range creditsBlocks() {
		switch v := b.(type) {
		case rulesHeading:
			if got := boundString(r.FontSourceCodePro12, v.text).Dx(); got > width {
				t.Errorf("the heading %q measures %dpx in a %dpx panel", v.text, got, width)
			}
		case rulesText:
			for _, line := range wrapParagraph(v.text, width) {
				if got := boundString(r.FontSourceCodePro10, line).Dx(); got > width {
					t.Errorf("a wrapped line measures %dpx in a %dpx panel: %q", got, width, line)
				}
			}
		}
	}
}

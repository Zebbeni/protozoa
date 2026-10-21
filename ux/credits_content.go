package ux

// The ABOUT screen: who made what, and the terms the borrowed work came under.
//
// Every line here is checkable against something in the repository — the music
// pack's own licence text, the name table inside each font file, the module
// list in go.mod. A credits page that cannot be checked is the one place in a
// project where a guess becomes a claim about somebody else's work, so nothing
// is written here that was not read off the thing it credits.
//
// The music bundle is CC0 and asks for no attribution at all, but names the
// credit it would like: the artist name "Abstraction" and a link to the pack
// or to abstractionmusic.com. Both are used as asked rather than reworded.
//
// Inversionz is the one typeface here that was BOUGHT. Its own name table
// says "donationware", which is how it is listed on dafont and is not what
// this project uses it under: the Hawtpixel Standard licence grants full
// commercial use of one Darrell Flood font, worldwide and in perpetuity, and
// permits embedding it in an app. A font file's metadata describes the
// channel it was published through, not the terms a particular user holds.

// creditsBlocks is the whole ABOUT document, in reading order.
func creditsBlocks() []rulesBlock {
	return []rulesBlock{
		rulesText{"Protozoa is a simulation of evolving organisms, made by Andrew Albers."},

		rulesHeading{"ART"},
		rulesText{"All sprites, animation and interface art by Andrew Albers, drawn in Aseprite " +
			"and exported to the per-resolution sheets the game loads."},

		rulesHeading{"MUSIC"},
		rulesText{"Music by Abstraction (abstractionmusic.com)."},
		rulesText{"The pieces are Reverence and Forget About Us, from the Music Loop Bundle at " +
			"tallbeard.itch.io/music-loop-bundle, released under CC0. The menus play one and " +
			"the replay viewer alternates between two takes of the other."},

		rulesHeading{"TYPEFACES"},
		rulesText{"Source Code Pro by Paul D. Hunt and Teo Tuominen for Adobe, under the SIL " +
			"Open Font License 1.1. Inversionz by Darrell Flood, used under a Hawtpixel " +
			"Standard commercial licence."},

		rulesHeading{"BUILT WITH"},
		rulesText{"Ebitengine, the Go game engine that draws every frame and plays the music. " +
			"go-colorful for blending the colours the grid is tinted with. golang.org/x/image " +
			"for the font rendering. atotto/clipboard for copying a seed out of the viewer."},

		rulesHeading{"SOURCE"},
		rulesText{"Protozoa is written in Go and its source is at github.com/Zebbeni/protozoa."},
	}
}

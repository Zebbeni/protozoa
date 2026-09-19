package resources

import (
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"github.com/Zebbeni/protozoa/animation"
	"github.com/Zebbeni/protozoa/config"
	"github.com/Zebbeni/protozoa/physiology"
)

// animationFileName is the filename stem used for per-action spritesheet PNGs
// (e.g. "small_move.png"). Kept in sync with resources/gen/main.go.
var animationFileName = map[animation.Animation]string{
	animation.AnimIdle:      "idle",
	animation.AnimMove:      "move",
	animation.AnimBlocked:   "blocked",
	animation.AnimTurnLeft:  "turn_left",
	animation.AnimTurnRight: "turn_right",
	animation.AnimAttack:    "attack",
	animation.AnimEat:       "eat",
	animation.AnimEatFail:   "eatfail",
	animation.AnimChemo:     "chemo",
	animation.AnimChemoFail: "chemofail",
	animation.AnimDie:       "die",
	animation.AnimDig:       "dig",
}

// organismRoleName maps each organism role to the filename stem used by the
// per-action spritesheets (e.g. "small", "medium", "large").
var organismRoleName = map[ImageRole]string{
	RoleOrganismTiny:   "tiny",
	RoleOrganismSmall:  "small",
	RoleOrganismMedium: "medium",
	RoleOrganismLarge:  "large",
}

const (
	dpi = 72
)

// ImageRole identifies the role of a sprite image.
//
// New roles are appended at the end so previously-serialized integer
// values stay stable (not that we currently serialize roles — but it's a
// good habit given how cheap it is).
type ImageRole int

const (
	RoleOrganismSmall ImageRole = iota
	RoleOrganismMedium
	RoleOrganismLarge
	RoleFoodSmall
	RoleFoodMedium
	RoleFoodLarge
	// Wall sprites by strength tier — see ux/grid.go's
	// wallRoleForStrength for the strength → role mapping. Each role
	// holds multiple Layer slots: LayerWallBase (always present) plus
	// LayerWallUp / LayerWallDown / LayerWallLeft / LayerWallRight
	// directional connectors, drawn on top of the base when the
	// cardinal neighbour also holds a wall.
	RoleWallWeak
	RoleWallMedium
	RoleWallStrong
	// Appended, not slotted in beside their siblings: the iota values
	// are stable by convention (see above), and the natural reading
	// order of each group is in the bucketing helpers, not here.
	RoleOrganismTiny
	RoleFoodTiny
	RoleWallGiant
)

// FrameSet holds one slice of per-frame sprites per Animation kind.
//
// Frame count by the sprite set's resolution: 4x4 and 8x8 hold 2,
// 16x16 holds 4. Non-organism roles (food,
// walls) only populate AnimIdle and only ever need one frame.
type FrameSet map[animation.Animation][]*ebiten.Image

// Layer identifies one drawable spritesheet layer for an organism role,
// matching the Aseprite layer names that the export lua script writes
// PNG files for. Low-res organism sprites (4x4 / 8x8) only populate
// LayerBody; high-res organism sprites (16x16+) populate a single body
// variant (picked from the Defense score) and additional overlay layers
// (motor, mouth and sensor, from physiology.Appearance).
//
// Food and wall roles also store their (single) sprite under LayerBody
// so every (role, layer) lookup goes through the same code path.
type Layer int

const (
	// LayerBody is the single-layer slot used by low-res organism
	// sprites, food, and walls. High-res organism sprites do not
	// populate LayerBody; they use the body-variant + overlay layers
	// below.
	LayerBody Layer = iota

	// Body variants — mutually exclusive. One is always drawn first
	// when rendering a high-res organism; the choice comes from
	// physiology.Appearance's Body class, derived from the Defense
	// score (basic / shell / spikes).
	LayerBodyBasic
	LayerBodyShell
	LayerBodySpikes

	// Appearance overlays — additive. Drawn around the body in the
	// canonical order below; a high-res organism draws at most one
	// motor (pili / flagella), one sensor (antennae / feelers /
	// tasters) and one mouth (teeth / fangs / tusks), per
	// physiology.Appearance.
	LayerPili
	LayerFlagella
	// Iota slot retained for back-compat with any code that bound an
	// integer Layer ID directly to art. New code should ignore it.
	_
	LayerAntennae
	LayerFeelers
	LayerTasters
	LayerTeeth
	LayerFangs
	LayerTusks

	// Wall layers. Authored as separate Aseprite layers within the
	// same wall slice so each strength tier exports its own base +
	// directional connector pieces. The grid renderer draws
	// LayerWallBase for every wall cell and additionally layers
	// LayerWallUp / Down / Left / Right when the corresponding
	// cardinal neighbour also holds a wall, producing a single
	// connected visual when walls cluster.
	LayerWallBase
	LayerWallUp
	LayerWallDown
	LayerWallLeft
	LayerWallRight
)

// layerFilenamePrefix maps each Layer to the filename prefix the lua
// export script writes for it, e.g. body_basic_small_attack.png →
// {LayerBodyBasic, RoleOrganismSmall, AnimAttack}. LayerBody has no
// prefix because the low-res "only one applicable layer" naming rule
// in the export script drops the layer prefix entirely (e.g.
// small_attack.png).
var layerFilenamePrefix = map[Layer]string{
	LayerBody:       "",
	LayerBodyBasic:  "body_basic",
	LayerBodyShell:  "body_shell",
	LayerBodySpikes: "body_spikes",
	LayerPili:       "pili",
	LayerFlagella:   "flagella",
	LayerAntennae:   "antennae",
	LayerFeelers:    "feelers",
	LayerTasters:    "tasters",
	LayerTeeth:      "teeth",
	LayerFangs:      "fangs",
	LayerTusks:      "tusks",
	LayerWallBase:   "wall_base",
	LayerWallUp:     "wall_up",
	LayerWallDown:   "wall_down",
	LayerWallLeft:   "wall_left",
	LayerWallRight:  "wall_right",
}

// UsesPrimaryColor reports whether the given Layer should be tinted
// with the organism's primary colour (true) or secondary colour
// (false). Body silhouettes always use primary; among overlays, the
// sensors group (antennae / feelers / tasters) also uses primary so
// the organism reads as one colour-coordinated creature with the
// motor / teeth overlays providing the contrasting accent. Used by
// every renderer that walks OrganismLayersFor and applies per-layer
// tints (grid, panel portrait, animation test).
func UsesPrimaryColor(layer Layer) bool {
	switch layer {
	case LayerBody,
		LayerBodyBasic, LayerBodyShell, LayerBodySpikes,
		LayerAntennae, LayerFeelers, LayerTasters:
		return true
	default:
		return false
	}
}

// OrganismLayersFor returns the ordered list of layers a high-res
// renderer should draw for an organism with the given appearance, from
// bottom to top:
//  1. Motor overlay   (Pili / Flagella)
//  2. Mouth overlay   (Teeth / Fangs / Tusks)
//  3. Body silhouette (basic / Shell / Spikes)
//  4. Sensor overlay  (Antennae / Feelers / Tasters)
//
// Locomotion sits underneath so the body covers wherever the limbs meet
// the silhouette; the mouth sits under the body so teeth read as
// protruding rather than floating in front of it; sensors sit on top
// because antennae and feelers are meant to be visible above every
// body variant.
//
// The appearance is derived from ability scores and decision-tree
// conditions (see physiology.AppearanceFor) and precomputed per
// organism, so this is pure lookup — no per-frame derivation.
func OrganismLayersFor(app physiology.Appearance) []Layer {
	out := make([]Layer, 0, 4)
	if layer, ok := motorLayer[app.Motor]; ok {
		out = append(out, layer)
	}
	if layer, ok := mouthLayer[app.Mouth]; ok {
		out = append(out, layer)
	}
	out = append(out, bodyLayer[app.Body])
	if layer, ok := sensorLayer[app.Sensor]; ok {
		out = append(out, layer)
	}
	return out
}

// Layer lookups per appearance class. The "none" classes are simply
// absent from the overlay maps, so an organism that hasn't specialised
// enough to earn an overlay draws without one.
var (
	bodyLayer = map[physiology.BodyClass]Layer{
		physiology.BodyBasic:  LayerBodyBasic,
		physiology.BodyShell:  LayerBodyShell,
		physiology.BodySpikes: LayerBodySpikes,
	}
	motorLayer = map[physiology.MotorClass]Layer{
		physiology.MotorPili:     LayerPili,
		physiology.MotorFlagella: LayerFlagella,
	}
	mouthLayer = map[physiology.MouthClass]Layer{
		physiology.MouthTeeth: LayerTeeth,
		physiology.MouthFangs: LayerFangs,
		physiology.MouthTusks: LayerTusks,
	}
	sensorLayer = map[physiology.SensorClass]Layer{
		physiology.SensorAntennae: LayerAntennae,
		physiology.SensorFeelers:  LayerFeelers,
		physiology.SensorTasters:  LayerTasters,
	}
)

// LayeredFrames holds per-layer FrameSets for one role. Low-res roles
// (organisms at 4x4 / 8x8, plus food and walls at every resolution)
// only populate LayerBody. High-res organism roles populate every
// body variant + overlay Layer for which a PNG exists on disk; layers
// without a PNG are simply absent from the map (the renderer skips
// them via SpriteLayer's nil return).
type LayeredFrames map[Layer]FrameSet

var (
	FontInversionz40    font.Face
	FontSourceCodePro12 font.Face
	FontSourceCodePro10 font.Face
	FontSourceCodePro8  font.Face

	PlayButton  *ebiten.Image
	PauseButton *ebiten.Image

	// Images is the active sprite set (set by SelectZoom), keyed by role.
	Images map[ImageRole]LayeredFrames
)

// ZoomImages holds sprite sets for the 3 native sprite sizes
// (0=4x4, 1=8x8, 2=16x16). 16x16 is the highest resolution the
// project authors; the camera upscales it for the larger zooms.
var ZoomImages [3]map[ImageRole]LayeredFrames

// currentZoom tracks which ZoomImages entry is active. Persisted across
// calls to initImages so reloads (e.g. theme toggling) keep pointing at
// the sprite set the camera is currently using, instead of snapping back
// to zero.
var currentZoom int

// ZoomHighRes is the 16x16 set: the highest resolution the project
// authors, and the only one with layered overlays — at 4x4 and 8x8 every
// layer lookup misses and rendering falls back to a bare body sprite.
// Anything drawing an organism away from the grid (the designer's
// portrait) wants this one rather than whatever zoom the camera last
// left selected.
const ZoomHighRes = len(ZoomImages) - 1

// CurrentZoom reports which set is active, so a caller that needs a
// specific one can put it back afterwards.
func CurrentZoom() int { return currentZoom }

func Init() {
	initFonts()
	initImages()
}

// SelectZoom sets the active image set to the given sprite-set index (0-2).
// The level is also remembered so subsequent reloads (ReloadImages) restore
// it instead of snapping back to zero.
func SelectZoom(level int) {
	if level < 0 || level >= len(ZoomImages) {
		return
	}
	currentZoom = level
	Images = ZoomImages[level]
}

// ReloadImages rebuilds every sprite FrameSet from disk. Used by the
// animation-test hot-reload path so sprite-sheet edits show up without
// restarting the app. Fonts aren't touched.
func ReloadImages() {
	initImages()
}

// Sprite returns the default sprite image for a given role/animation/
// frame — the LayerBody entry at low-res, falling back to LayerBodyBasic
// at high-res (where physiology-driven layer compositing normally drives
// rendering). Defaults to AnimIdle when the requested animation has no
// frames and cycles within the available frames when frame >= len. Used
// by call sites that don't carry a physiology bitmask (animation test
// screen, food / wall draws via static frames, fallback callers).
func Sprite(role ImageRole, anim animation.Animation, frame int) *ebiten.Image {
	return spriteFromSet(Images, role, defaultLayer(Images, role), anim, frame)
}

// SpriteAtZoom is like Sprite but reads from a specific sprite-set
// level (0=4x4, 1=8x8, 2=16x16) regardless of which zoom is currently
// active. Used by the panel's organism portrait, which always renders
// from the 16x16 set so the sprite reads at 4x scale.
func SpriteAtZoom(level int, role ImageRole, anim animation.Animation, frame int) *ebiten.Image {
	if level < 0 || level >= len(ZoomImages) {
		return nil
	}
	images := ZoomImages[level]
	return spriteFromSet(images, role, defaultLayer(images, role), anim, frame)
}

// SpriteLayer returns the sprite image for a specific role/layer/anim/
// frame in the active zoom set, or nil if no PNG was loaded for that
// layer (which is the normal case for layers an organism's physiology
// doesn't unlock). Used by the high-res organism renderer to stamp one
// layer at a time per-organism based on its physiology.
func SpriteLayer(role ImageRole, layer Layer, anim animation.Animation, frame int) *ebiten.Image {
	return spriteFromSet(Images, role, layer, anim, frame)
}

// SpriteLayerAtZoom is like SpriteLayer but reads from a specific
// sprite-set level regardless of which zoom is currently active. Used
// by the panel's selected-organism portrait, which always renders from
// the 16x16 set.
func SpriteLayerAtZoom(level int, role ImageRole, layer Layer, anim animation.Animation, frame int) *ebiten.Image {
	if level < 0 || level >= len(ZoomImages) {
		return nil
	}
	return spriteFromSet(ZoomImages[level], role, layer, anim, frame)
}

// defaultLayer picks the layer Sprite() should read from for the given
// role: LayerBody when populated (low-res organism, food, wall), else
// LayerBodyBasic (high-res organism fallback when no physiology is
// available at the call site).
func defaultLayer(images map[ImageRole]LayeredFrames, role ImageRole) Layer {
	if layers, ok := images[role]; ok {
		if _, has := layers[LayerBody]; has {
			return LayerBody
		}
	}
	return LayerBodyBasic
}

func spriteFromSet(images map[ImageRole]LayeredFrames, role ImageRole, layer Layer, anim animation.Animation, frame int) *ebiten.Image {
	layers, ok := images[role]
	if !ok {
		return nil
	}
	set, ok := layers[layer]
	if !ok {
		return nil
	}
	frames, ok := set[anim]
	if !ok || len(frames) == 0 {
		frames = set[animation.AnimIdle]
	}
	if len(frames) == 0 {
		return nil
	}
	return frames[frame%len(frames)]
}

func initFonts() {
	inversionz := loadFont("resources/fonts/Inversionz.ttf")
	FontInversionz40 = fontFace(inversionz, 40)
	sourceCode := loadFont("resources/fonts/SourceCodePro-Regular.ttf")
	FontSourceCodePro12 = fontFace(sourceCode, 12)
	FontSourceCodePro10 = fontFace(sourceCode, 10)
	FontSourceCodePro8 = fontFace(sourceCode, 8)
}

func initImages() {
	PlayButton = loadImage("resources/images/play_button.png")
	PauseButton = loadImage("resources/images/pause_button.png")

	dirs := [3]string{"4x4", "8x8", "16x16"}
	sizes := [3]int{4, 8, 16}

	// Both themes draw the same sprite set. The light theme lifts the
	// sprites' greys toward white as they load (see loadSprite), so dark
	// outlines and shading don't read as near-black against the light
	// background. ReloadImages is called by cycleTheme so switching
	// themes at runtime reloads with or without the lift.
	themeDir := spriteDir

	for i, dir := range dirs {
		size := sizes[i]
		path := "resources/images/" + themeDir + "/" + dir + "/"

		// Base single-frame sprites per organism role. Used as a fallback
		// when a per-action sheet is missing. Loaded from disk if the
		// corresponding square_<role>.png exists; otherwise synthesised
		// programmatically so the loader never crashes on missing art.
		// Quarters of the cell, matching the four size buckets.
		baseTiny := loadOrGenerateFilled(path+"square_tiny.png", size, max(1, size/4))
		baseSmall := loadOrGenerateFilled(path+"square_small.png", size, max(1, size/2))
		baseMedium := loadOrGenerateFilled(path+"square_medium.png", size, max(2, size*3/4))
		baseLarge := loadOrGenerateFilled(path+"square_large.png", size, size)

		// Walls are authored as four strength tiers per resolution;
		// the grid renderer picks between them based on the wall's
		// current strength as a fraction of MaxWallStrength. Each
		// tier falls back to a generated box-outline at the right
		// pixel size when its PNG hasn't been drawn yet.
		// Walls are authored as one slice per strength tier (weak /
		// medium / strong / giant) with separate Aseprite layers per piece
		// (wall_base + four directional connectors). The lua export
		// writes each layer to wall_<layer>_<strength>.png so the
		// loader can pick them up independently.
		wallWeak := loadWallLayers(path, "weak", size)
		wallMedium := loadWallLayers(path, "medium", size)
		wallStrong := loadWallLayers(path, "strong", size)
		wallGiant := loadWallLayers(path, "giant", size)
		// Food is authored as four size tiers per resolution; the grid
		// renderer picks between them based on the food item's value as a
		// fraction of MaxFoodValue. Circle fallbacks match the organism
		// size-tier fallbacks so missing art degrades gracefully.
		foodTiny := loadOrGenerateCircle(path+"food_tiny.png", size, max(1, size/4))
		foodSmall := loadOrGenerateCircle(path+"food_small.png", size, max(1, size/2))
		foodMedium := loadOrGenerateCircle(path+"food_medium.png", size, max(2, size*3/4))
		foodLarge := loadOrGenerateCircle(path+"food_large.png", size, size)

		// Frames per cycle by resolution: 4x4 → 2, 8x8 → 2, 16x16 → 4.
		// Four is the ceiling — higher zoom levels upscale the 16x16 art,
		// buying on-screen size, not more animation steps — and two is the
		// floor, so the smallest sprites animate like the rest rather than
		// being the one set that can only hold a pose.
		// Must match zoomSpriteFrameCounts in ux/camera.go.
		//
		// Art that hasn't caught up is safe: a sheet narrower than the
		// frame count is repeated rather than sliced (see loadAnimSheets),
		// so a 1-frame 4x4 strip renders exactly as it did before.
		orgFrames := size / 4
		if orgFrames < 2 {
			orgFrames = 2
		} else if orgFrames > 4 {
			orgFrames = 4
		}

		bases := map[ImageRole]*ebiten.Image{
			RoleOrganismTiny:   baseTiny,
			RoleOrganismSmall:  baseSmall,
			RoleOrganismMedium: baseMedium,
			RoleOrganismLarge:  baseLarge,
		}

		ZoomImages[i] = map[ImageRole]LayeredFrames{
			RoleFoodTiny:   {LayerBody: staticFrames(foodTiny)},
			RoleFoodSmall:  {LayerBody: staticFrames(foodSmall)},
			RoleFoodMedium: {LayerBody: staticFrames(foodMedium)},
			RoleFoodLarge:  {LayerBody: staticFrames(foodLarge)},
			RoleWallWeak:   wallWeak,
			RoleWallMedium: wallMedium,
			RoleWallStrong: wallStrong,
			RoleWallGiant:  wallGiant,
		}
		// Low-res (4x4 / 8x8) authors a single unvaried `body` layer per
		// organism role; high-res (16x16+) authors mutually-exclusive
		// body variants and additive feature overlays. The cutoff is
		// 16 because that's where Aseprite art has enough pixels for
		// the silhouette differences to read.
		highRes := size >= 16
		for role, base := range bases {
			ZoomImages[i][role] = loadOrganismLayers(path, role, base, size, orgFrames, highRes)
		}
	}

	// Preserve whichever zoom was active before a reload — on first init
	// currentZoom is zero so this still picks the 4x4 set by default.
	SelectZoom(currentZoom)
}

// loadOrGenerateFilled returns loadImage(fullPath) if the file exists,
// otherwise a programmatically-generated filled-square fallback. Used
// per-file so a missing base PNG doesn't crash init — the loader falls
// back gracefully for sizes the user hasn't drawn art for yet.
func loadOrGenerateFilled(fullPath string, totalSize, innerSize int) *ebiten.Image {
	if assetExists(fullPath) {
		return loadSprite(fullPath)
	}
	return generateFilledImage(totalSize, innerSize)
}

// loadOrGenerateCircle is the same pattern for circle-shaped fallbacks
// (food).
func loadOrGenerateCircle(fullPath string, totalSize, diameter int) *ebiten.Image {
	if assetExists(fullPath) {
		return loadSprite(fullPath)
	}
	return generateCircle(totalSize, diameter)
}

// loadOrNil returns the loaded image when the asset exists, otherwise
// nil. Used for sprite slots that should silently no-op when the
// artist hasn't drawn them yet (wall connectors); spriteFromSet's
// empty-frames-returns-nil contract then lets renderers skip them.
func loadOrNil(fullPath string) *ebiten.Image {
	if assetExists(fullPath) {
		return loadSprite(fullPath)
	}
	return nil
}

// loadWallLayers loads one strength tier's full layered set: the
// base sprite plus the four directional connectors. The base falls
// back to a generated box so an unauthored wall is still visible.
// Connectors load only when the corresponding PNG exists — a missing
// connector means the renderer just doesn't draw that overlay, so
// walls without connector art appear as isolated piles regardless of
// their neighbours.
//
// Filename convention mirrors the lua export: wall_<layer>_<strength>.png
// (e.g. wall_base_medium.png, wall_up_strong.png).
func loadWallLayers(path, strength string, size int) LayeredFrames {
	out := make(LayeredFrames)
	out[LayerWallBase] = staticFrames(loadOrGenerateBox(path+"wall_base_"+strength+".png", size))
	for _, layer := range []Layer{LayerWallUp, LayerWallDown, LayerWallLeft, LayerWallRight} {
		fname := path + layerFilenamePrefix[layer] + "_" + strength + ".png"
		if img := loadOrNil(fname); img != nil {
			out[layer] = staticFrames(img)
		}
	}
	return out
}

// loadOrGenerateBox is the same pattern for box-outline fallbacks
// (walls). All three wall strength tiers fall back to the same
// generated box at the right pixel size when the PNG hasn't been
// drawn yet; the renderer's strength → tier mapping still picks
// between them so the distinct sprites can land later without code
// changes.
func loadOrGenerateBox(fullPath string, totalSize int) *ebiten.Image {
	if assetExists(fullPath) {
		return loadSprite(fullPath)
	}
	return generateBoxImage(totalSize)
}

// loadOrganismLayers builds the LayeredFrames for one organism role at
// one zoom. At low-res (highRes == false) the artist authors a single
// `body` layer per role + action and the export script writes filenames
// without a layer prefix (e.g. small_move.png) — that single sheet is
// loaded into LayerBody. At high-res the artist authors one PNG per
// (layer, role, action) triple (e.g. body_basic_small_move.png,
// pili_small_move.png) and each Layer with a PNG on disk gets its
// own FrameSet; layers without PNGs are simply omitted so the renderer
// only stamps art the artist drew. The base role sprite (or its
// generated fallback) backstops the body slot when no per-action sheet
// is present, so missing art degrades to a flat shape rather than to
// nothing.
func loadOrganismLayers(path string, role ImageRole, base *ebiten.Image, frameSize, orgFrames int, highRes bool) LayeredFrames {
	out := make(LayeredFrames)
	if !highRes {
		out[LayerBody] = loadAnimSheets(path, "", role, base, frameSize, orgFrames)
		return out
	}
	// High-res: try every Layer except LayerBody (which is the low-res
	// single-layer slot and isn't authored at this resolution). Layers
	// with no PNGs at all are skipped; LayerBodyBasic in particular is
	// also backstopped with the base sprite so missing default-body
	// art still draws something rather than nothing.
	for layer, prefix := range layerFilenamePrefix {
		if layer == LayerBody {
			continue
		}
		var layerBase *ebiten.Image
		if layer == LayerBodyBasic {
			layerBase = base
		}
		set := loadAnimSheets(path, prefix, role, layerBase, frameSize, orgFrames)
		if set != nil {
			out[layer] = set
		}
	}
	return out
}

// loadAnimSheets builds a FrameSet for one (layer-prefix, role) pair.
// Filenames are "<prefix>_<role>_<action>.png" when prefix is non-empty,
// or "<role>_<action>.png" when prefix is "" (the low-res single-layer
// naming rule). Sheets narrower than orgFrames cells are treated as
// static and repeated. When fallback is non-nil, animations whose sheet
// is missing fall back to repeating it (used so the default body layer
// still renders when the artist hasn't drawn a per-action sheet yet).
// When fallback is nil, missing-sheet animations are simply absent from
// the returned FrameSet — and if no animation has a sheet at all, the
// function returns nil so the caller can omit the layer entirely.
func loadAnimSheets(path, prefix string, role ImageRole, fallback *ebiten.Image, frameSize, orgFrames int) FrameSet {
	roleName := organismRoleName[role]
	if roleName == "" {
		return nil
	}
	set := make(FrameSet, len(animation.AllAnimations))
	loaded := false
	for _, anim := range animation.AllAnimations {
		animName := animationFileName[anim]
		if animName == "" {
			continue
		}
		var sheetPath string
		if prefix == "" {
			sheetPath = path + roleName + "_" + animName + ".png"
		} else {
			sheetPath = path + prefix + "_" + roleName + "_" + animName + ".png"
		}
		if !assetExists(sheetPath) {
			if fallback != nil {
				set[anim] = repeatSprite(fallback, orgFrames)
				loaded = true
			}
			continue
		}
		sheet := loadSprite(sheetPath)
		if sheet.Bounds().Dx() < orgFrames*frameSize {
			set[anim] = repeatSprite(sheet, orgFrames)
		} else {
			set[anim] = sliceSheet(sheet, orgFrames)
		}
		loaded = true
	}
	if !loaded {
		return nil
	}
	return set
}

// sliceSheet splits a horizontal spritesheet into per-frame SubImage
// views. Frame width = total width / frames, height = full sheet
// height (so multi-cell vertical sprites are preserved per frame).
func sliceSheet(sheet *ebiten.Image, frames int) []*ebiten.Image {
	if frames < 1 {
		frames = 1
	}
	b := sheet.Bounds()
	frameW := b.Dx() / frames
	frameH := b.Dy()
	out := make([]*ebiten.Image, frames)
	for i := 0; i < frames; i++ {
		rect := image.Rect(i*frameW, 0, (i+1)*frameW, frameH)
		out[i] = sheet.SubImage(rect).(*ebiten.Image)
	}
	return out
}

// repeatSprite returns a frames-long slice pointing at the same base
// sprite. Used as a fallback when a per-action spritesheet isn't present
// on disk.
func repeatSprite(base *ebiten.Image, frames int) []*ebiten.Image {
	if frames < 1 {
		frames = 1
	}
	out := make([]*ebiten.Image, frames)
	for i := range out {
		out[i] = base
	}
	return out
}

// staticFrames produces a FrameSet that only has a single AnimIdle entry,
// suitable for non-organism roles (food, walls) that don't animate.
func staticFrames(img *ebiten.Image) FrameSet {
	return FrameSet{
		animation.AnimIdle: {img},
	}
}

func generateFilledImage(totalSize, innerSize int) *ebiten.Image {
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, totalSize, totalSize))
	offset := (totalSize - innerSize) / 2
	for y := offset; y < offset+innerSize; y++ {
		for x := offset; x < offset+innerSize; x++ {
			img.Set(x, y, white)
		}
	}
	return ebiten.NewImageFromImage(img)
}

func generateCircle(totalSize, diameter int) *ebiten.Image {
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, totalSize, totalSize))
	cx, cy := float64(totalSize)/2.0, float64(totalSize)/2.0
	r := float64(diameter) / 2.0
	for y := 0; y < totalSize; y++ {
		for x := 0; x < totalSize; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, white)
			}
		}
	}
	return ebiten.NewImageFromImage(img)
}

func generateBoxImage(size int) *ebiten.Image {
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for i := 0; i < size; i++ {
		img.Set(i, 0, white)
		img.Set(i, size-1, white)
		img.Set(0, i, white)
		img.Set(size-1, i, white)
	}
	return ebiten.NewImageFromImage(img)
}

// spriteDir is the image directory every theme's grid sprites load from.
const spriteDir = "grid_dark"

// lightThemeSpriteShadow, lightThemeSpriteGamma and lightThemeSpriteDarken
// set the tone curve the light theme applies to sprite greys. Each opaque
// pixel's channels (in [0, 1]) go through two steps:
//
//	lifted = shadow + (1 - shadow) × value^gamma
//	final  = lifted - darken × (1 - lifted)²
//
// Sprites are authored for the dark background, where dark outlines and
// shading read well; multiplied by an organism's colour they come out
// close to black against the light theme's background. The shadow point
// lifts the darkest greys only so far, keeping outlines clearly darker
// than the fill, and a gamma below 1 brightens midtones and fills more
// than the outlines, so the lighter sprite keeps its contrast instead of
// flattening toward white. The darken step then deepens the darks alone:
// its pull shrinks with the square of the distance from white, so lights
// barely move while outlines and deep shading get noticeably darker. Tune by eye with the animation test screen
// (t toggles theme, r reloads).
const (
	lightThemeSpriteShadow = 0.25
	lightThemeSpriteGamma  = 0.65
	lightThemeSpriteDarken = 0.3
)

// loadSprite loads a grid sprite, applying the light theme's tone curve.
//
// Done once at load rather than per draw: tinting is a per-vertex colour
// scale that ebiten batches, but a tone curve needs a colour matrix or
// shader, which would break batching and cost a draw call per organism.
func loadSprite(path string) *ebiten.Image {
	img := decodeImage(path)
	if config.IsLightTheme() {
		img = toneImage(img, lightThemeSpriteShadow, lightThemeSpriteGamma, lightThemeSpriteDarken)
	}
	return ebiten.NewImageFromImage(img)
}

// toneImage returns a copy of img with each opaque pixel's colour channels
// remapped by the light theme's tone curve (channels in [0, 1],
// non-premultiplied):
//
//	lifted = shadow + (1 - shadow) × c^gamma
//	c'     = lifted - darken × (1 - lifted)², floored at 0
//
// White stays white and the order of greys is preserved: the darken step
// is increasing in lifted for any darken ≥ 0. Strong darkening does push
// the darkest greys below zero, where the floor merges them into black. Transparency is unchanged, so silhouettes
// and soft edges keep their shape.
func toneImage(img image.Image, shadow, gamma, darken float64) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	shadow = min(1, max(0, shadow))
	// Past 0.5 more of the darkest greys would floor to black and lose
	// their shading; cap it there.
	darken = min(0.5, max(0, darken))
	if gamma <= 0 {
		gamma = 1
	}
	// Only 256 possible channel values, so build the curve once.
	var curve [256]uint8
	for v := range curve {
		lifted := shadow + (1-shadow)*math.Pow(float64(v)/255, gamma)
		mapped := max(0, lifted-darken*(1-lifted)*(1-lifted))
		curve[v] = uint8(math.Round(255 * mapped))
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A != 0 {
				c.R, c.G, c.B = curve[c.R], curve[c.G], curve[c.B]
			}
			out.SetNRGBA(x-b.Min.X, y-b.Min.Y, c)
		}
	}
	return out
}

func loadImage(path string) *ebiten.Image {
	return ebiten.NewImageFromImage(decodeImage(path))
}

// decodeImage reads and decodes a PNG from the asset filesystem.
func decodeImage(path string) image.Image {
	if assetsFS == nil {
		log.Fatalf("resources: asset FS not initialised; UseEmbeddedAssets must be called before loading %q", path)
	}
	reader, err := assetsFS.Open(path)
	if err != nil {
		log.Fatalf("resources: failed to open %q: %v", path, err)
	}
	defer reader.Close()
	img, err := png.Decode(reader)
	if err != nil {
		log.Fatalf("resources: failed to decode %q: %v", path, err)
	}
	return img
}

func loadFont(path string) *opentype.Font {
	if assetsFS == nil {
		log.Fatalf("resources: asset FS not initialised; UseEmbeddedAssets must be called before loading %q", path)
	}
	data, err := fs.ReadFile(assetsFS, path)
	if err != nil {
		log.Fatalf("resources: failed to read font %q: %v", path, err)
	}
	tt, err := opentype.Parse(data)
	if err != nil {
		log.Fatalf("resources: failed to parse font %q: %v", path, err)
	}
	return tt
}

func fontFace(openFont *opentype.Font, size float64) font.Face {
	face, err := opentype.NewFace(openFont, &opentype.FaceOptions{
		Size:    size,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	return face
}

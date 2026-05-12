package resources

import (
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"

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
}

// organismRoleName maps each organism role to the filename stem used by the
// per-action spritesheets (e.g. "small", "medium", "large").
var organismRoleName = map[ImageRole]string{
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
	// wallRoleForStrength for the strength → role mapping.
	RoleWallWeak
	RoleWallMedium
	RoleWallStrong
)

// FrameSet holds one slice of per-frame sprites per Animation kind.
//
// Frame count scales with the sprite set's resolution: 4x4 holds 1 frame,
// 8x8 holds 2, 16x16 holds 4 (resolution / 4). Non-organism roles (food,
// walls) only populate AnimIdle and only ever need one frame.
type FrameSet map[animation.Animation][]*ebiten.Image

// Layer identifies one drawable spritesheet layer for an organism role,
// matching the Aseprite layer names that the export lua script writes
// PNG files for. Low-res organism sprites (4x4 / 8x8) only populate
// LayerBody; high-res organism sprites (16x16+) populate a single body
// variant (picked from the defense tree) and additional overlay layers
// (one per non-defense feature tree's deepest-held feature).
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
	// when rendering a high-res organism; the choice is driven by
	// the defense tree (Shell/Spikes/Camouflage), with FeatNone in
	// that tree mapping to LayerBodyBasic.
	LayerBodyBasic
	LayerBodyShell
	LayerBodySpikes
	LayerBodyCamouflage

	// Feature overlays — additive. Drawn on top of the body in the
	// canonical order below; a high-res organism draws the overlay
	// corresponding to the deepest-held feature in each non-defense
	// tree (Flagellae, Sensors, Teeth).
	LayerFlagellae
	LayerCilia
	LayerStinger
	LayerAntennae
	LayerFeelers
	LayerTasters
	LayerTeeth
	LayerFangs
	LayerTusks
)

// layerFilenamePrefix maps each Layer to the filename prefix the lua
// export script writes for it, e.g. body_basic_small_attack.png →
// {LayerBodyBasic, RoleOrganismSmall, AnimAttack}. LayerBody has no
// prefix because the low-res "only one applicable layer" naming rule
// in the export script drops the layer prefix entirely (e.g.
// small_attack.png).
var layerFilenamePrefix = map[Layer]string{
	LayerBody:           "",
	LayerBodyBasic:      "body_basic",
	LayerBodyShell:      "body_shell",
	LayerBodySpikes:     "body_spikes",
	LayerBodyCamouflage: "body_camouflage",
	LayerFlagellae:      "flagellae",
	LayerCilia:          "cilia",
	LayerStinger:        "stinger",
	LayerAntennae:       "antennae",
	LayerFeelers:        "feelers",
	LayerTasters:        "tasters",
	LayerTeeth:          "teeth",
	LayerFangs:          "fangs",
	LayerTusks:          "tusks",
}

// featureOverlayLayer maps each non-defense feature to its overlay
// Layer. Defense features map to body variants instead — see
// bodyVariantLayer. Only the deepest-held feature in each non-defense
// tree contributes an overlay (mirroring physiology.Combined's per-tree
// "one ancestor, not stacked" semantics), so deeper features in the
// same tree replace shallower ones rather than stacking on top.
var featureOverlayLayer = map[physiology.Feature]Layer{
	physiology.FeatFlagellae: LayerFlagellae,
	physiology.FeatCilia:     LayerCilia,
	physiology.FeatStinger:   LayerStinger,
	physiology.FeatAntennae:  LayerAntennae,
	physiology.FeatFeelers:   LayerFeelers,
	physiology.FeatTasters:   LayerTasters,
	physiology.FeatTeeth:     LayerTeeth,
	physiology.FeatFangs:     LayerFangs,
	physiology.FeatTusks:     LayerTusks,
}

// bodyVariantLayer returns the body-variant Layer for an organism with
// the given physiology — driven by the defense tree's deepest-held
// feature (Shell/Spikes/Camouflage), falling back to LayerBodyBasic
// when the organism hasn't evolved into the defense tree.
func bodyVariantLayer(features physiology.Set) Layer {
	switch features.Deepest(physiology.TreeDefense) {
	case physiology.FeatShell:
		return LayerBodyShell
	case physiology.FeatSpikes:
		return LayerBodySpikes
	case physiology.FeatCamouflage:
		return LayerBodyCamouflage
	default:
		return LayerBodyBasic
	}
}

// appendOverlay appends the overlay layer for the deepest-held feature
// in tree to out, or leaves out unchanged if the organism hasn't entered
// the tree (no overlay) or the deepest feature has no mapping (e.g.
// defense features, which drive bodyVariantLayer instead).
func appendOverlay(out []Layer, features physiology.Set, tree physiology.Tree) []Layer {
	deepest := features.Deepest(tree)
	if deepest == physiology.FeatNone {
		return out
	}
	if layer, ok := featureOverlayLayer[deepest]; ok {
		out = append(out, layer)
	}
	return out
}

// OrganismLayersFor returns the ordered list of layers a high-res
// renderer should draw for an organism with the given physiology, from
// bottom to top:
//  1. Flagellae overlay  (Flagellae / Cilia / Stinger)
//  2. Teeth overlay      (Teeth / Fangs / Tusks)
//  3. Body variant       (basic / Shell / Spikes / Camouflage)
//  4. Sensors overlay    (Antennae / Feelers / Tasters)
//
// Locomotion sits underneath so the body covers wherever the limbs meet
// the silhouette; teeth sit under the body so they read as protruding
// from the mouth rather than floating in front of it; sensors sit on
// top because antennae and feelers are intended to be visible above
// every body / armour variant.
func OrganismLayersFor(features physiology.Set) []Layer {
	out := make([]Layer, 0, 4)
	out = appendOverlay(out, features, physiology.TreeFlagellae)
	out = appendOverlay(out, features, physiology.TreeTeeth)
	out = append(out, bodyVariantLayer(features))
	out = appendOverlay(out, features, physiology.TreeSensors)
	return out
}

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

// ZoomImages holds sprite sets for the 4 native sprite sizes
// (0=4x4, 1=8x8, 2=16x16, 3=32x32).
var ZoomImages [4]map[ImageRole]LayeredFrames

// currentZoom tracks which ZoomImages entry is active. Persisted across
// calls to initImages so reloads (e.g. theme toggling) keep pointing at
// the sprite set the camera is currently using, instead of snapping back
// to zero.
var currentZoom int

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

	dirs := [4]string{"4x4", "8x8", "16x16", "32x32"}
	sizes := [4]int{4, 8, 16, 32}

	// Light vs dark theme uses separate sprite directories so artists
	// can keep two parallel sets — same Lua export script, different
	// source aseprite files. ReloadImages is called by cycleTheme so
	// switching themes at runtime picks up the new sheets.
	themeDir := "grid_dark"
	if config.IsLightTheme() {
		themeDir = "grid_light"
	}

	for i, dir := range dirs {
		size := sizes[i]
		path := "resources/images/" + themeDir + "/" + dir + "/"

		// Base single-frame sprites per organism role. Used as a fallback
		// when a per-action sheet is missing. Loaded from disk if the
		// corresponding square_<role>.png exists; otherwise synthesised
		// programmatically so the loader never crashes on missing art.
		baseSmall := loadOrGenerateFilled(path+"square_small.png", size, max(1, size/3))
		baseMedium := loadOrGenerateFilled(path+"square_medium.png", size, max(2, size*2/3))
		baseLarge := loadOrGenerateFilled(path+"square_large.png", size, size)

		// Walls are authored as three strength tiers per resolution;
		// the grid renderer picks between them based on the wall's
		// current strength as a fraction of MaxWallStrength. Each
		// tier falls back to a generated box-outline at the right
		// pixel size when its PNG hasn't been drawn yet.
		wallWeak := loadOrGenerateBox(path+"wall_weak.png", size)
		wallMedium := loadOrGenerateBox(path+"wall_medium.png", size)
		wallStrong := loadOrGenerateBox(path+"wall_strong.png", size)
		// Food is authored as three size tiers per resolution; the grid
		// renderer picks between them based on the food item's value as a
		// fraction of MaxFoodValue. Circle fallbacks match the organism
		// size-tier fallbacks so missing art degrades gracefully.
		foodSmall := loadOrGenerateCircle(path+"food_small.png", size, max(1, size/3))
		foodMedium := loadOrGenerateCircle(path+"food_medium.png", size, max(2, size*2/3))
		foodLarge := loadOrGenerateCircle(path+"food_large.png", size, size)

		// Frames per cycle scale with resolution (1 / 2 / 4 at 4 / 8 / 16).
		// Missing per-action sheets fall back to repeating the base sprite
		// so all frame slots render the same image (harmless).
		orgFrames := size / 4
		if orgFrames < 1 {
			orgFrames = 1
		}

		bases := map[ImageRole]*ebiten.Image{
			RoleOrganismSmall:  baseSmall,
			RoleOrganismMedium: baseMedium,
			RoleOrganismLarge:  baseLarge,
		}

		ZoomImages[i] = map[ImageRole]LayeredFrames{
			RoleFoodSmall:  {LayerBody: staticFrames(foodSmall)},
			RoleFoodMedium: {LayerBody: staticFrames(foodMedium)},
			RoleFoodLarge:  {LayerBody: staticFrames(foodLarge)},
			RoleWallWeak:   {LayerBody: staticFrames(wallWeak)},
			RoleWallMedium: {LayerBody: staticFrames(wallMedium)},
			RoleWallStrong: {LayerBody: staticFrames(wallStrong)},
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
		return loadImage(fullPath)
	}
	return generateFilledImage(totalSize, innerSize)
}

// loadOrGenerateCircle is the same pattern for circle-shaped fallbacks
// (food).
func loadOrGenerateCircle(fullPath string, totalSize, diameter int) *ebiten.Image {
	if assetExists(fullPath) {
		return loadImage(fullPath)
	}
	return generateCircle(totalSize, diameter)
}

// loadOrGenerateBox is the same pattern for box-outline fallbacks
// (walls). All three wall strength tiers fall back to the same
// generated box at the right pixel size when the PNG hasn't been
// drawn yet; the renderer's strength → tier mapping still picks
// between them so the distinct sprites can land later without code
// changes.
func loadOrGenerateBox(fullPath string, totalSize int) *ebiten.Image {
	if assetExists(fullPath) {
		return loadImage(fullPath)
	}
	return generateBoxImage(totalSize)
}

// loadOrganismLayers builds the LayeredFrames for one organism role at
// one zoom. At low-res (highRes == false) the artist authors a single
// `body` layer per role + action and the export script writes filenames
// without a layer prefix (e.g. small_move.png) — that single sheet is
// loaded into LayerBody. At high-res the artist authors one PNG per
// (layer, role, action) triple (e.g. body_basic_small_move.png,
// flagellae_small_move.png) and each Layer with a PNG on disk gets its
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
		sheet := loadImage(sheetPath)
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

func loadImage(path string) *ebiten.Image {
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
	return ebiten.NewImageFromImage(img)
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

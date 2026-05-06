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
	RoleBox // walls
)

// FrameSet holds one slice of per-frame sprites per Animation kind.
//
// Frame count scales with the sprite set's resolution: 4x4 holds 1 frame,
// 8x8 holds 2, 16x16 holds 4 (resolution / 4). Non-organism roles (food,
// walls) only populate AnimIdle and only ever need one frame.
type FrameSet map[animation.Animation][]*ebiten.Image

var (
	FontInversionz40    font.Face
	FontSourceCodePro12 font.Face
	FontSourceCodePro10 font.Face
	FontSourceCodePro8  font.Face

	PlayButton  *ebiten.Image
	PauseButton *ebiten.Image

	// Images is the active sprite set (set by SelectZoom), keyed by role.
	Images map[ImageRole]FrameSet
)

// ZoomImages holds sprite sets for the 3 native sprite sizes
// (0=4x4, 1=8x8, 2=16x16).
var ZoomImages [3]map[ImageRole]FrameSet

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

// Sprite returns the sprite image for a given role/animation/frame,
// defaulting to AnimIdle when the requested animation has no frames and
// cycling within the available frames when frame >= len.
func Sprite(role ImageRole, anim animation.Animation, frame int) *ebiten.Image {
	set, ok := Images[role]
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

		box := generateBoxImage(size)
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

		ZoomImages[i] = map[ImageRole]FrameSet{
			RoleFoodSmall:  staticFrames(foodSmall),
			RoleFoodMedium: staticFrames(foodMedium),
			RoleFoodLarge:  staticFrames(foodLarge),
			RoleBox:        staticFrames(box),
		}
		for role, base := range bases {
			ZoomImages[i][role] = loadOrganismFrames(path, role, base, size, orgFrames)
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

// loadOrganismFrames builds the FrameSet for one organism role at one zoom.
// For each Animation it tries to load a per-action spritesheet at
// "<path>/<role>_<action>.png" (e.g. "small_move.png"). If present it's
// sliced into orgFrames frames; each frame's width is derived from the
// sheet (total width / orgFrames) so multi-cell sheets (e.g. 128x16 move
// sheets depicting a 2-cell journey) work without per-action configuration.
// If the sheet is missing it falls back to repeating the base single-frame
// sprite.
func loadOrganismFrames(path string, role ImageRole, base *ebiten.Image, frameSize, orgFrames int) FrameSet {
	_ = frameSize // per-frame width is inferred from sheet dimensions
	set := make(FrameSet, len(animation.AllAnimations))
	roleName := organismRoleName[role]
	for _, anim := range animation.AllAnimations {
		animName := animationFileName[anim]
		sheetPath := path + roleName + "_" + animName + ".png"
		if roleName != "" && animName != "" && assetExists(sheetPath) {
			set[anim] = loadSpriteSheet(sheetPath, orgFrames)
			continue
		}
		set[anim] = repeatSprite(base, orgFrames)
	}
	return set
}

// loadSpriteSheet loads a horizontal spritesheet and returns per-frame
// SubImage views. Frame width is inferred from the sheet's dimensions
// (total width / frames), so a sheet can hold multi-cell frames without
// the caller specifying the cell size.
func loadSpriteSheet(path string, frames int) []*ebiten.Image {
	if frames < 1 {
		frames = 1
	}
	sheet := loadImage(path)
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

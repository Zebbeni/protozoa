package resources

import (
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"github.com/Zebbeni/protozoa/animation"
)

// animationFileName is the filename stem used for per-action spritesheet PNGs
// (e.g. "small_move.png"). Kept in sync with resources/gen/main.go.
var animationFileName = map[animation.Animation]string{
	animation.AnimIdle:    "idle",
	animation.AnimMove:    "move",
	animation.AnimBlocked: "blocked",
	animation.AnimTurn:    "turn",
	animation.AnimAttack:  "attack",
	animation.AnimEat:     "eat",
	animation.AnimChemo:   "chemo",
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
type ImageRole int

const (
	RoleOrganismSmall ImageRole = iota
	RoleOrganismMedium
	RoleOrganismLarge
	RoleFood
	RoleBox // walls
)

// FrameSet holds one slice of per-animation-frame sprites per Animation kind.
//
// For 4x4 sprites every slice is length 1 (we only show the final frame per
// cycle at that zoom). For 16x16 sprites organism animations hold
// animation.BaseFramesPerCycle frames; non-organism roles (food, walls)
// still only populate AnimIdle and only ever need one frame.
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

// ZoomImages holds sprite sets for the 2 native sprite sizes (0=4x4, 1=16x16).
var ZoomImages [2]map[ImageRole]FrameSet

func Init() {
	initFonts()
	initImages()
}

// SelectZoom sets the active image set to the given sprite-set index (0-1).
func SelectZoom(level int) {
	if level < 0 || level > 1 {
		return
	}
	Images = ZoomImages[level]
}

// Sprite returns the sprite image for a given role/animation/frame, defaulting
// to AnimIdle when the requested animation has no frames and cycling within
// the available frames when frame >= len.
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

	dirs := [2]string{"4x4", "16x16"}
	sizes := [2]int{4, 16}

	for i, dir := range dirs {
		size := sizes[i]
		path := "resources/images/grid/" + dir + "/"

		box := generateBoxImage(size)
		food := generateCircle(size, max(2, size*2/3))

		// Load or generate the base single-frame sprite per organism role.
		// These are used both as a fallback when a per-action sheet is
		// missing and to populate the 4x4 zoom's single-frame animations.
		var baseSmall, baseMedium, baseLarge *ebiten.Image
		if dirExists("resources/images/grid/" + dir) {
			baseSmall = loadImage(path + "square_small.png")
			baseMedium = loadImage(path + "square_medium.png")
			baseLarge = loadImage(path + "square_large.png")
			food = loadImage(path + "food.png")
		} else {
			baseSmall = generateFilledImage(size, max(1, size/3))
			baseMedium = generateFilledImage(size, max(2, size*2/3))
			baseLarge = generateFilledImage(size, size)
		}

		// Number of frames per organism animation at this zoom. At 4x4 we
		// show a single frame per cycle by design; at 16x16 we animate
		// across the full 4-frame cycle.
		orgFrames := 1
		if size >= 16 {
			orgFrames = animation.BaseFramesPerCycle
		}

		bases := map[ImageRole]*ebiten.Image{
			RoleOrganismSmall:  baseSmall,
			RoleOrganismMedium: baseMedium,
			RoleOrganismLarge:  baseLarge,
		}

		ZoomImages[i] = map[ImageRole]FrameSet{
			RoleFood: staticFrames(food),
			RoleBox:  staticFrames(box),
		}
		for role, base := range bases {
			ZoomImages[i][role] = loadOrganismFrames(path, role, base, size, orgFrames)
		}
	}

	SelectZoom(0)
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
	_ = frameSize // kept for API symmetry; per-frame width is inferred from the sheet
	set := make(FrameSet, len(animation.AllAnimations))
	roleName := organismRoleName[role]
	for _, anim := range animation.AllAnimations {
		animName := animationFileName[anim]
		sheetPath := path + roleName + "_" + animName + ".png"
		if roleName != "" && animName != "" && fileExists(sheetPath) {
			set[anim] = loadSpriteSheet(sheetPath, orgFrames)
			continue
		}
		set[anim] = repeatSprite(base, orgFrames)
	}
	return set
}

// loadSpriteSheet loads a horizontal spritesheet and returns per-frame
// SubImage views. Frame width is inferred from the sheet's dimensions
// (total width / frames), so a sheet can hold multi-cell frames (e.g. a
// 2-cell-wide move sprite) without the caller specifying it. Shares pixels
// with the source image so SubImage views are cheap.
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

// repeatSprite returns a frames-long slice containing the same base sprite.
// Used when a per-action spritesheet isn't present on disk.
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
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, totalSize, totalSize))
	offset := (totalSize - innerSize) / 2
	for y := offset; y < offset+innerSize; y++ {
		for x := offset; x < offset+innerSize; x++ {
			img.Set(x, y, black)
		}
	}
	return ebiten.NewImageFromImage(img)
}

func generateCircle(totalSize, diameter int) *ebiten.Image {
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, totalSize, totalSize))
	cx, cy := float64(totalSize)/2.0, float64(totalSize)/2.0
	r := float64(diameter) / 2.0
	for y := 0; y < totalSize; y++ {
		for x := 0; x < totalSize; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, black)
			}
		}
	}
	return ebiten.NewImageFromImage(img)
}

func generateBoxImage(size int) *ebiten.Image {
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for i := 0; i < size; i++ {
		img.Set(i, 0, black)
		img.Set(i, size-1, black)
		img.Set(0, i, black)
		img.Set(size-1, i, black)
	}
	return ebiten.NewImageFromImage(img)
}

func dirExists(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(absPath)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(absPath)
	return err == nil && !info.IsDir()
}

func loadImage(path string) *ebiten.Image {
	filepath, err := filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}
	reader, err := os.Open(filepath)
	if err != nil {
		log.Fatal(err)
	}
	img, err := png.Decode(reader)
	if err != nil {
		log.Fatal(err)
	}
	return ebiten.NewImageFromImage(img)
}

func loadFont(path string) *opentype.Font {
	filepath, err := filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}
	fontData, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Fatal(err)
	}
	tt, err := opentype.Parse(fontData)
	if err != nil {
		log.Fatal(err)
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

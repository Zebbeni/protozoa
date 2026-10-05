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

// animationFileName is the filename stem used for per-action spritesheet PNGs (e.g. "small_move.png").
var animationFileName = map[animation.Animation]string{
	animation.AnimIdle:       "idle",
	animation.AnimMove:       "move",
	animation.AnimBlocked:    "blocked",
	animation.AnimTurnLeft:   "turn_left",
	animation.AnimTurnRight:  "turn_right",
	animation.AnimAttack:     "attack",
	animation.AnimEat:        "eat",
	animation.AnimEatFail:    "eatfail",
	animation.AnimChemo:      "chemo",
	animation.AnimChemoFail:  "chemofail",
	animation.AnimDie:        "die",
	animation.AnimDig:        "dig",
	animation.AnimAttackMove: "attack_success",
}

// animationBorrows is the sheet an animation falls back to when it has none
// of its own, instead of the role's static base image. An action whose art
// has not been drawn yet should look like the nearest action that has been,
// not like an organism standing still.
var animationBorrows = map[animation.Animation]animation.Animation{
	animation.AnimAttackMove: animation.AnimAttack,
}

// organismRoleName maps each organism role to the filename stem used by the per-action spritesheets (e.g. "small", "medium", "large").
var organismRoleName = map[ImageRole]string{
	RoleOrganismTiny:   "tiny",
	RoleOrganismSmall:  "small",
	RoleOrganismMedium: "medium",
	RoleOrganismLarge:  "large",
}

const (
	dpi = 72
)

type ImageRole int

const (
	RoleOrganismSmall ImageRole = iota
	RoleOrganismMedium
	RoleOrganismLarge
	RoleFoodSmall
	RoleFoodMedium
	RoleFoodLarge
	// Wall sprites by strength tier — see ux/grid.go's wallRoleForStrength for the strength → role mapping.
	RoleWallWeak
	RoleWallMedium
	RoleWallStrong
	// Appended, not slotted in beside their siblings.
	RoleOrganismTiny
	RoleFoodTiny
	RoleWallGiant
)

type FrameSet map[animation.Animation][]*ebiten.Image

// Layer identifies one drawable spritesheet layer for an organism role, matching the Aseprite layer names that the export lua script writes PNG files for.
type Layer int

const (
	// LayerBody is the single-layer slot used by low-res organism sprites, food, and walls.
	LayerBody Layer = iota

	LayerBodyBasic
	LayerBodyShell
	LayerBodySpikes

	LayerPili
	LayerFlagella
	// Iota slot retained for back-compat with any code that bound an integer Layer ID directly to art.
	_
	LayerAntennae
	LayerFeelers
	LayerTasters
	LayerTeeth
	LayerFangs
	LayerTusks

	LayerWallBase
	LayerWallUp
	LayerWallDown
	LayerWallLeft
	LayerWallRight
)

// layerFilenamePrefix maps each Layer to the filename prefix the lua export script writes for it, e.g. body_basic_small_attack.png → {LayerBodyBasic, RoleOrganismSmall, AnimAttack}.
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

// UsesPrimaryColor reports whether the given Layer should be tinted with the organism's primary colour (true) or secondary colour (false).
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

// OrganismLayersFor returns the ordered list of layers a high-res renderer should draw for an organism with the given appearance, from bottom to top.
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

// ZoomImages holds sprite sets for the 3 native sprite sizes (0=4x4, 1=8x8, 2=16x16).
var ZoomImages [3]map[ImageRole]LayeredFrames

var currentZoom int

const ZoomHighRes = len(ZoomImages) - 1

// CurrentZoom reports which set is active, so a caller that needs a specific one can put it back afterwards.
func CurrentZoom() int { return currentZoom }

func Init() {
	initFonts()
	initImages()
}

// SelectZoom sets the active image set to the given sprite-set index (0-2).
func SelectZoom(level int) {
	if level < 0 || level >= len(ZoomImages) {
		return
	}
	currentZoom = level
	Images = ZoomImages[level]
}

func ReloadImages() {
	initImages()
}

// Sprite returns the default sprite image for a given role/animation/ frame.
func Sprite(role ImageRole, anim animation.Animation, frame int) *ebiten.Image {
	return spriteFromSet(Images, role, defaultLayer(Images, role), anim, frame)
}

// SpriteAtZoom is like Sprite but reads from a specific sprite-set level (0=4x4, 1=8x8, 2=16x16) regardless of which zoom is currently active.
func SpriteAtZoom(level int, role ImageRole, anim animation.Animation, frame int) *ebiten.Image {
	if level < 0 || level >= len(ZoomImages) {
		return nil
	}
	images := ZoomImages[level]
	return spriteFromSet(images, role, defaultLayer(images, role), anim, frame)
}

// SpriteLayer returns the sprite image for a specific role/layer/anim/ frame in the active zoom set, or nil if no PNG was loaded for that layer (which is the normal case for layers an organism's physiology doesn't unlock).
func SpriteLayer(role ImageRole, layer Layer, anim animation.Animation, frame int) *ebiten.Image {
	return spriteFromSet(Images, role, layer, anim, frame)
}

// SpriteLayerAtZoom is like SpriteLayer but reads from a specific sprite-set level regardless of which zoom is currently active.
func SpriteLayerAtZoom(level int, role ImageRole, layer Layer, anim animation.Animation, frame int) *ebiten.Image {
	if level < 0 || level >= len(ZoomImages) {
		return nil
	}
	return spriteFromSet(ZoomImages[level], role, layer, anim, frame)
}

// defaultLayer picks the layer Sprite() should read from for the given role.
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

	// Both themes draw the same sprite set.
	themeDir := spriteDir

	for i, dir := range dirs {
		size := sizes[i]
		path := "resources/images/" + themeDir + "/" + dir + "/"

		baseTiny := loadOrGenerateFilled(path+"square_tiny.png", size, max(1, size/4))
		baseSmall := loadOrGenerateFilled(path+"square_small.png", size, max(1, size/2))
		baseMedium := loadOrGenerateFilled(path+"square_medium.png", size, max(2, size*3/4))
		baseLarge := loadOrGenerateFilled(path+"square_large.png", size, size)

		// Walls are authored as four strength tiers per resolution.
		wallWeak := loadWallLayers(path, "weak", size)
		wallMedium := loadWallLayers(path, "medium", size)
		wallStrong := loadWallLayers(path, "strong", size)
		wallGiant := loadWallLayers(path, "giant", size)
		// Food is authored as four size tiers per resolution.
		foodTiny := loadOrGenerateCircle(path+"food_tiny.png", size, max(1, size/4))
		foodSmall := loadOrGenerateCircle(path+"food_small.png", size, max(1, size/2))
		foodMedium := loadOrGenerateCircle(path+"food_medium.png", size, max(2, size*3/4))
		foodLarge := loadOrGenerateCircle(path+"food_large.png", size, size)

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
		// Low-res (4x4 / 8x8) authors a single unvaried `body` layer per organism role.
		highRes := size >= 16
		for role, base := range bases {
			ZoomImages[i][role] = loadOrganismLayers(path, role, base, size, orgFrames, highRes)
		}
	}

	// Preserve whichever zoom was active before a reload.
	SelectZoom(currentZoom)
}

// loadOrGenerateFilled returns loadImage(fullPath) if the file exists, otherwise a programmatically-generated filled-square fallback.
func loadOrGenerateFilled(fullPath string, totalSize, innerSize int) *ebiten.Image {
	if assetExists(fullPath) {
		return loadSprite(fullPath)
	}
	return generateFilledImage(totalSize, innerSize)
}

func loadOrGenerateCircle(fullPath string, totalSize, diameter int) *ebiten.Image {
	if assetExists(fullPath) {
		return loadSprite(fullPath)
	}
	return generateCircle(totalSize, diameter)
}

// loadOrNil returns the loaded image when the asset exists, otherwise nil.
func loadOrNil(fullPath string) *ebiten.Image {
	if assetExists(fullPath) {
		return loadSprite(fullPath)
	}
	return nil
}

// loadWallLayers loads one strength tier's full layered set.
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

func loadOrGenerateBox(fullPath string, totalSize int) *ebiten.Image {
	if assetExists(fullPath) {
		return loadSprite(fullPath)
	}
	return generateBoxImage(totalSize)
}

func loadOrganismLayers(path string, role ImageRole, base *ebiten.Image, frameSize, orgFrames int, highRes bool) LayeredFrames {
	out := make(LayeredFrames)
	if !highRes {
		out[LayerBody] = loadAnimSheets(path, "", role, base, frameSize, orgFrames)
		return out
	}
	// High-res: try every Layer except LayerBody (which is the low-res single-layer slot and isn't authored at this resolution).
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
			// Borrowed below, once every sheet that exists has loaded.
			if _, borrows := animationBorrows[anim]; borrows {
				continue
			}
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
	// A second pass, so a borrow does not depend on AllAnimations order.
	for anim, from := range animationBorrows {
		if set[anim] != nil {
			continue
		}
		if set[from] != nil {
			set[anim] = set[from]
			continue
		}
		if fallback != nil {
			set[anim] = repeatSprite(fallback, orgFrames)
			loaded = true
		}
	}
	if !loaded {
		return nil
	}
	return set
}

// sliceSheet splits a horizontal spritesheet into per-frame SubImage views.
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

// repeatSprite returns a frames-long slice pointing at the same base sprite.
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

// staticFrames produces a FrameSet that only has a single AnimIdle entry, suitable for non-organism roles (food, walls) that don't animate.
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

// lightThemeSpriteShadow, lightThemeSpriteGamma and lightThemeSpriteDarken set the tone curve the light theme applies to sprite greys.
const (
	lightThemeSpriteShadow = 0.25
	lightThemeSpriteGamma  = 0.65
	lightThemeSpriteDarken = 0.3
)

// loadSprite loads a grid sprite, applying the light theme's tone curve.
func loadSprite(path string) *ebiten.Image {
	img := decodeImage(path)
	if config.IsLightTheme() {
		img = toneImage(img, lightThemeSpriteShadow, lightThemeSpriteGamma, lightThemeSpriteDarken)
	}
	return ebiten.NewImageFromImage(img)
}

// toneImage returns a copy of img with each opaque pixel's colour channels remapped by the light theme's tone curve (channels in [0, 1], non-premultiplied).
func toneImage(img image.Image, shadow, gamma, darken float64) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	shadow = min(1, max(0, shadow))
	// Past 0.5 more of the darkest greys would floor to black and lose their shading; cap it there.
	darken = min(0.5, max(0, darken))
	if gamma <= 0 {
		gamma = 1
	}
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

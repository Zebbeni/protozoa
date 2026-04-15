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
)

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

var (
	FontInversionz40    font.Face
	FontSourceCodePro12 font.Face
	FontSourceCodePro10 font.Face
	FontSourceCodePro8  font.Face

	PlayButton  *ebiten.Image
	PauseButton *ebiten.Image

	// Images is the active sprite set, keyed by role (set by SelectZoom)
	Images map[ImageRole]*ebiten.Image
)

// ZoomImages holds sprite sets for all 3 zoom levels (indexed by ZoomLevel 0-2).
var ZoomImages [3]map[ImageRole]*ebiten.Image

func Init() {
	initFonts()
	initImages()
}

// SelectZoom sets the active image set to the given zoom level (0-2).
func SelectZoom(level int) {
	if level < 0 || level > 2 {
		return
	}
	Images = ZoomImages[level]
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

	for i, dir := range dirs {
		size := sizes[i]
		path := "resources/images/grid/" + dir + "/"

		box := generateBoxImage(size)
		food := generateCircle(size, max(2, size*2/3))

		if dirExists("resources/images/grid/" + dir) {
			ZoomImages[i] = map[ImageRole]*ebiten.Image{
				RoleOrganismSmall:  loadImage(path + "square_small.png"),
				RoleOrganismMedium: loadImage(path + "square_medium.png"),
				RoleOrganismLarge:  loadImage(path + "square_large.png"),
				RoleFood:           loadImage(path + "food.png"),
				RoleBox:            box,
			}
		} else {
			ZoomImages[i] = map[ImageRole]*ebiten.Image{
				RoleOrganismSmall:  generateFilledImage(size, max(1, size/3)),
				RoleOrganismMedium: generateFilledImage(size, max(2, size*2/3)),
				RoleOrganismLarge:  generateFilledImage(size, size),
				RoleFood:           food,
				RoleBox:            box,
			}
		}
	}

	SelectZoom(1)
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

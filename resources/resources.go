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

var (
	// FontInversionz40 is a size 50 Inversionz font face
	FontInversionz40 font.Face
	// FontSourceCodePro12 is a size 12 SourceCodePro (Regular) font face
	FontSourceCodePro12 font.Face
	// FontSourceCodePro10 is a size 10 SourceCodePro (Regular) font face
	FontSourceCodePro10 font.Face
	// FontSourceCodePro8 is a size 8 SourceCodePro (Regular) font face
	FontSourceCodePro8 font.Face

	// PlayButton is a 30x30 image
	PlayButton *ebiten.Image
	// PauseButton is a 30x30 image
	PauseButton *ebiten.Image

	// SquareSmall, SquareMedium, etc. are the active set (set by SelectZoom)
	SquareSmall  *ebiten.Image
	SquareMedium *ebiten.Image
	SquareLarge  *ebiten.Image
	SquareFill   *ebiten.Image
	SquareBox    *ebiten.Image
)

// ZoomResources holds the square images for a single zoom level.
type ZoomResources struct {
	Small, Medium, Large, Fill, Box *ebiten.Image
}

// ZoomImages holds resources for all 3 zoom levels (indexed by ZoomLevel 0-2).
var ZoomImages [3]ZoomResources

// Init loads all fonts and images to be used in the UI
func Init() {
	initFonts()
	initImages()
}

// SelectZoom sets the active square images to the given zoom level (0-2).
func SelectZoom(level int) {
	if level < 0 || level > 2 {
		return
	}
	res := ZoomImages[level]
	SquareSmall = res.Small
	SquareMedium = res.Medium
	SquareLarge = res.Large
	SquareFill = res.Fill
	SquareBox = res.Box
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
	// Panel Images
	PlayButton = loadImage("resources/images/play_button.png")
	PauseButton = loadImage("resources/images/pause_button.png")

	// Load all zoom levels
	dirs := [3]string{"4x4", "8x8", "16x16"}
	sizes := [3]int{4, 8, 16}

	for i, dir := range dirs {
		size := sizes[i]
		if dirExists("resources/images/grid/" + dir) {
			ZoomImages[i] = ZoomResources{
				Small:  loadImage("resources/images/grid/" + dir + "/square_small.png"),
				Medium: loadImage("resources/images/grid/" + dir + "/square_large.png"),
				Large:  loadImage("resources/images/grid/" + dir + "/square_large.png"),
				Fill:   loadImage("resources/images/grid/" + dir + "/square_fill.png"),
				Box:    loadImage("resources/images/grid/" + dir + "/square_box.png"),
			}
		} else {
			// Generate placeholder images for missing zoom levels
			ZoomImages[i] = generateSquareImages(size)
		}
	}

	// Default to medium zoom
	SelectZoom(1)
}

// generateSquareImages creates simple white-mask square images at the given size.
func generateSquareImages(size int) ZoomResources {
	small := size / 3
	if small < 1 {
		small = 1
	}
	med := size * 2 / 3
	if med < 2 {
		med = 2
	}

	return ZoomResources{
		Small:  generateFilledSquare(size, small),
		Medium: generateFilledSquare(size, med),
		Large:  generateFilledSquare(size, size),
		Fill:   generateFilledSquare(size, size),
		Box:    generateBoxSquare(size),
	}
}

func generateFilledSquare(totalSize, innerSize int) *ebiten.Image {
	// Use opaque black — ColorM.Translate adds RGB to produce the final color
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

func generateBoxSquare(size int) *ebiten.Image {
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
	ebitenImg := ebiten.NewImageFromImage(img)
	return ebitenImg
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

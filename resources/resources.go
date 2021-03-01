package resources

import (
	"bytes"
	"image"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/hajimehoshi/ebiten"
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
	// FontSourceCodePro10 is a size 11 SourceCodePro (Regular) font face
	FontSourceCodePro10 font.Face

	// PlayButton is a 30x30 image
	PlayButton *ebiten.Image
	// PauseButton is a 30x30 image
	PauseButton *ebiten.Image

	// SquareSmall5x5 is a 5x5 image to render for small organisms
	SquareSmall5x5 *ebiten.Image
	// SquareSmall5x5 is a 5x5 image to render for medium organisms
	SquareMedium5x5 *ebiten.Image
	// SquareSmall5x5 is a 5x5 image to render for large organisms
	SquareLarge5x5 *ebiten.Image
)

// Init loads all fonts and images to be used in the UI
func Init() {
	initFonts()
	initImages()
}

func initFonts() {
	inversionz := loadFont("resources/fonts/Inversionz.ttf")
	FontInversionz40 = fontFace(inversionz, 40)
	sourceCode := loadFont("resources/fonts/SourceCodePro-Regular.ttf")
	FontSourceCodePro12 = fontFace(sourceCode, 12)
	FontSourceCodePro10 = fontFace(sourceCode, 10)
}

func initImages() {
	// Panel Images
	PlayButton = loadImage("resources/images/play_button.png")
	PauseButton = loadImage("resources/images/pause_button.png")
	// Grid Images
	SquareSmall5x5 = loadImage("resources/images/grid/5x5_square_small.png")
	SquareMedium5x5 = loadImage("resources/images/grid/5x5_square_medium.png")
	SquareLarge5x5 = loadImage("resources/images/grid/5x5_square_large.png")
}

func loadImage(path string) *ebiten.Image {
	filepath, err := filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}
	imageData, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Fatal(err)
	}
	image, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		log.Fatal(err)
	}
	ebitenImg, err := ebiten.NewImageFromImage(image, ebiten.FilterDefault)
	if err != nil {
		log.Fatal(err)
	}
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

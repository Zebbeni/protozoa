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
	// SourceCodePro12 is a size 12 SourceCodePro (Regular) font face
	SourceCodePro12 font.Face
	// SourceCodePro10 is a size 11 SourceCodePro (Regular) font face
	SourceCodePro10 font.Face

	// PlayButton is a 30x30 image
	PlayButton *ebiten.Image
	// PauseButton is a 30x30 image
	PauseButton *ebiten.Image
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
	SourceCodePro12 = fontFace(sourceCode, 12)
	SourceCodePro10 = fontFace(sourceCode, 10)
}

func initImages() {
	PlayButton = loadImage("resources/images/play_button.png")
	PauseButton = loadImage("resources/images/pause_button.png")
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

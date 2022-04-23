package resources

import (
	"fmt"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/Zebbeni/protozoa/config"

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
	// FontSourceCodePro10 is a size 11 SourceCodePro (Regular) font face
	FontSourceCodePro10 font.Face

	// PlayButton is a 30x30 image
	PlayButton *ebiten.Image
	// PauseButton is a 30x30 image
	PauseButton *ebiten.Image

	// SquareSmall is an image to render for small organisms and food
	SquareSmall *ebiten.Image
	// SquareMedium is an image to render for medium organisms and food
	SquareMedium *ebiten.Image
	// SquareLarge is an image to render for large organisms and food
	SquareLarge *ebiten.Image
	// SquarePoison is an image to render for poison locations
	Poison *ebiten.Image
	// SquarePoison is an image to render for wall locations
	Wall *ebiten.Image
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

	var dir string
	switch config.GridUnitSize() {
	case 4:
		dir = "4x4"
		break
	case 5:
		dir = "5x5"
		break
	case 8:
		dir = "8x8"
		break
	default:
		panic(fmt.Sprintf("Unsupported grid unit size: %d", config.GridUnitSize()))
	}
	SquareSmall = loadImage(fmt.Sprintf("resources/images/grid/%s/square_small.png", dir))
	SquareMedium = loadImage(fmt.Sprintf("resources/images/grid/%s/square_large.png", dir))
	SquareLarge = loadImage(fmt.Sprintf("resources/images/grid/%s/square_large.png", dir))
	Poison = loadImage(fmt.Sprintf("resources/images/grid/%s/poison.png", dir))
	Wall = loadImage(fmt.Sprintf("resources/images/grid/%s/wall.png", dir))
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
		fmt.Println("shit")
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

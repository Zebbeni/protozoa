package resources

import (
	"io/ioutil"
	"log"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

const (
	dpi = 72
)

var (
	// FontInversionz50 is a size 50 Inversionz font face
	FontInversionz50 font.Face
)

// InitFonts loads and creates font faces for all the fonts used in the UI
func InitFonts() {
	inversionz := loadFont("resources/fonts/inversionz.ttf")
	FontInversionz50 = fontFace(inversionz, 50)
}

func loadFont(path string) *opentype.Font {
	filepath, err := filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}
	onyxData, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Fatal(err)
	}
	tt, err := opentype.Parse(onyxData)
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

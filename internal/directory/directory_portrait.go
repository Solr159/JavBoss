package directory

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"

	_ "golang.org/x/image/webp"
)

// Match the portrait frame and saved crop position in web/src/features/jav/coverCrop.js.
func portraitCropBounds(bounds image.Rectangle, position float64) image.Rectangle {
	const aspect = 376.0 / 538.0
	if math.IsNaN(position) || math.IsInf(position, 0) {
		position = 1
	}
	position = min(1, max(0, position))
	width := min(bounds.Dx(), max(1, int(math.Round(float64(bounds.Dy())*aspect))))
	height := min(bounds.Dy(), max(1, int(math.Round(float64(bounds.Dx())/aspect))))
	left := bounds.Min.X + int(math.Round(float64(bounds.Dx()-width)*position))
	top := bounds.Min.Y + (bounds.Dy()-height)/2
	return image.Rect(left, top, left+width, top+height)
}

func writePortraitSidecar(source, target string, position float64) error {
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open cover: %w", err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("decode cover: %w", err)
	}
	if img.Bounds().Empty() {
		return fmt.Errorf("cover has empty dimensions")
	}
	bounds := portraitCropBounds(img.Bounds(), position)
	portrait := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(portrait, portrait.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(portrait, portrait.Bounds(), img, bounds.Min, draw.Over)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, portrait, &jpeg.Options{Quality: 95}); err != nil {
		return fmt.Errorf("encode portrait: %w", err)
	}
	return writeFileAtomically(target, &encoded, 0o644)
}

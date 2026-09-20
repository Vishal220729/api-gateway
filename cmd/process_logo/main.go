package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	f, err := os.Open("web/logo.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	src, err := png.Decode(f)
	if err != nil {
		panic(err)
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// 1. Transparent version (white becomes transparent)
	transImg := image.NewRGBA(image.Rect(0, 0, w, h))
	// 2. Dark theme optimized version (white becomes transparent, dark text becomes bright white/cyan)
	darkImg := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.At(x, y)
			r, g, b, a := c.RGBA()
			rf := float64(r >> 8)
			gf := float64(g >> 8)
			bf := float64(b >> 8)
			af := float64(a >> 8)

			// Check distance from pure white (255, 255, 255)
			distFromWhite := math.Sqrt(math.Pow(255-rf, 2) + math.Pow(255-gf, 2) + math.Pow(255-bf, 2))

			var newAlpha uint8
			if distFromWhite < 15 {
				newAlpha = 0
			} else if distFromWhite < 45 {
				newAlpha = uint8((distFromWhite - 15) / 30.0 * 255.0)
			} else {
				newAlpha = uint8(af)
			}

			// Save to transImg
			transImg.Set(x, y, color.RGBA{
				R: uint8(rf),
				G: uint8(gf),
				B: uint8(bf),
				A: newAlpha,
			})

			// For darkImg: if it's the dark blue text (rf < 60 && gf < 80 && bf < 120), brighten it to crisp white/light cyan
			if newAlpha > 0 {
				if rf < 60 && gf < 80 && bf < 120 {
					// Dark text -> brighten to light cyan/white
					darkImg.Set(x, y, color.RGBA{
						R: 240,
						G: 253,
						B: 250,
						A: newAlpha,
					})
				} else {
					darkImg.Set(x, y, color.RGBA{
						R: uint8(rf),
						G: uint8(gf),
						B: uint8(bf),
						A: newAlpha,
					})
				}
			} else {
				darkImg.Set(x, y, color.RGBA{0, 0, 0, 0})
			}
		}
	}

	outTrans, _ := os.Create("web/logo-transparent.png")
	defer outTrans.Close()
	png.Encode(outTrans, transImg)

	outDark, _ := os.Create("web/logo-dark.png")
	defer outDark.Close()
	png.Encode(outDark, darkImg)
}

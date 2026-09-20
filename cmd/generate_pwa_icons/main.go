package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
)

func resizeNearest(src image.Image, targetW, targetH int) *image.RGBA {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

	// Fill background with deep obsidian black #050806
	bgColor := color.RGBA{R: 5, G: 8, B: 6, A: 255}
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	// Keep aspect ratio and center
	scaleX := float64(targetW) / float64(srcW)
	scaleY := float64(targetH) / float64(srcH)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}
	// Add padding
	scale *= 0.82

	scaledW := int(float64(srcW) * scale)
	scaledH := int(float64(srcH) * scale)
	offsetX := (targetW - scaledW) / 2
	offsetY := (targetH - scaledH) / 2

	for y := 0; y < scaledH; y++ {
		srcY := bounds.Min.Y + int(float64(y)/scale)
		if srcY >= bounds.Max.Y {
			srcY = bounds.Max.Y - 1
		}
		for x := 0; x < scaledW; x++ {
			srcX := bounds.Min.X + int(float64(x)/scale)
			if srcX >= bounds.Max.X {
				srcX = bounds.Max.X - 1
			}
			c := src.At(srcX, srcY)
			_, _, _, a := c.RGBA()
			if a > 0 {
				dst.Set(offsetX+x, offsetY+y, c)
			}
		}
	}
	return dst
}

func main() {
	srcFile, err := os.Open("web/logo-dark.png")
	if err != nil {
		log.Fatalf("failed to open web/logo-dark.png: %v", err)
	}
	defer srcFile.Close()

	srcImg, _, err := image.Decode(srcFile)
	if err != nil {
		log.Fatalf("failed to decode logo: %v", err)
	}

	// 1. Generate icon-192.png
	icon192 := resizeNearest(srcImg, 192, 192)
	f192, err := os.Create("web/icon-192.png")
	if err != nil {
		log.Fatalf("failed to create icon-192.png: %v", err)
	}
	defer f192.Close()
	if err := png.Encode(f192, icon192); err != nil {
		log.Fatalf("failed to encode icon-192.png: %v", err)
	}
	log.Println("Generated web/icon-192.png")

	// 2. Generate icon-512.png
	icon512 := resizeNearest(srcImg, 512, 512)
	f512, err := os.Create("web/icon-512.png")
	if err != nil {
		log.Fatalf("failed to create icon-512.png: %v", err)
	}
	defer f512.Close()
	if err := png.Encode(f512, icon512); err != nil {
		log.Fatalf("failed to encode icon-512.png: %v", err)
	}
	log.Println("Generated web/icon-512.png")

	// 3. Generate maskable-icon-512.png
	fMask, err := os.Create("web/maskable-icon-512.png")
	if err != nil {
		log.Fatalf("failed to create maskable-icon-512.png: %v", err)
	}
	defer fMask.Close()
	if err := png.Encode(fMask, icon512); err != nil {
		log.Fatalf("failed to encode maskable-icon-512.png: %v", err)
	}
	log.Println("Generated web/maskable-icon-512.png")
}
